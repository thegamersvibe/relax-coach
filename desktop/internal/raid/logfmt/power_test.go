package logfmt_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
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
			{"?", "1|2", "bad", model.Opt{}, model.Opt{}, model.Opt{}},
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
