package death_test

import (
	"fmt"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

var tankB = modeltest.Player("Faye-Birch-PvP-EU")

func reviewFight(es ...model.Event) model.Fight {
	f := fight(es...)
	f.Players = append(f.Players, tankB)
	f.Roles[tankB.ID] = "tank"
	return f
}
func reviewMelee(tankAt, victimAt int64) model.Fight {
	return reviewFight(hit(tankAt, tank, boss, 91, 1), hit(victimAt, victim, boss, 92, 1), sample(8000, 700, 1000), swing(9900, boss, victim, 20), died(10000, victim))
}
func res(at int64, who model.Actor) model.Event {
	return model.Event{TimeMs: at, Kind: model.Resurrect, Dst: who}
}
func missedSwing(at int64, src, dst model.Actor, how string) model.Event {
	// logfmt's SWING_MISSED has no spell triple and StreamNone.
	return modeltest.At(at).Src(src).Dst(dst).Miss(how).Build()
}

func TestZZFalseBlame(t *testing.T) {
	t.Run("passive_victim_no_tanks_alive", func(t *testing.T) {
		f := reviewFight(died(8000, tank), died(8000, tankB), swing(9999, boss, victim, 1), died(10000, victim))
		wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
	})
	t.Run("unknown_roles_are_not_known_nontank", func(t *testing.T) {
		f := reviewMelee(9000, 9001)
		f.Roles = nil
		wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
	})
	t.Run("dead_recent_tank_must_not_charge_stale_tank", func(t *testing.T) {
		f := reviewMelee(1000, 3000)
		f.Events = append(f.Events, hit(9800, tankB, boss, 91, 1), swing(9801, boss, tankB, 1), died(9802, tankB))
		wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
	})
	t.Run("tank_dead_at_turn_resurrected_by_kill", func(t *testing.T) {
		f := reviewMelee(1000, 3000)
		f.Events = append(f.Events, died(6000, tank), swing(8000, boss, victim, 1), res(9000, tank))
		wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
	})
	t.Run("same_ms_tank_hit_before_boss_turn", func(t *testing.T) {
		f := reviewFight(hit(1000, tank, boss, 91, 1), hit(2000, victim, boss, 92, 1), hit(9000, tank, boss, 91, 1), swing(9000, boss, victim, 1), died(10000, victim))
		wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
	})
	t.Run("self_source_majority_then_environmental_kill", func(t *testing.T) {
		f := slowDeath(6000)
		f.Events[1].Amount = model.Some(500)
		f.Events[1].Src = victim
		f.Events[3].Src = model.Actor{}
		f.Events[3].SpellID = 0
		f.Events[3].SpellName = "FALLING"
		c := config()
		mechanic(&c, 81)
		wantDeath(t, forVictim(t, f, c), death.Unavoidable, "high", model.OwnerNobody, "")
	})
}

func TestPulledAggroBound(t *testing.T) {
	for _, elapsed := range []int64{9000, 10000, 10001, 60000} {
		t.Run(fmt.Sprint(elapsed), func(t *testing.T) {
			f := fight(hit(1000, victim, boss, 91, 1), hit(1001, tank, boss, 92, 1), swing(1000+elapsed, boss, victim, 1), died(1000+elapsed, victim))
			c := config()
			c.Scoring.Death.PulledAggroMs = 10000
			if elapsed <= 10000 {
				wantDeath(t, forVictim(t, f, c), death.PulledAggro, "high", model.OwnerPlayer, "", victim.Name)
			} else {
				wantDeath(t, forVictim(t, f, c), death.Unresolved, "low", model.OwnerUndecided, "")
			}
		})
	}
	f := fight(hit(1000, victim, boss, 91, 1), hit(1001, tank, boss, 92, 1), swing(10000, boss, victim, 1), died(10000, victim))
	c := config()
	c.Scoring.Death.PulledAggroMs = 8999
	wantDeath(t, forVictim(t, f, c), death.Unresolved, "low", model.OwnerUndecided, "")
	c.Scoring.Death.PulledAggroMs = 9000
	wantDeath(t, forVictim(t, f, c), death.PulledAggro, "high", model.OwnerPlayer, "", victim.Name)
}

func TestAggroUnknownVictimRole(t *testing.T) {
	for _, opening := range []bool{true, false} {
		for _, role := range []string{"", "unknown", "missing"} {
			t.Run(fmt.Sprintf("opening_%v_role_%s", opening, role), func(t *testing.T) {
				f := reviewMelee(1000, 3000)
				f.Events = append(f.Events, swing(7000, boss, tank, 1))
				if opening {
					f.Events[0].TimeMs = 3001
				}
				if role == "missing" {
					delete(f.Roles, victim.ID)
				} else {
					f.Roles[victim.ID] = role
				}
				wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
			})
		}
	}
}

func TestZZAggroAtAttackTime(t *testing.T) {
	f := reviewFight(hit(1000, tank, boss, 91, 1), died(8000, tank), died(8000, tankB), hit(9000, victim, boss, 92, 1), res(9500, tank), hit(9600, tank, boss, 91, 1), swing(9900, boss, victim, 1), died(10000, victim))
	// The architect removed the no-living-tank alternative. This reproduction
	// now documents unresolved, not the reviewer's superseded puller charge.
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
}

func TestPulledAggroKillingRun(t *testing.T) {
	for _, how := range []string{"MISS", "PARRY", "DODGE"} {
		f := fight(hit(1000, victim, boss, 91, 1), hit(1001, tank, boss, 92, 1), missedSwing(9000, boss, victim, how), swing(11999, boss, victim, 1), died(12000, victim))
		wantDeath(t, forVictim(t, f, config()), death.PulledAggro, "high", model.OwnerPlayer, "", victim.Name)
	}
	f := fight(swing(9000, boss, victim, 1), hit(9001, victim, boss, 91, 1), hit(9002, tank, boss, 92, 1), swing(9900, boss, victim, 1), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
}

func TestPulledAggroDamageOnlyAndFightReach(t *testing.T) {
	for _, how := range []string{"MISS", "DODGE", "PARRY", "ABSORB"} {
		t.Run(how, func(t *testing.T) {
			f := fight(missedSwing(1000, victim, boss, how), hit(2000, tank, boss, 91, 1), swing(9000, boss, victim, 1), died(10000, victim))
			wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
		})
	}
	f := fight(hit(-1, victim, boss, 91, 1), hit(1000, tank, boss, 92, 1), swing(9000, boss, victim, 1), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
	// A new life does not reset rule 3's explicitly fight-wide first damage.
	f = fight(hit(1000, victim, boss, 91, 1), died(2000, victim), res(59000, victim), hit(59500, victim, boss, 91, 1), hit(59501, tank, boss, 92, 1), swing(60000, boss, victim, 1), died(60001, victim))
	ds, _ := death.Review(f, config())
	wantDeath(t, ds[len(ds)-1], death.Unresolved, "low", model.OwnerUndecided, "")
}
