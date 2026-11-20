package death_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestNilSourceIsNotEnvironmental(t *testing.T) {
	for _, mode := range []string{"spell", "blank", "unknown_type", "swing", "spell_named_FIRE", "one_shot"} {
		t.Run(mode, func(t *testing.T) {
			f := slowDeath(6000)
			e := &f.Events[3]
			e.Src = model.Actor{}
			switch mode {
			case "blank":
				e.SpellID = 0
				e.SpellName = ""
			case "unknown_type":
				e.SpellID = 0
				e.SpellName = "NOT_AN_ENVIRONMENTAL_TYPE"
			case "swing":
				e.SpellID = 0
				e.SpellName = "FIRE"
				e.Stream = model.SwingLanded
			case "spell_named_FIRE":
				e.SpellName = "FIRE"
			case "one_shot":
				e.Amount = model.Some(1000)
			}
			wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
		})
	}
}

func TestEnvironmentalParserProvenance(t *testing.T) {
	for _, kind := range []string{"FALLING", "FIRE", "LAVA", "SLIME", "DROWNING", "FATIGUE"} {
		t.Run(kind, func(t *testing.T) {
			line := fmt.Sprintf("1/1/2026 00:00:09.9000  ENVIRONMENTAL_DAMAGE,nil,nil,0x0,0x0,%s,%s,0x0,0x0,%s,100,100,-1,1,0,0,0,0,0,0\n", victim.ID, victim.Name, kind)
			var parsed []model.Event
			var report logfmt.ParseReport
			for e := range logfmt.Decode(strings.NewReader(line), &report) {
				parsed = append(parsed, e)
			}
			if len(parsed) != 1 {
				t.Fatalf("decoded %d events: %+v", len(parsed), report)
			}
			e := parsed[0]
			if e.Kind != model.Damage || e.Stream != model.StreamNone || e.SpellID != 0 || e.Src.ID != "" || e.SpellName != kind {
				t.Fatalf("unexpected environmental encoding: %+v", e)
			}
			e.TimeMs = 9900
			f := slowDeath(6000)
			f.Events[3] = e
			wantDeath(t, forVictim(t, f, config()), death.Unavoidable, "high", "nobody", "")
			c := config()
			mechanic(&c, 0)
			wantDeath(t, forVictim(t, f, c), death.Unresolved, "low", "undecided", "")
		})
	}
}

func TestUnownedGuardianExclusion(t *testing.T) {
	for _, candidate := range []bool{false, true} {
		f := raidwide(5, 3)
		guardian := hostile("Synthetic unowned guardian", 77)
		f.Unowned = []string{guardian.ID}
		i := len(f.Events) - 1
		if candidate {
			i = 2
		}
		f.Events[i].Src = guardian
		wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
	}
}
