package cfg_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestInstanceClassification(t *testing.T) {
	c := load(t, seedDir)
	for _, tc := range []struct {
		id, group int64
		kind      model.RunKind
		listed    bool
	}{
		{329, 5, model.RunDungeon, true}, {329, 40, model.RunDungeon, true},
		{99999, 5, model.RunUnknown, false}, {99999, 6, model.RunRaid, false}, {99999, 0, model.RunUnknown, false},
	} {
		kind, listed := c.InstanceKind(tc.id, tc.group)
		if kind != tc.kind || listed != tc.listed {
			t.Fatalf("%+v: kind=%v listed=%v", tc, kind, listed)
		}
	}
	dir := copyData(t)
	change(t, dir, "instances.json", func(d map[string]any) { d["raid_group_size_above"] = 7 })
	kind, listed := load(t, dir).InstanceKind(99999, 6)
	if kind != model.RunUnknown || listed {
		t.Fatal("group threshold was hardcoded or unknown id hidden")
	}
	change(t, dir, "instances.json", func(d map[string]any) {
		d["instances"] = append(d["instances"].([]any), map[string]any{"id": 99999, "name": "Synthetic raid", "kind": "raid", "source": "synthetic test, not shipped data"})
	})
	kind, listed = load(t, dir).InstanceKind(99999, 1)
	if kind != model.RunRaid || !listed {
		t.Fatal("sourced listing must beat fallback group size")
	}
}

func TestProfileLookupAndCalibration(t *testing.T) {
	c := load(t, seedDir)
	if _, ok := c.Profile("holy"); ok {
		t.Fatal("ambiguous word matched a profile")
	}
	for _, id := range []string{"PALADIN.holy", "PRIEST.holy"} {
		if p, ok := c.Profile(id); !ok || p.ID != id {
			t.Fatalf("lost canonical %s", id)
		}
	}
	want := map[string]bool{"PALADIN.shockadin": true, "SHAMAN.enhancement_tank": true, "MAGE.battlemage_frost": true, "MAGE.battlemage_fire": true, "MAGE.battlemage_arcane": true}
	for _, p := range c.Specs.Profiles {
		if p.Uncalibrated != want[p.ID] {
			t.Fatalf("unexpected calibration for %s", p.ID)
		}
		if len(p.CastSignatures) != 0 {
			t.Fatalf("unverified cast-only signature for %s", p.ID)
		}
		for _, s := range p.Spells {
			if s.ID != 0 {
				t.Fatal("seed should not contain unreviewed spell ids")
			}
		}
	}
	for _, id := range []string{"WARLOCK.destruction", "HUNTER.marksmanship"} {
		p, _ := c.Profile(id)
		if p.Pets != "optional" {
			t.Fatalf("%s must permit documented no-pet builds", id)
		}
	}
}

func TestBossOnlyDefaultAndOverrides(t *testing.T) {
	c := load(t, seedDir)
	if _, ok := c.Encounter(77); ok {
		t.Fatal("candidate promoted to enabled rule")
	}
	if r := c.OutputTargets(77); r.Mode != "boss_only" || len(r.Units) != 0 {
		t.Fatalf("default targets: %+v", r)
	}
	dir := copyData(t)
	change(t, dir, "encounters-enabled.json", func(d map[string]any) {
		r := syntheticRule()
		r["boss_units"] = []any{map[string]any{"id": 81, "name": "Synthetic left boss", "source": "synthetic log"}, map[string]any{"id": 82, "name": "Synthetic right boss", "source": "synthetic log"}}
		r["target_rule"] = map[string]any{"mode": "boss_and_units", "units": []any{map[string]any{"id": 83, "name": "Synthetic required add", "source": "synthetic log"}}}
		d["encounters"] = []any{r}
	})
	c = load(t, dir)
	e, ok := c.Encounter(77)
	if !ok || len(e.BossUnits) != 2 || e.BossUnits[0].Name != "Synthetic left boss" {
		t.Fatalf("boss override: %+v", e)
	}
	if r := c.OutputTargets(77); r.Mode != "boss_and_units" || len(r.Units) != 1 || r.Units[0].Name != "Synthetic required add" {
		t.Fatalf("output override: %+v", r)
	}
	if c.OutputTargets(78).Mode != "boss_only" {
		t.Fatal("per-boss override leaked")
	}
	if len(e.Mechanics) != 1 || e.Mechanics[0].EvidenceSource.Kind != "real_log" {
		t.Fatal("validated mechanic not exposed")
	}
}

func TestPersonalSupportNotCredited(t *testing.T) {
	c := load(t, seedDir)
	want := map[string]bool{"Improved Scorch": false, "Winter's Chill": false, "Improved Shadow Bolt": false, "Hemorrhage": false}
	for _, e := range c.Support.Effects {
		if _, ok := want[e.Spell.Name]; ok {
			if e.Scope != "personal" || e.Credit {
				t.Fatalf("personal buff credited: %+v", e)
			}
			want[e.Spell.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Fatalf("missing %s", name)
		}
	}
}

func TestUncertainSupportIsNotInvented(t *testing.T) {
	c := load(t, seedDir)
	found := false
	for _, e := range c.Support.Effects {
		if e.Spell.Name == "Furious Howl" {
			found = true
			if e.Scope != "unknown" || e.Credit || e.Stacks != nil {
				t.Fatalf("conflicting scope promoted to fact: %+v", e)
			}
		}
		if e.Spell.Name == "Thunder Clap" && e.Stacks != nil {
			t.Fatal("unsourced stacking must remain unknown")
		}
	}
	if !found {
		t.Fatal("missing scope conflict")
	}
	dir := copyData(t)
	change(t, dir, "support.json", func(d map[string]any) {
		e := d["effects"].([]any)[0].(map[string]any)
		e["scope"] = "unknown"
		e["credit"] = true
	})
	refused(t, dir, "support.json")
}

func TestNoItemDatabase(t *testing.T) {
	if got := cfg.ItemLabel(123456789); got != "123456789" {
		t.Fatalf("unknown item must remain its id, got %q", got)
	}
}
