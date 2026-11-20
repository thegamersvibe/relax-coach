package logfmt_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"strings"
	"testing"
)

func TestAdvancedPowerFields(t *testing.T) {
	for _, tc := range []struct {
		name       string
		spell, env bool
		tail       []fixture.Field
	}{
		{"SWING_DAMAGE", false, false, nums(7, 9, -1, 1, 0, 0, 0, 0, 0, 0)},
		{"SWING_DAMAGE_LANDED", false, false, nums(7, 9, -1, 1, 0, 0, 0, 0, 0, 0)},
		{"SPELL_DAMAGE", true, false, append(nums(7, 9, -1, 1, 0, 0, 0, 0, 0, 0), "ST")},
		{"SPELL_HEAL", true, false, nums(7, 9, 2, 0, 0)},
		{"SPELL_CAST_SUCCESS", true, false, nil},
		{"SPELL_ENERGIZE", true, false, nums(7, 0, 0, 99)},
		{"ENVIRONMENTAL_DAMAGE", false, true, nums(7, 9, -1, 1, 0, 0, 0, 0, 0, 0)},
	} {
		for _, values := range []struct {
			typ, cur, max fixture.Field
			t, c, m       model.Opt
		}{
			{"0", "0", "900", model.Some(0), model.Some(0), model.Some(900)},
			{"3", "17", "100", model.Some(3), model.Some(17), model.Some(100)},
			{"nil", "nil", "nil", model.Opt{}, model.Opt{}, model.Opt{}},
			{"?", "x", "bad", model.Opt{}, model.Opt{}, model.Opt{}},
		} {
			f := common()
			if tc.spell {
				f = append(f, spell()...)
			}
			block := advanced(bram.ID)
			// Independent synthetic wire positions: do not depend on the fixture's
			// new Power fields to prove the decoder mapping.
			block[8], block[9], block[13] = "81", "91", "131"
			block[10], block[11], block[12] = values.typ, values.cur, values.max
			f = append(f, block...)
			if tc.env {
				f = append(f, "FIRE")
			}
			f = append(f, tc.tail...)
			events, r := decode(line(t, tc.name, f...))
			if len(events) != 1 {
				t.Fatal(tc.name, r)
			}
			e := events[0]
			if e.PowerType != values.t || e.Power != values.c || e.PowerMax != values.m || e.InfoUnit != bram.ID {
				t.Fatalf("%s power=(%v,%v,%v) want=(%v,%v,%v)", tc.name, e.PowerType, e.Power, e.PowerMax, values.t, values.c, values.m)
			}
		}
	}
	e, r := decode(damage(t, "SPELL_DAMAGE", false))
	if len(e) != 1 || e[0].Power.Present || e[0].PowerMax.Present || e[0].PowerType.Present {
		t.Fatal(r, e)
	}
}

// Two power types in one field (energy and combo points): the first value of each field is kept, the second
// type is not stored, and the line is counted in the parse report.
func TestTwoPowerTypesKeepTheFirst(t *testing.T) {
	f := append(common(), spell()...)
	block := advanced(bram.ID)
	block[10], block[11], block[12] = "3|4", "62|3", "115|5"
	two := line(t, "SPELL_CAST_SUCCESS", append(f, block...)...)
	block = advanced(bram.ID)
	block[10], block[11], block[12] = "3", "40", "100"
	one := line(t, "SPELL_CAST_SUCCESS", append(f, block...)...)
	events, r := decode(two + one)
	balanced(t, r)
	if len(events) != 2 {
		t.Fatalf("events: %d", len(events))
	}
	if e := events[0]; e.PowerType != model.Some(3) || e.Power != model.Some(62) || e.PowerMax != model.Some(115) {
		t.Errorf("first of each: %+v %+v %+v", e.PowerType, e.Power, e.PowerMax)
	}
	if e := events[1]; e.PowerType != model.Some(3) || e.Power != model.Some(40) || e.PowerMax != model.Some(100) {
		t.Errorf("single type unchanged: %+v", e)
	}
	if r.MultiPower != 1 || !strings.Contains(r.String(), "Lines with two power types: 1") {
		t.Errorf("counted once: %d", r.MultiPower)
	}
}
