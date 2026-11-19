package model_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

// allSet reports every field of v that is still its zero value.
func allSet(t *testing.T, v any) {
	t.Helper()
	rv := reflect.ValueOf(v)
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).IsZero() {
			t.Errorf("%s.%s is not set by the builders", rv.Type().Name(), rv.Type().Field(i).Name)
		}
	}
}

// The step-0 addendum (PHASE1-BRIEF.md): one fight built with every field through the builders, so later
// lanes need no model edits. No field is set by hand.
func TestFightWithEveryNewField(t *testing.T) {
	tank, healer := modeltest.Player("Alia-Stone-PvP-EU"), modeltest.Player("Bram-Oak-Hill-PvP-EU")
	boss := modeltest.Creature("Ash Warden", 42)
	pet := modeltest.Pet("Cinder", healer)
	enc := model.Encounter{ID: 7, Name: "Ash Warden", Difficulty: 1, GroupSize: 5,
		InstanceID: model.Some(329), Success: model.Some(1), FightMs: model.Some(61000)}
	start, end := modeltest.Encounter(enc, 1000, 62000)
	combatant := model.Combatant{PlayerID: tank.ID, Stats: []int64{1}, Talents: [][3]int64{{1, 2, 3}},
		Gear: []model.GearSlot{{ItemID: 1, ItemLevel: 2}}, Auras: []model.Aura{{CasterID: tank.ID, SpellID: 3, Stacks: 1}}, Raw: []string{"x"}}
	events := []model.Event{
		modeltest.At(800).Pos(5, 5).Build(), // the fight's first line, at byte 5
		modeltest.At(900).Zone(329, "Ash Keep", 1).Build(),
		modeltest.At(950).Info(combatant).Build(),
		start,
		modeltest.At(2000).Src(tank).Dst(boss).Spell(5, "Strike").Damage(100, -1).Build(),
		modeltest.At(2500).Src(boss).Dst(tank).Damage(30, -1).Landed().Build(),
		modeltest.At(3000).Src(healer).Dst(tank).Spell(6, "Mend").Heal(80, 20).Build(),
		modeltest.At(4000).Src(boss).Dst(tank).Miss("DODGE").Build(),
		modeltest.At(5000).Src(healer).Dst(tank).Spell(8, "Ward").Aura(model.AuraApplied, "BUFF").Build(),
		modeltest.At(6000).Src(pet).Dst(boss).Owner(healer.ID).Damage(5, -1).Build(),
		modeltest.At(7000).Src(tank).Spell(9, "Rage").Power(40, 100, 1).Build(),
		modeltest.At(8000).Dst(healer).Died(1).Build(),
		modeltest.At(8500).Src(boss).Dst(tank).Absorb(healer, 17, 250).Build(),
		end,
		modeltest.At(62001).Pos(1000, 50).Build(), // the last line, ending at byte 1050
	}
	f := modeltest.NewFight("f1", enc, events...).Partial().Players(tank, healer).Combatants(combatant).
		Owner(pet, healer).Unowned("Pet-0-unknown").BossUnits(boss).Role(tank, "tank").Role(healer, "healer").
		Spec(tank, "WARRIOR.protection").Class(tank, "WARRIOR").Class(healer, "PRIEST").Range().Build()
	allSet(t, f)
	if !f.Boss || f.InstanceID != 329 || f.DifficultyID != 1 || f.DurationMs != model.Some(61000) || f.StartMs != 800 || f.EndMs != 62001 {
		t.Errorf("fight from the encounter: %+v", f)
	}
	if f.Offset != 5 || f.Len != 1045 || f.Owners[pet.ID] != healer.ID || f.Roles[healer.ID] != "healer" ||
		f.Specs[tank.ID] != "WARRIOR.protection" || f.BossUnits[0] != boss.ID || len(f.Players) != 2 || f.Unowned[0] != "Pet-0-unknown" {
		t.Errorf("derived fields: %+v", f)
	}
	// Every Event field is carried by at least one event of the fight: generic, so a new field needs a builder
	// (or an entry in handOnly) before this passes.
	et := reflect.TypeOf(model.Event{})
	for i := 0; i < et.NumField(); i++ {
		name, set := et.Field(i).Name, false
		for _, e := range f.Events {
			if !reflect.ValueOf(e).Field(i).IsZero() {
				set = true
			}
		}
		if !set && !handOnly[name] {
			t.Errorf("Event.%s is not carried by any event", name)
		}
	}
	run := modeltest.NewRun(f).Roster(tank, healer).Partial().Logger(tank).Start(900, 1).
		Header("1.15.7", 22, true).Source("ab12", 4096)
	allSet(t, run.Night())
	allSet(t, run.Dungeon(329))
	for _, src := range []model.Source{run.Night().Source, run.Dungeon(329).Source} {
		allSet(t, src)
		if src != (model.Source{SHA256: "ab12", Bytes: 4096}) {
			t.Errorf("source on both run types: %+v", src)
		}
	}
	allSet(t, enc)
	allSet(t, *f.Events[1].Zone)
	allSet(t, combatant)
	if ab := f.Events[12]; ab.Kind != model.Absorb || ab.Caster != healer || ab.Src != boss || ab.Dst != tank ||
		ab.ExtraSpellID != model.Some(17) || ab.Amount != model.Some(250) || ab.SpellID != 0 {
		t.Errorf("absorb: %+v", ab)
	}
	if f.Classes[tank.ID] != "WARRIOR" || f.Classes[healer.ID] != "PRIEST" || len(f.Classes) != 2 {
		t.Errorf("classes: %v", f.Classes)
	}
	if !reflect.DeepEqual(f.Events, events) || f.Combatants[0].PlayerID != tank.ID || f.Events[4].Amount != model.Some(100) ||
		f.Events[1].Zone.Difficulty != 1 || f.Events[2].Info.PlayerID != tank.ID {
		t.Errorf("events and combatant carried unchanged: %+v", f.Combatants)
	}
	if n := run.Night(); n.StartMs != 900 || n.OffsetH != 1 || n.Build != "1.15.7" || n.LogVersion != 22 || !n.Advanced || n.Source.Bytes != 4096 {
		t.Errorf("night: %+v", n)
	}
	if d := run.Dungeon(329); d.InstanceID != 329 || d.Source.SHA256 != "ab12" || d.Logger != tank || len(d.Roster) != 2 {
		t.Errorf("dungeon: %+v", d)
	}
	if modeltest.NewRun(f).Night().StartMs != f.StartMs {
		t.Error("a run starts at its first fight by default")
	}
	// Deaths: an unresolved one (no owner names, a suggested cause) and a charged one (names, no suggestion).
	d := modeltest.DeathOf(tank, "f1", 4000, "unresolved").Evidence(3900, 4000).Confidence("low").
		Owner("undecided").Suggest("avoidable").Build()
	if d.Player != tank || d.FightID != "f1" || d.TimeMs != 4000 || d.Cause != "unresolved" || len(d.Evidence) != 2 ||
		d.Confidence != "low" || d.OwnerKind != "undecided" || len(d.Owners) != 0 || d.SuggestedCause != "avoidable" {
		t.Errorf("unresolved death: %+v", d)
	}
	d2 := modeltest.DeathOf(healer, "f1", 5000, "unhealed").Owner("healers", "Bram-Oak-Hill-PvP-EU", "Cato-PvP-EU").Build()
	if d2.OwnerKind != "healers" || len(d2.Owners) != 2 || d2.Owners[1] != "Cato-PvP-EU" || d2.SuggestedCause != "" {
		t.Errorf("charged death: %+v", d2)
	}
	if n := reflect.TypeOf(model.Death{}).NumField(); n != 9 {
		t.Errorf("Death has %d fields; extend the death checks above for the new one", n)
	}
}

