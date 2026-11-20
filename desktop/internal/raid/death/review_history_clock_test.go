package death_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Adapted from the round-3 reviewer probes. Never sort these raw log lines.
func TestPreWindowHistoryRollback(t *testing.T) {
	loaded, err := cfg.Load(filepath.Join("..", "..", "..", "..", "rules", "raid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []cfg.Config{config(), loaded.Snapshot()} {
		for _, step := range []int64{1, 2, 1000, 5000} {
			t.Run(fmt.Sprintf("tank_first_%s_%d", c.Scoring.Version, step), func(t *testing.T) {
				f := fight(hit(9999, tank, boss, 91, 1), hit(9999-step, tank, boss, 91, 1), hit(9999-step, victim, boss, 92, 1), swing(10000, boss, victim, 1), died(20000, victim))
				wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
			})
		}
		for _, step := range []int64{1, 2, 1000, 4000} {
			t.Run(fmt.Sprintf("engagement_%s_%d", c.Scoring.Version, step), func(t *testing.T) {
				f := fight(hit(6000, tank, boss, 91, 1), hit(6000-step, tank, boss, 91, 1), swing(9999, boss, tank, 1), swing(10000, boss, victim, 1), died(20000, victim))
				wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
			})
		}
		t.Run("opener_outside_window_"+c.Scoring.Version, func(t *testing.T) {
			f := fight(hit(1000, tank, boss, 91, 1), hit(999, victim, boss, 91, 1), swing(5000, boss, tank, 1), hit(9000, tank, boss, 91, 1), hit(10500, tank, boss, 91, 1), swing(10900, boss, victim, 1), died(11002, victim))
			wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
			f.Events[1].TimeMs = 1001
			wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
		})
		t.Run("earlier_life_"+c.Scoring.Version, func(t *testing.T) {
			f := fight(hit(1000, tank, boss, 91, 1), hit(999, victim, boss, 91, 1), died(2000, victim), model.Event{TimeMs: 2500, Kind: model.Resurrect, Dst: victim}, swing(3000, boss, tank, 1), hit(4000, tank, boss, 91, 1), swing(5000, boss, victim, 1), died(5100, victim))
			ds, _, _ := review(t, f, 100000, c)
			wantDeath(t, ds[len(ds)-1], model.CauseUnresolved, "low", model.OwnerUndecided, "")
		})
		t.Run("positive_controls_"+c.Scoring.Version, func(t *testing.T) {
			f := fight(hit(9998, victim, boss, 92, 1), hit(9999, tank, boss, 91, 1), swing(10000, boss, victim, 1), died(20000, victim))
			wantDeath(t, forVictim(t, f, c), model.CausePulledAggro, "high", model.OwnerPlayer, "", victim.Name)
			f = fight(hit(5999, tank, boss, 91, 1), swing(9999, boss, tank, 1), swing(10000, boss, victim, 1), died(20000, victim))
			wantDeath(t, forVictim(t, f, c), model.CauseLostBoss, "high", model.OwnerPlayer, "", tank.Name)
		})
	}
}

func TestHistoryRollbackDeclinesSurvivalRules(t *testing.T) {
	for _, mode := range []string{"unhealed", "wipe", "raidwide"} {
		t.Run(mode, func(t *testing.T) {
			c := config()
			f := fight(sample(10000, 1000, 1000), hit(14000, boss, victim, 81, 1), sample(16000, 500, 1000), hit(19999, boss, victim, 81, 1), died(20000, victim))
			want := model.CauseUnhealed
			switch mode {
			case "wipe":
				c.Scoring.Death.WipeAllTanksDead = true
				f.Events = append([]model.Event{died(9000, tank)}, f.Events...)
				want = model.CauseAfterWipe
			case "raidwide":
				f = fight(hit(19999, boss, victim, 81, 1), hit(19999, boss, tank, 81, 1), hit(19999, boss, other, 81, 1), died(20000, victim))
				want = model.CauseUnavoidable
			}
			if d := forVictim(t, f, c); d.Cause != want {
				t.Fatalf("inactive control: got %s want %s", d.Cause, want)
			}
			f.Events = append([]model.Event{{TimeMs: 8000}, {TimeMs: 7999}}, f.Events...)
			wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
		})
	}
}

func TestRaidwideFutureClockStep(t *testing.T) {
	f := fight(hit(19999, boss, victim, 81, 1), died(20000, victim), hit(20002, boss, tank, 81, 1), hit(20001, boss, other, 81, 1))
	wantDeath(t, forVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f.Events[3].TimeMs = 20002
	wantDeath(t, forVictim(t, f, config()), model.CauseUnavoidable, "high", model.OwnerNobody, "")
}

func TestClockGuardFirstAdjacentPair(t *testing.T) {
	c := config()
	mechanic(&c, 81)
	f := fight(model.Event{TimeMs: 10001}, model.Event{TimeMs: 10000}, hit(19999, boss, victim, 81, 1), died(20000, victim))
	wantDeath(t, forVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f.Events[0].TimeMs = 10000
	wantDeath(t, forVictim(t, f, c), model.CauseAvoidable, "high", model.OwnerPlayer, "", victim.Name)
}
