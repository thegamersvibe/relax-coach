package death_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"testing"
)

func TestClockRollbackDeclinesEveryRule(t *testing.T) {
	for _, mode := range []string{"avoidable", "unhealed", "suggestion", "wipe", "below_lower_bound"} {
		t.Run(mode, func(t *testing.T) {
			c := config()
			f := slowDeath(6000)
			if mode == "avoidable" {
				mechanic(&c, 82)
			}
			if mode == "suggestion" {
				c.Scoring.Death.MinHealthSamples = 3
			}
			if mode == "wipe" {
				c.Scoring.Death.WipeAllTanksDead = true
				f.Events = append([]model.Event{died(4000, tank)}, f.Events...)
			}
			at := int64(8999)
			if mode == "below_lower_bound" {
				at = -1
			}
			// Unrelated lines still invalidate chronology inside the positional span.
			es := append([]model.Event(nil), f.Events[:len(f.Events)-2]...)
			es = append(es, model.Event{TimeMs: 9000}, model.Event{TimeMs: at})
			f.Events = append(es, f.Events[len(f.Events)-2:]...)
			wantDeath(t, forVictim(t, f, c), death.Unresolved, "low", model.OwnerUndecided, "")
		})
	}
}

func TestClockRollbackScope(t *testing.T) {
	c := config()
	mechanic(&c, 81)
	for _, mode := range []string{"before_window", "after_death", "previous_life", "equal_time"} {
		t.Run(mode, func(t *testing.T) {
			f := quickDeath()
			c.Scoring.Death.WindowMs = 2000
			switch mode {
			case "before_window":
				f.Events = append([]model.Event{{TimeMs: 7000}, {TimeMs: 6999}}, f.Events...)
			case "after_death":
				f.Events = append(f.Events, hit(9999, boss, victim, 82, 999))
			case "previous_life":
				f.Events = append([]model.Event{{TimeMs: 8000}, {TimeMs: 7999}, died(8000, victim)}, f.Events...)
			case "equal_time":
				f.Events = append([]model.Event{{TimeMs: 8000}}, f.Events...)
			}
			ds, _, _ := review(t, f, 100000, c)
			wantDeath(t, ds[len(ds)-1], death.Avoidable, "high", model.OwnerPlayer, "", victim.Name)
		})
	}
}
