package death_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func TestPreviousDeathLineExcluded(t *testing.T) {
	previous := died(2000, victim)
	previous.InfoUnit, previous.HP, previous.HPMax = victim.ID, model.Some(200), model.Some(1000)
	f := fight(previous, hit(3000, boss, victim, 81, 1), sample(6000, 200, 1000), hit(9900, boss, victim, 82, 1), died(10000, victim))
	ds, _ := death.Review(f, config())
	if len(ds) != 2 {
		t.Fatalf("deaths=%d", len(ds))
	}
	wantDeath(t, ds[1], death.Unresolved, "low", model.OwnerUndecided, death.Unhealed)
}

func TestKillingRunLifeWindow(t *testing.T) {
	f := reviewFight(hit(1000, victim, boss, 91, 1), swing(2000, boss, victim, 1), died(2001, victim), hit(3000, tank, boss, 91, 1), res(6000, victim), swing(11999, boss, victim, 1), died(12000, victim))
	ds, _ := death.Review(f, config())
	if len(ds) != 2 {
		t.Fatalf("deaths=%d", len(ds))
	}
	wantDeath(t, ds[0], death.PulledAggro, "high", model.OwnerPlayer, "", victim.Name)
	wantDeath(t, ds[1], death.Unresolved, "low", model.OwnerUndecided, "")
	// Without another life boundary, the ordinary time bound still excludes
	// the earlier swing when determining this death's killing run.
	f = reviewFight(hit(0, victim, boss, 91, 1), swing(1, boss, victim, 1), hit(2, tank, boss, 91, 1), swing(19999, boss, victim, 1), died(20000, victim))
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
}

func TestOneShotDoesNotSumEarlierLife(t *testing.T) {
	f := fight(sample(8900, 1000, 1000), hit(9000, boss, victim, 81, 1000), died(9001, victim), res(9500, victim), sample(9600, 1000, 1000), hit(9999, boss, victim, 82, 1), died(10000, victim))
	ds, _ := death.Review(f, config())
	if len(ds) != 2 {
		t.Fatalf("deaths=%d", len(ds))
	}
	wantDeath(t, ds[0], death.Unavoidable, "high", model.OwnerNobody, "")
	wantDeath(t, ds[1], death.Unresolved, "low", model.OwnerUndecided, "")
}

func TestAggroActionsAfterDeathExcluded(t *testing.T) {
	f := reviewFight(swing(10000, boss, victim, 1), died(10000, victim), hit(10000, victim, boss, 91, 1))
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
	f = reviewFight(hit(1000, tank, boss, 91, 1), swing(8000, boss, tank, 1), swing(10000, boss, victim, 1), died(10000, victim), hit(10000, tank, boss, 91, 1))
	wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", model.OwnerPlayer, "", tank.Name)
}

func TestDeadTankOpenerStillCounts(t *testing.T) {
	f := reviewFight(hit(1000, tank, boss, 91, 1), hit(3000, victim, boss, 92, 1), hit(4000, tankB, boss, 91, 1), died(5000, tank), swing(8000, boss, tankB, 1), swing(9000, boss, victim, 1), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", model.OwnerPlayer, "", tankB.Name)
}

func TestAggroSpecificBossAndDamage(t *testing.T) {
	second := hostile("Synthetic other boss", 72)
	for _, how := range []string{"MISS", "PARRY", "DODGE"} {
		// The architect's tank-hit definition is a damage event, not an
		// unsuccessful tank attack. Boss missed swings still establish targets.
		f := reviewFight(hit(1000, tank, boss, 91, 1), swing(8000, boss, tank, 1), missedSwing(8999, tank, boss, how), swing(9000, boss, victim, 1), died(10000, victim))
		wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", model.OwnerPlayer, "", tank.Name)
	}
	for _, e := range []model.Event{
		swing(8999, second, tankB, 1),
		hit(8999, boss, tankB, 91, 1),
		{TimeMs: 8999, Kind: model.Missed, Src: boss, Dst: tankB, SpellID: 91},
		{TimeMs: 8999, Kind: model.Missed, Src: boss, Dst: tankB, SpellName: "Synthetic spell"},
	} {
		f := reviewFight(hit(1000, tank, boss, 91, 1), swing(8000, boss, tank, 1), e, hit(8999, tank, second, 91, 1), swing(9000, boss, victim, 1), died(10000, victim))
		f.BossUnits = append(f.BossUnits, second.ID)
		wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", model.OwnerPlayer, "", tank.Name)
	}
	// A zero-damage event is still a hit; no inferred threat or effective-
	// damage threshold is added to the architect's event-kind definition.
	f := reviewFight(hit(1000, victim, boss, 91, 0), hit(1001, tank, boss, 91, 1), swing(9000, boss, victim, 1), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), death.PulledAggro, "high", model.OwnerPlayer, "", victim.Name)
	f = reviewFight(hit(1000, tank, boss, 91, 1), swing(8000, boss, tank, 1), hit(8999, tank, boss, 91, 0), swing(9000, boss, victim, 1), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
}