// The lane-to-lane result types, built only through builders, every field set and checked.
func TestResultTypesThroughBuilders(t *testing.T) {
	p := modeltest.Player("Cato-PvP-EU")
	allSet(t, modeltest.Wipe("f1", 2, 3, 4))
	if w := modeltest.Wipe("f1", 2, 3, 4); w.FightID != "f1" || w.FirstDeath != 2 || len(w.Following) != 2 || w.Following[1] != 4 {
		t.Errorf("wipe: %+v", w)
	}
	ev := modeltest.Evidence("Stood in fire 3 times.", []int64{1200, 3400}, "hits", 3)
	allSet(t, ev)
	if ev.Counts["hits"] != 3 || ev.AtMs[1] != 3400 || ev.Text != "Stood in fire 3 times." {
		t.Errorf("evidence: %+v", ev)
	}
	part := modeltest.Part("mechanics", 873, ev)
	allSet(t, part)
	if !reflect.DeepEqual(part.Evidence, ev) {
		t.Errorf("part keeps the whole evidence: %+v", part.Evidence)
	}
	if part.Tenths != model.Some(873) || part.Name != "mechanics" || modeltest.Unmeasured("utility").Tenths.Present {
		t.Errorf("parts: %+v", part)
	}
	fs := modeltest.FightScore("f1", p, "dps").Score(912).Coverage(85).Parts(part, modeltest.Unmeasured("utility")).
		Flags("partialFight").Build()
	allSet(t, fs)
	if fs.PlayerID != p.ID || fs.Tenths != model.Some(912) || fs.Coverage != 85 || len(fs.Parts) != 2 || fs.Flags[0] != "partialFight" || fs.Role != "dps" {
		t.Errorf("fight score: %+v", fs)
	}
	if nd := modeltest.FightScore("f1", p, "dps").Coverage(0).Build(); nd.Tenths.Present {
		t.Error("no Score means not enough data")
	}
	ns := modeltest.NightScore(p, "spec").Score(880).Coverage(90).Rank(1).Fights(fs).Build()
	allSet(t, ns)
	if ns.Rank != model.Some(1) || ns.Baseline != "spec" || ns.Tenths != model.Some(880) || ns.Coverage != 90 || len(ns.Fights) != 1 {
		t.Errorf("night score: %+v", ns)
	}
	if modeltest.NightScore(p, "spec").Score(1).Build().Rank.Present {
		t.Error("no Rank means unranked (dungeon run, not enough data)")
	}
	pr := modeltest.Prep("Cato-PvP-EU", "MAGE", 60, 1, "1.8.1").Played(3600).Consumable("flask", 13512, 2).
		ConsumablesUnreadable().Gear(model.PrepGear{Slot: 1, WornID: 111, BagID: 12345}).GearUnreadable().
		Reason("ready").Clock(1000, 1002).Build()
	allSet(t, pr)
	allSet(t, pr.Consumables[0])
	allSet(t, pr.BetterGear[0])
	if pr.Name != "Cato-PvP-EU" || pr.Class != "MAGE" || pr.Level != 60 || pr.Protocol != 1 || pr.AddonVersion != "1.8.1" ||
		pr.PlayedAtLevelS != model.Some(3600) || pr.Reason != "ready" || pr.SenderClock != 1000 || pr.Stamped != 1002 {
		t.Errorf("prep: %+v", pr)
	}
	if !reflect.DeepEqual(pr.Consumables, []model.PrepConsumable{{Category: "flask", ItemID: 13512, Count: 2}}) ||
		!reflect.DeepEqual(pr.BetterGear, []model.PrepGear{{Slot: 1, WornID: 111, BagID: 12345}}) {
		t.Errorf("prep rows: %+v %+v", pr.Consumables, pr.BetterGear)
	}
	g := modeltest.Prep("Cato-PvP-EU", "MAGE", 60, 1, "1.8.1").Build()
	if g.GearChecked || g.GearUnreadable || g.ConsumablesUnreadable || g.PlayedAtLevelS.Present || g.Reason != "" {
		t.Error("defaults: gear not checked, nothing unreadable, played unreadable until set")
	}
	if c := modeltest.Prep("Cato-PvP-EU", "MAGE", 60, 1, "1.8.1").Gear().Build(); !c.GearChecked || c.GearUnreadable || len(c.BetterGear) != 0 {
		t.Error("checked with nothing better is not unreadable")
	}
}

