package logfmt_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestAbsorbSelfShieldCasterPreserved(t *testing.T) {
	for _, tc := range []struct {
		name      string
		withSpell bool
		casterAt  int
	}{{"swing", false, 8}, {"spell", true, 11}} {
		t.Run(tc.name, func(t *testing.T) {
			f := absorbFields(tc.withSpell)
			copy(f[tc.casterAt:tc.casterAt+4], fixture.Actor(bram))
			events, r := decode(line(t, "SPELL_ABSORBED", f...))
			balanced(t, r)
			if len(events) != 1 || r.Parsed != 1 || len(r.Skipped) != 0 {
				t.Fatal("self-shield absorb was not decoded")
			}
			e := events[0]
			if e.Kind != model.Absorb || e.Caster != bram || e.Caster != e.Dst || e.Src != alia || e.Dst != bram {
				t.Fatal("self-shield caster was dropped or attribution changed")
			}
		})
	}
}

func TestAbsorbAbsentCasterNotInferred(t *testing.T) {
	for _, tc := range []struct {
		name      string
		withSpell bool
		casterAt  int
	}{{"swing", false, 8}, {"spell", true, 11}} {
		t.Run(tc.name, func(t *testing.T) {
			// absent() recognizes nil and the zero GUID; an empty field also
			// stays empty. None of them is permission to infer Dst as caster.
			for _, absent := range []struct {
				name string
				wire fixture.Field
			}{{"empty", ""}, {"zero-guid", "0000000000000000"}, {"nil", fixture.Nil}} {
				t.Run(absent.name, func(t *testing.T) {
					f := absorbFields(tc.withSpell)
					copy(f[tc.casterAt:tc.casterAt+4], fixture.Actor(model.Actor{}))
					f[tc.casterAt] = absent.wire
					events, r := decode(line(t, "SPELL_ABSORBED", f...))
					balanced(t, r)
					if len(events) != 1 || r.Parsed != 1 || len(r.Skipped) != 0 {
						t.Fatal("absorb with absent caster was not decoded")
					}
					e := events[0]
					if e.Kind != model.Absorb || e.Caster != (model.Actor{}) || e.Src != alia || e.Dst != bram {
						t.Fatal("absent caster was inferred or attribution changed")
					}
				})
			}
		})
	}
}
