package logfmt_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestInvalidActorFlagsSkipped(t *testing.T) {
	cases := []struct {
		name      string
		withSpell bool
		flagIndex int
	}{
		{"source", false, 2}, {"destination", false, 6},
		{"swing-caster", false, 10}, {"spell-caster", true, 13},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, bad := range []string{
				"1297", "4294967295", "0b10100010001", "0o2421", "0511",
				"0x5_11", "0x_511", "0X511", "", "0x", "nil",
				"+1297", "-1297", "-1", "+0x511", "-0x511", "0x+511", "0x-511",
				"0x100000000", "0xFFFFFFFFFF", "4294967296", "abc", "0xg",
			} {
				t.Run(bad, func(t *testing.T) {
					f := absorbFields(tc.withSpell)
					f[tc.flagIndex] = fixture.Field(bad)
					row := line(t, "SPELL_ABSORBED", f...)
					events, r := decode(row + damage(t, "SPELL_DAMAGE", false))
					balanced(t, r)
					if len(events) != 1 || r.Parsed != 1 || r.Skipped["invalid actor flags"] != 1 || r.EventCounts["SPELL_ABSORBED"] != 1 {
						t.Fatalf("bad flags yielded: events=%d report=%v", len(events), r)
					}
					if events[0].Line != 2 || events[0].Offset != int64(len(row)) {
						t.Fatal("bad flags lost recovery offsets")
					}
				})
			}
		})
	}
	// Known opaque combat events still have common actors; MAP_CHANGE does not.
	for _, name := range []string{"SPELL_CREATE", "ENCHANT_APPLIED", "ENCHANT_REMOVED", "SPELL_CAST_START"} {
		f := append(common(), spell()...)
		f[2] = "abc"
		events, r := decode(line(t, name, f...))
		balanced(t, r)
		if len(events) != 0 || r.Skipped["invalid actor flags"] != 1 {
			t.Errorf("%s invalid flags accepted", name)
		}
	}
}

func TestValidActorFlagsPreserved(t *testing.T) {
	for _, tc := range []struct {
		wire  string
		flags uint32
	}{
		{"0x0", 0}, {"0x511", 0x511}, {"0x10a48", 0x10a48}, {"0x10A48", 0x10a48},
		{"0xabcdef", 0xabcdef}, {"0xABCDEF", 0xabcdef}, {"0xAbCdEf", 0xabcdef},
		{"0xffffffff", 0xffffffff}, {"0xFFFFFFFF", 0xffffffff},
	} {
		f := absorbFields(false)
		f[2], f[6], f[10] = fixture.Field(tc.wire), fixture.Field(tc.wire), fixture.Field(tc.wire)
		events, r := decode(line(t, "SPELL_ABSORBED", f...))
		if len(events) != 1 || len(r.Skipped) != 0 {
			t.Fatalf("valid flags %s rejected", tc.wire)
		}
		e := events[0]
		if e.Src.Flags != tc.flags || e.Dst.Flags != tc.flags || e.Caster.Flags != tc.flags {
			t.Errorf("valid flags %s changed", tc.wire)
		}
	}
}

var cato = model.Actor{ID: "Player-999-FA003ABC", Name: "Cato-PvP-EU", Flags: 0x514}

func absorbFields(withSpell bool) []fixture.Field {
	f := common()
	if withSpell {
		f = append(f, spell()...)
	}
	f = append(f, fixture.Actor(cato)...)
	return append(f, "900003", fixture.Text("Practice Shield"), "0x2", "37", "40", "1")
}

func TestAbsorbCasterBothWidths(t *testing.T) {
	for _, tc := range []struct {
		name      string
		withSpell bool
		spellID   int64
		spellName string
	}{{"swing", false, 0, ""}, {"spell", true, 900001, "Practice Bolt"}} {
		t.Run(tc.name, func(t *testing.T) {
			events, r := decode(line(t, "SPELL_ABSORBED", absorbFields(tc.withSpell)...))
			balanced(t, r)
			if len(events) != 1 || r.Parsed != 1 || len(r.Skipped) != 0 {
				t.Fatalf("events=%d report=%v", len(events), r)
			}
			e := events[0]
			if e.Kind != model.Absorb || e.Caster != cato || e.Src != alia || e.Dst != bram || e.SpellID != tc.spellID || e.SpellName != tc.spellName || e.ExtraSpellID != model.Some(900003) || e.Amount != model.Some(37) || !e.Crit || e.InfoOwnerID != "" || len(e.Raw) != 0 {
				t.Fatalf("absorb attribution: %+v", e)
			}
		})
	}
	for _, name := range []string{"SWING_DAMAGE", "SPELL_DAMAGE"} {
		events, _ := decode(damage(t, name, true))
		if len(events) != 1 || events[0].Caster != (model.Actor{}) {
			t.Fatal("non-absorb has a shield caster")
		}
	}
}
