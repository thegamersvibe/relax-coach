package logfmt_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func combatant() model.Combatant {
	c := model.Combatant{PlayerID: alia.ID, Stats: make([]int64, 24), Talents: [][3]int64{{901001, 902001, 3}, {901002, 902002, 5}}, Auras: []model.Aura{{CasterID: bram.ID, SpellID: 900004, Stacks: 2}}}
	for i := range c.Stats {
		c.Stats[i] = int64(i + 1)
	}
	for i := 0; i < 19; i++ {
		c.Gear = append(c.Gear, model.GearSlot{ItemID: int64(800001 + i), ItemLevel: 60, Extra: []string{"(17,0,0)", "[23,29]", "()"}})
	}
	return c
}
func TestCombatantNestedListsAndMalformedCollections(t *testing.T) {
	c := combatant()
	f := fixture.Combatant(c)
	good := line(t, "COMBATANT_INFO", f...)
	events, r := decode(good)
	balanced(t, r)
	if len(events) != 1 || events[0].Kind != model.CombatantInfo || events[0].Info == nil {
		t.Fatal(r)
	}
	got := events[0].Info
	if got.PlayerID != c.PlayerID || !reflect.DeepEqual(got.Stats, c.Stats) || !reflect.DeepEqual(got.Talents, c.Talents) || !reflect.DeepEqual(got.Gear, c.Gear) || !reflect.DeepEqual(got.Auras, c.Auras) || len(events[0].Raw) != 34 || len(got.Raw) != 34 || got.Raw[27] != "(0,0,0,0)" {
		t.Fatalf("%+v", got)
	}
	for _, bad := range []string{
		strings.Replace(good, "]", "", 1), strings.Replace(good, "(17,0,0)", "(17,0,0]", 1),
		line(t, "COMBATANT_INFO", append(append([]fixture.Field{}, f[:25]...), append([]fixture.Field{"[(1,2)]"}, f[26:]...)...)...),
	} {
		_, r := decode(bad)
		balanced(t, r)
		if r.Parsed != 0 || r.LinesRead != 1 {
			t.Fatal(r)
		}
	}
	// Empty lists are present empty data, not a missing field or zero-filled slot.
	c.Talents = nil
	c.Gear = nil
	c.Auras = nil
	events, r = decode(line(t, "COMBATANT_INFO", fixture.Combatant(c)...))
	if len(events) != 1 || len(events[0].Info.Talents) != 0 || len(events[0].Info.Gear) != 0 || len(events[0].Info.Auras) != 0 {
		t.Fatal(r)
	}
}

func TestEncounterAndZonePayloadsReportedWipeAndRaid(t *testing.T) {
	// A wipe and a forty-player raid are format-reported, not live observations.
	start := line(t, "ENCOUNTER_START", "700001", fixture.Text("Practice Keeper"), "1", "40", "555")
	for _, success := range []int64{0, 1} {
		end := line(t, "ENCOUNTER_END", "700001", fixture.Text("Practice Keeper"), "1", "40", fixture.Number(success), "45001")
		events, r := decode(start + end + line(t, "ZONE_CHANGE", "0", fixture.Text("Practice Town"), "0"))
		balanced(t, r)
		if len(events) != 3 {
			t.Fatal(r)
		}
		s, e, z := events[0], events[1], events[2]
		if s.Kind != model.EncounterStart || s.Enc == nil || s.Enc.ID != 700001 || s.Enc.Name != "Practice Keeper" || s.Enc.Difficulty != 1 || s.Enc.GroupSize != 40 || s.Enc.InstanceID != model.Some(555) || s.Enc.Success.Present || s.Enc.FightMs.Present {
			t.Fatal(s)
		}
		if e.Kind != model.EncounterEnd || e.Enc == nil || e.Enc.Success != model.Some(success) || e.Enc.FightMs != model.Some(45001) || e.Enc.InstanceID.Present {
			t.Fatal(e)
		}
		if z.Kind != model.ZoneChange || z.Zone == nil || z.Zone.InstanceID != 0 || z.Zone.Name != "Practice Town" || z.Zone.Difficulty != 0 {
			t.Fatal(z)
		}
		for _, e := range events {
			if len(e.Raw) != 0 {
				t.Fatal("retained raw", e.Kind)
			}
		}
	}
}
