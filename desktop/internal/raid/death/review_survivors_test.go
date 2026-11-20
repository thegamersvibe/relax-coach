package death_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"testing"
)

func TestLostBossOnlyHolderDamageRepairsGap(t *testing.T) {
	f := reviewFight(hit(1000, tank, boss, 91, 1), hit(2000, victim, boss, 92, 1), swing(8000, boss, tank, 1), hit(8999, tankB, boss, 91, 1), swing(9000, boss, victim, 1), died(10000, victim))
	wantDeath(t, forVictim(t, f, config()), death.TankLostBoss, "high", model.OwnerPlayer, "", tank.Name)
	// The same recent damage by the actual holder repairs the gap.
	f.Events[3].Src = tank
	wantDeath(t, forVictim(t, f, config()), death.Unresolved, "low", model.OwnerUndecided, "")
}

func TestLastHolderLineOutsideWindowIsNotSkipped(t *testing.T) {
	f := reviewFight(swing(10001, boss, tank, 1), swing(9999, boss, other, 1), swing(20000, boss, victim, 1), died(20001, victim))
	// The rollback is before the death window (which begins at 10002), but the
	// last holder line is too old and cannot be replaced by an earlier tank.
	c := config()
	c.Scoring.Death.WindowMs = 9999
	wantDeath(t, forVictim(t, f, c), death.Unresolved, "low", model.OwnerUndecided, "")
}

func TestPulledAggroDoesNotAlsoMatchLostBoss(t *testing.T) {
	f := reviewFight(hit(1000, victim, boss, 92, 1), hit(1001, tank, boss, 91, 1), swing(8000, boss, tank, 1), swing(9000, boss, victim, 1), died(10000, victim))
	// Without the rule-3 exclusion, lostBoss also matches and downgrades confidence.
	wantDeath(t, forVictim(t, f, config()), death.PulledAggro, "high", model.OwnerPlayer, "", victim.Name)
}