// Evidence takes any integer type and refuses a mistake loudly.
func TestEvidenceBuilderStrict(t *testing.T) {
	ev := modeltest.Evidence("x", nil, "a", int8(1), "b", uint16(2), "c", int64(3))
	if !reflect.DeepEqual(ev.Counts, map[string]int64{"a": 1, "b": 2, "c": 3}) {
		t.Errorf("counts: %v", ev.Counts)
	}
	cases := map[string][]any{"has no value": {"a", 1, "b"}, "is not a string": {1, 1}, "is not an integer": {"a", 1.5}}
	for name, args := range cases {
		func() {
			defer func() {
				if msg, _ := recover().(string); !strings.Contains(msg, name) {
					t.Errorf("want a panic naming the mistake (%s), got %q", name, msg)
				}
			}()
			modeltest.Evidence("x", nil, args...)
		}()
	}
}

// Event fields with no builder: the decoder fills them, and tests that need them set them by hand.
var handOnly = map[string]bool{"OffsetH": true, "Line": true, "Absorbed": true, "Crit": true,
	"HP": true, "HPMax": true, "InfoUnit": true, "X": true, "Y": true, "ExtraSpellID": true, "Raw": true}

// Each builder sets exactly what it names (pins the builders other lanes rely on).
func TestEventBuildersPinned(t *testing.T) {
	e := modeltest.At(1).Spell(77, "Cleave").Pos(1234, 56).Build()
	if e.SpellID != 77 || e.SpellName != "Cleave" || e.Offset != 1234 || e.Len != 56 {
		t.Errorf("spell and position: %+v", e)
	}
	z := modeltest.At(1).Zone(329, "Ash Keep", 2).Build().Zone
	if z == nil || z.InstanceID != 329 || z.Name != "Ash Keep" || z.Difficulty != 2 {
		t.Errorf("zone: %+v", z)
	}
	p := modeltest.At(1).Power(40, 100, 0).Build()
	if p.Power != model.Some(40) || p.PowerMax != model.Some(100) || p.PowerType != model.Some(0) {
		t.Errorf("power (type 0 is mana, present): %+v", p)
	}
	// A boss fight whose end line has no length: DurationMs stays unset, never end minus start.
	boss := model.Encounter{ID: 7, InstanceID: model.Some(329)}
	f := modeltest.Fight("b", boss, modeltest.At(1000).Build(), modeltest.At(9000).Build())
	if !f.Boss || f.DurationMs.Present {
		t.Errorf("boss without a server length: %+v", f)
	}
}

