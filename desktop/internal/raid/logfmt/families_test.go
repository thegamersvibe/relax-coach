package logfmt_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestCombatFamilies(t *testing.T) {
	cases := []struct {
		name string
		kind model.Kind
		tail []fixture.Field
		adv  bool
	}{
		{"SPELL_CAST_START", model.CastStart, nil, false}, {"SPELL_CAST_SUCCESS", model.CastSuccess, nil, true}, {"SPELL_CAST_FAILED", model.CastFailed, []fixture.Field{fixture.Text("Not ready")}, false},
		{"SPELL_HEAL", model.Heal, nums(80, 90, 10, 5, 1), true}, {"SPELL_PERIODIC_HEAL", model.Heal, nums(80, 90, 10, 5, 1), true},
		{"SPELL_ENERGIZE", model.Energize, []fixture.Field{"8.0000", "0.0000", "0", "50"}, true}, {"SPELL_PERIODIC_ENERGIZE", model.Energize, nums(8, 0, 0, 50), true}, {"SPELL_PERIODIC_LEECH", model.Energize, nums(8, 0, 0, 50), true}, {"SPELL_DRAIN", model.Energize, nums(8, 0, 0, 50), true}, {"SPELL_PERIODIC_DRAIN", model.Energize, nums(8, 0, 0, 50), true},
		{"SPELL_AURA_APPLIED", model.AuraApplied, []fixture.Field{"BUFF"}, false}, {"SPELL_AURA_REMOVED", model.AuraRemoved, []fixture.Field{"DEBUFF"}, false}, {"SPELL_AURA_REFRESH", model.AuraRefresh, []fixture.Field{"BUFF", "2"}, false},
		{"SPELL_AURA_APPLIED_DOSE", model.AuraDose, []fixture.Field{"DEBUFF", "2"}, false}, {"SPELL_AURA_REMOVED_DOSE", model.AuraDose, []fixture.Field{"BUFF", "2"}, false}, {"SPELL_AURA_BROKEN", model.AuraBroken, []fixture.Field{"DEBUFF"}, false},
		{"SPELL_AURA_BROKEN_SPELL", model.AuraBroken, []fixture.Field{"900002", fixture.Text("Practice Break"), "0x1", "DEBUFF"}, false},
		{"SPELL_INTERRUPT", model.Interrupt, []fixture.Field{"900002", fixture.Text("Practice Break"), "0x1"}, false},
		{"SPELL_DISPEL", model.Dispel, []fixture.Field{"900002", fixture.Text("Practice Break"), "0x1", "DEBUFF"}, false},
		{"SPELL_SUMMON", model.Summon, nil, false}, {"SPELL_RESURRECT", model.Resurrect, nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := append(common(), spell()...)
			if tc.adv {
				f = append(f, advanced(alia.ID)...)
			}
			f = append(f, tc.tail...)
			events, r := decode(line(t, tc.name, f...))
			balanced(t, r)
			if len(events) != 1 {
				t.Fatal(r)
			}
			e := events[0]
			if e.Kind != tc.kind || e.Src != alia || e.Dst != bram || e.SpellID != 900001 || len(e.Raw) != 0 {
				t.Fatal(e)
			}
			switch tc.kind {
			case model.Heal:
				if e.Amount != model.Some(80) || e.Overheal != model.Some(10) || e.Absorbed != model.Some(5) || !e.Crit {
					t.Fatal(e)
				}
			case model.Energize:
				if e.Amount != model.Some(8) {
					t.Fatal(e)
				}
			case model.AuraApplied, model.AuraRemoved, model.AuraRefresh, model.AuraDose:
				if e.AuraType != string(tc.tail[0]) {
					t.Fatal(e)
				}
			}
			if tc.kind == model.AuraDose && e.Amount != model.Some(2) {
				t.Fatal(e)
			}
			if tc.kind == model.Interrupt || tc.kind == model.Dispel || tc.name == "SPELL_AURA_BROKEN_SPELL" {
				if e.ExtraSpellID != model.Some(900002) {
					t.Fatal(e)
				}
			}
			if (tc.kind == model.Dispel || tc.kind == model.AuraBroken) && e.AuraType != "DEBUFF" {
				t.Fatal(e)
			}
			// Advanced presence is a width decision even for non-damage families.
			if tc.adv {
				short := append(append(common(), spell()...), tc.tail...)
				s, r := decode(line(t, tc.name, short...))
				if len(s) != 1 || s[0].HP.Present {
					t.Fatal(r, s)
				}
			}
		})
	}
}

