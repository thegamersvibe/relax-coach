package model_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

// The step-0 addendum (PHASE1-BRIEF.md): one fight built with every new field, so later lanes need no model edits.
func TestFightWithEveryNewField(t *testing.T) {
	tank, healer := modeltest.Player("Alia-Stone-PvP-EU"), modeltest.Player("Bram-Oak-Hill-PvP-EU")
	boss := modeltest.Creature("Ash Warden", 42)
	pet := modeltest.Pet("Cinder", healer)
	enc := model.Encounter{ID: 7, Name: "Ash Warden", Difficulty: 1, GroupSize: 5,
		InstanceID: model.Some(329), Success: model.Some(1), FightMs: model.Some(61000)}
	start, end := modeltest.Encounter(enc, 1000, 62000)
	start.Offset, start.Len = 10, 120
	combatant := model.Combatant{PlayerID: tank.ID, Stats: []int64{1}, Talents: [][3]int64{{1, 2, 3}},
		Gear: []model.GearSlot{{ItemID: 1, ItemLevel: 2}}, Auras: []model.Aura{{CasterID: tank.ID, SpellID: 3, Stacks: 1}}, Raw: []string{"x"}}
	events := []model.Event{
		modeltest.At(900).Zone(329, "Ash Keep", 1).Pos(0, 10).Build(),
		modeltest.At(950).Info(combatant).Build(),
		start,
		modeltest.At(2000).Src(tank).Dst(boss).Spell(5, "Strike").Damage(100, -1).Pos(130, 90).Build(),
		modeltest.At(3000).Src(healer).Dst(tank).Spell(6, "Mend").Heal(80, 20).Build(),
		modeltest.At(4000).Src(boss).Dst(tank).Miss("DODGE").Build(),
		modeltest.At(5000).Src(healer).Dst(tank).Spell(8, "Ward").Aura(model.AuraApplied, "BUFF").Build(),
		modeltest.At(6000).Src(pet).Dst(boss).Owner(healer.ID).Damage(5, -1).Build(),
		end,
	}
	f := modeltest.Fight("f1", enc, events...)
	f.Partial = true
	f.Players = []model.Actor{tank, healer}
	f.Combatants = []model.Combatant{combatant}
	f.Owners = map[string]string{pet.ID: healer.ID}
	f.Unowned = []string{"Pet-0-unknown"}
	f.BossUnits = []string{boss.ID}
	f.Roles = map[string]string{tank.ID: "tank", healer.ID: "healer"}

	fv := reflect.ValueOf(f)
	for i := 0; i < fv.NumField(); i++ {
		if fv.Field(i).IsZero() {
			t.Errorf("Fight.%s is not set by the builders", fv.Type().Field(i).Name)
		}
	}
	if !f.Boss || f.InstanceID != 329 || f.DifficultyID != 1 || f.DurationMs != model.Some(61000) || f.StartMs != 900 || f.EndMs != 62000 {
		t.Errorf("fight from the encounter: %+v", f)
	}
	// Every field added to Event is carried by at least one event of the fight.
	for _, name := range []string{"Offset", "Len", "Overheal", "MissType", "AuraType", "InfoOwnerID", "Enc", "Zone", "Info"} {
		set := false
		for _, e := range f.Events {
			if !reflect.ValueOf(e).FieldByName(name).IsZero() {
				set = true
			}
		}
		if !set {
			t.Errorf("Event.%s is not carried by any event", name)
		}
	}
	night := model.RaidNight{Fights: []model.Fight{f}, Roster: f.Players, Partial: true, Logger: tank, StartMs: 900, OffsetH: 1}
	run := model.DungeonRun{Fights: []model.Fight{f}, Roster: f.Players, InstanceID: 329, Partial: true, Logger: tank, StartMs: 900, OffsetH: 1}
	for _, v := range []reflect.Value{reflect.ValueOf(night), reflect.ValueOf(run)} {
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).IsZero() {
				t.Errorf("%s.%s is not set", v.Type().Name(), v.Type().Field(i).Name)
			}
		}
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
