package death_test

import (
	"fmt"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestLostBossObservedHolder(t *testing.T) {
	for _, lastTarget := range []model.Actor{tankB, other, {}} {
		t.Run(lastTarget.Name, func(t *testing.T) {
			f := reviewFight(hit(5500, tank, boss, 91, 1), hit(5000, tankB, boss, 91, 1), swing(10000, boss, victim, 1), died(10001, victim))
			if lastTarget.ID != "" {
				f.Events = append(f.Events, swing(8000, boss, tank, 1), swing(9000, boss, lastTarget, 1))
			}
			if lastTarget.ID == tankB.ID {
				wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", "player", "", tankB.Name)
			} else {
				wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
			}
		})
	}
}

func TestLostBossHolderWindow(t *testing.T) {
	for _, age := range []int64{9999, 10000, 10001} {
		t.Run(fmt.Sprint(age), func(t *testing.T) {
			f := reviewFight(hit(1000, tank, boss, 91, 1), swing(20000-age, boss, tank, 1), swing(20000, boss, victim, 1), died(20001, victim))
			if age <= 10000 {
				wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", "player", "", tank.Name)
			} else {
				wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
			}
		})
	}
	f := reviewFight(hit(1000, tank, boss, 91, 1), swing(15000, boss, tank, 1), swing(20000, boss, victim, 1), died(20001, victim))
	c := config()
	c.Scoring.Death.WindowMs = 4999
	wantDeath(t, forVictim(t, f, c), death.Unresolved, "low", "undecided", "")
	c.Scoring.Death.WindowMs = 5000
	wantDeath(t, forVictim(t, f, c), death.TankLostBoss, "high", "player", "", tank.Name)
}

func TestLostBossHolderAliveAtTurn(t *testing.T) {
	for _, tc := range []struct {
		name string
		es   []model.Event
		lost bool
	}{
		{"dead_at_turn", []model.Event{died(8000, tank)}, false},
		{"res_after_turn", []model.Event{died(8000, tank), res(9500, tank)}, false},
		{"dies_after_turn", []model.Event{died(9500, tank)}, true},
		{"res_before_turn", []model.Event{died(7000, tank), res(8000, tank)}, true},
		{"dead_same_ms_before_turn", []model.Event{died(9000, tank)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := reviewFight(hit(1000, tank, boss, 91, 1), swing(6000, boss, tank, 1))
			f.Events = append(f.Events, tc.es...)
			f.Events = append(f.Events, swing(9000, boss, victim, 1), swing(9900, boss, victim, 1), died(10000, victim))
			if tc.lost {
				wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", "player", "", tank.Name)
			} else {
				wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
			}
		})
	}
}

func TestLostBossSameTimestampOrder(t *testing.T) {
	for _, holderFirst := range []bool{false, true} {
		f := reviewFight(hit(1000, tank, boss, 91, 1))
		holder, turn := swing(9000, boss, tank, 1), swing(9000, boss, victim, 1)
		if holderFirst {
			f.Events = append(f.Events, holder, turn)
		} else {
			f.Events = append(f.Events, turn, holder)
		}
		f.Events = append(f.Events, died(10000, victim))
		if holderFirst {
			wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", "player", "", tank.Name)
		} else {
			wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
		}
	}
	// A holder's own damage at the turn millisecond counts even if the line
	// follows the turn. Unlike the prior target, this is a time-gap predicate.
	for _, hitFirst := range []bool{false, true} {
		f := reviewFight(hit(1000, tank, boss, 91, 1), swing(8000, boss, tank, 1))
		h, turn := hit(9000, tank, boss, 91, 1), swing(9000, boss, victim, 1)
		if hitFirst {
			f.Events = append(f.Events, h, turn)
		} else {
			f.Events = append(f.Events, turn, h)
		}
		f.Events = append(f.Events, died(10000, victim))
		wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
	}
}

func TestLostBossMissedSwingsAndNoHit(t *testing.T) {
	for _, how := range []string{"MISS", "PARRY", "DODGE", "ABSORB"} {
		t.Run(how, func(t *testing.T) {
			f := reviewFight(missedSwing(7000, boss, tank, how), missedSwing(8000, boss, victim, how), swing(9900, boss, victim, 1), died(10000, victim))
			wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", "player", "", tank.Name)
			// A damage gap is measured from the first missed victim swing, not
			// the killing hit; 8000-4000 is exactly the non-blaming edge.
			f.Events = append(f.Events, hit(4000, tank, boss, 91, 1))
			wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
		})
	}
}

func TestLostBossTiedAttacksDoNotChooseHolder(t *testing.T) {
	f := reviewFight(hit(1000, tank, boss, 91, 1), hit(1000, tankB, boss, 92, 1), swing(9900, boss, victim, 1), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", "undecided", "")
	// Equal tank damage does not make the boss's observed target ambiguous.
	f.Events = append(f.Events, swing(8000, boss, tankB, 1))
	wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", "player", "", tankB.Name)
}