func TestMissAndAbsorbWidths(t *testing.T) {
	for _, tc := range []struct {
		name       string
		widths     []int
		spell, tag bool
	}{
		{"SWING_MISSED", []int{11, 12, 14}, false, false}, {"RANGE_MISSED", []int{14, 17}, true, false},
		{"SPELL_MISSED", []int{15, 16, 18}, true, true}, {"SPELL_PERIODIC_MISSED", []int{15, 16, 18}, true, true}, {"DAMAGE_SHIELD_MISSED", []int{15, 16}, true, true},
	} {
		for _, width := range tc.widths {
			f := common()
			if tc.spell {
				f = append(f, spell()...)
			}
			base := len(f) + 1
			reason := "DODGE"
			if width-base >= 4 {
				reason = "ABSORB"
			}
			f = append(f, fixture.Field(reason), fixture.Nil)
			for len(f) < width-1 {
				f = append(f, "17")
			}
			if tc.tag {
				f[len(f)-1] = "ST"
			}
			events, r := decode(line(t, tc.name, f...))
			balanced(t, r)
			if len(events) != 1 || events[0].Kind != model.Missed || events[0].MissType != reason {
				t.Fatal(tc.name, width, r)
			}
			if reason == "ABSORB" && events[0].Absorbed != model.Some(17) {
				t.Fatal(events[0])
			}
		}
	}
	for _, withSpell := range []bool{false, true} {
		f := common()
		if withSpell {
			f = append(f, spell()...)
		}
		f = append(f, fixture.Actor(model.Actor{ID: "Player-999-FA003ABC", Name: "Cato-PvP-EU", Flags: 0x512})...)
		f = append(f, "900003", fixture.Text("Practice Shield"), "0x2", "37", "40", "1")
		events, r := decode(line(t, "SPELL_ABSORBED", f...))
		balanced(t, r)
		if len(events) != 1 || events[0].Kind != model.Absorb || events[0].ExtraSpellID != model.Some(900003) || events[0].Amount != model.Some(37) || !events[0].Crit || len(events[0].Raw) != 0 {
			t.Fatal(r, events)
		}
		if events[0].Src != alia || events[0].Dst != bram {
			t.Fatal(events[0])
		}
		_, bad := decode(line(t, "SPELL_ABSORBED", append(f, "extra")...))
		if bad.Skipped["unknown field count"] != 1 {
			t.Fatal(bad)
		}
	}
}

func TestDeathsAndKnownOther(t *testing.T) {
	for _, flag := range []int64{0, 1} {
		events, r := decode(line(t, "UNIT_DIED", append(common(), fixture.Number(flag))...))
		if len(events) != 1 || events[0].DiedFlag != model.Some(flag) || events[0].IsDeath() != (flag == 0) {
			t.Fatal(r, events)
		}
	}
	events, r := decode(line(t, "PARTY_KILL", append(common(), "0")...))
	if len(events) != 1 || events[0].Kind != model.PartyKill || events[0].IsDeath() {
		t.Fatal(r, events)
	}
	for name, width := range map[string]int{"SPELL_EXTRA_ATTACKS": 13, "SPELL_INSTAKILL": 13, "SPELL_CREATE": 12, "ENCHANT_APPLIED": 12, "ENCHANT_REMOVED": 12, "MAP_CHANGE": 7} {
		f := make([]fixture.Field, width-1)
		for i := range f {
			f[i] = "0"
		}
		events, r := decode(line(t, name, f...))
		if len(events) != 1 || events[0].Kind != model.Other || len(events[0].Raw) != width || len(r.UnknownEvents) != 0 {
			t.Fatal(r, events)
		}
		_, r = decode(line(t, name, append(f, "0")...))
		if r.Skipped["unknown field count"] != 1 {
			t.Fatal(r)
		}
	}
}
