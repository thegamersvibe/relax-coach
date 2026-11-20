package death_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// A later line cannot retroactively call a wipe for an earlier death.
func TestFutureWipeCallLine(t *testing.T) {
	c := config()
	c.Scoring.Death.WipeAllTanksDead = true
	mechanic(&c, 81)
	f := fight(hit(9999, boss, victim, 81, 1), died(10000, victim), died(9998, tank))
	wantDeath(t, forVictim(t, f, c), model.CauseAvoidable, "high", model.OwnerPlayer, "", victim.Name)
}