// Each encounter line carries only its own fields: no instance id on the end line; no success or length on the start.
func TestEncounterLinesCarryTheirOwnFields(t *testing.T) {
	enc := model.Encounter{ID: 7, Name: "Ash Warden", Difficulty: 1, GroupSize: 5,
		InstanceID: model.Some(329), Success: model.Some(1), FightMs: model.Some(61000)}
	s, e := modeltest.Encounter(enc, 1000, 62000)
	if s.Kind != model.EncounterStart || e.Kind != model.EncounterEnd || s.TimeMs != 1000 || e.TimeMs != 62000 {
		t.Fatalf("kinds or times: %+v %+v", s, e)
	}
	if s.Enc.InstanceID != model.Some(329) || s.Enc.Success.Present || s.Enc.FightMs.Present {
		t.Errorf("start line: %+v", *s.Enc)
	}
	if e.Enc.InstanceID.Present || e.Enc.Success != model.Some(1) || e.Enc.FightMs != model.Some(61000) {
		t.Errorf("end line: %+v", *e.Enc)
	}
	if s.Enc == e.Enc {
		t.Error("the two lines share one Encounter value")
	}
	// A trash pull is not a boss fight, and its length is never derived from start and end.
	trash := modeltest.Fight("t1", model.Encounter{}, modeltest.At(1).Build(), modeltest.At(9).Build())
	if trash.Boss || trash.DurationMs.Present {
		t.Errorf("trash: %+v", trash)
	}
}

func TestEventBuildersSetTheirKind(t *testing.T) {
	a := modeltest.Player("Cato-PvP-EU")
	cases := map[string]struct {
		e    model.Event
		kind model.Kind
	}{
		"heal": {modeltest.At(1).Dst(a).Heal(5, 1).Build(), model.Heal},
		"miss": {modeltest.At(1).Dst(a).Miss("ABSORB").Build(), model.Missed},
		"aura": {modeltest.At(1).Dst(a).Aura(model.AuraRemoved, "DEBUFF").Build(), model.AuraRemoved},
		"zone": {modeltest.At(1).Zone(0, "Open World", 0).Build(), model.ZoneChange},
		"info": {modeltest.At(1).Info(model.Combatant{PlayerID: a.ID}).Build(), model.CombatantInfo},
	}
	for name, c := range cases {
		if c.e.Kind != c.kind {
			t.Errorf("%s: kind %s, want %s", name, c.e.Kind, c.kind)
		}
	}
	if h := cases["heal"].e; h.Amount != model.Some(5) || h.Overheal != model.Some(1) {
		t.Errorf("heal amounts: %+v", h)
	}
	if z := cases["zone"].e.Zone; z == nil || z.InstanceID != 0 || z.Name != "Open World" {
		t.Errorf("zone: %+v", z)
	}
}
