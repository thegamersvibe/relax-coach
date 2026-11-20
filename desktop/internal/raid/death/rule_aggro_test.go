package death_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func meleeDeath() model.Fight {
	return fight(hit(1000, tank, boss, 91, 1), hit(2000, victim, boss, 92, 1), sample(8000, 800, 1000), swing(9000, boss, victim, 100), sample(9500, 500, 1000), swing(9900, boss, victim, 100), died(10000, victim))
}
func TestPulledAggroBeforeTank(t *testing.T) {
	f := meleeDeath()
	f.Events[0].TimeMs = 2001
	wantDeath(t, forVictim(t, f, config()), "pulled aggro", "high", "player", "", victim.Name)
	f.Events[0].TimeMs = 2000
	c := config()
	c.Scoring.Death.LostBossGapMs = ptr(9000)
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
	f = meleeDeath()
	f.Events[0].TimeMs = 9500
	wantDeath(t, forVictim(t, f, config()), "pulled aggro", "high", "player", "", victim.Name)
	pet := modeltest.Pet("Synthetic pet", victim)
	f.Events[1].Src = pet
	f.Owners = map[string]string{pet.ID: victim.ID}
	wantDeath(t, forVictim(t, f, config()), "pulled aggro", "high", "player", "", victim.Name)
	delete(f.Owners, pet.ID)
	wantDeath(t, forVictim(t, f, config()), "unresolved", "low", "undecided", "")
}
func TestPulledAggroNoLivingTank(t *testing.T) {
	f := meleeDeath()
	f.Events = append(f.Events, swing(7000, boss, tank, 1), died(8000, tank))
	// The bounded-rule ruling removed the no-living-tank puller shortcut.
	wantDeath(t, forVictim(t, f, config()), "unresolved", "low", "undecided", "")
	f.Events[len(f.Events)-1] = modeltest.At(8000).Dst(tank).Died(1).Build()
	wantDeath(t, forVictim(t, f, config()), "tank lost the boss", "high", "player", "", tank.Name)
}
func TestLostBossGapAndOwner(t *testing.T) {
	for _, tc := range []struct {
		at    int64
		cause string
	}{{4999, "tank lost the boss"}, {5000, "unresolved"}, {5001, "unresolved"}} {
		f := meleeDeath()
		f.Events[0].TimeMs = tc.at
		f.Events[1].TimeMs = 6000
		f.Events = append(f.Events, swing(8000, boss, tank, 1)) // observed holder
		d := forVictim(t, f, config())
		if tc.cause == "unresolved" {
			wantDeath(t, d, tc.cause, "low", "undecided", "")
		} else {
			wantDeath(t, d, tc.cause, "high", "player", "", tank.Name)
		}
	}
	f := meleeDeath()
	f.Events = append(f.Events, swing(8000, boss, tank, 1), hit(9500, tank, boss, 91, 1)) // a hit after the turn cannot repair the earlier gap
	wantDeath(t, forVictim(t, f, config()), "tank lost the boss", "high", "player", "", tank.Name)
	c := config()
	c.Scoring.Death.LostBossGapMs = nil
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
	pet := modeltest.Pet("Synthetic tank pet", tank)
	f = meleeDeath()
	f.Events[0].Src = pet
	f.Owners = map[string]string{pet.ID: tank.ID}
	f.Events = append(f.Events, swing(8000, boss, tank, 1))
	wantDeath(t, forVictim(t, f, config()), "tank lost the boss", "high", "player", "", tank.Name)
}
func TestAmbiguousAggroHasNoSuggestionAndBossSwingEvidence(t *testing.T) {
	tankVictim := meleeDeath()
	tankVictim.Events = append(tankVictim.Events, swing(8000, boss, tank, 1)) // otherwise proves lost-boss
	tankVictim.Roles[victim.ID] = "tank"
	wantDeath(t, forVictim(t, tankVictim, config()), "unresolved", "low", "undecided", "")
	f := meleeDeath()
	f.Events[0].TimeMs = 7000
	f.Events[1].TimeMs = 7100
	f.Events = append(f.Events, modeltest.At(7500).Src(healer).Dst(victim).Heal(1, 0).Build())
	d := forVictim(t, f, config())
	wantDeath(t, d, "unresolved", "low", "undecided", "")
	if !reflect.DeepEqual(d.Evidence, []int64{9000, 9900}) {
		t.Fatalf("aggro evidence=%v", d.Evidence)
	}
	f.Roles[victim.ID] = "tank"
	wantDeath(t, forVictim(t, f, config()), "unresolved", "low", "undecided", "")
	f = meleeDeath()
	f.BossUnits = nil
	wantDeath(t, forVictim(t, f, config()), "unresolved", "low", "undecided", "")
	f = meleeDeath()
	f.Events[5].Stream = model.SwingSource
	f.Events[3].Stream = model.SwingSource
	wantDeath(t, forVictim(t, f, config()), "unresolved", "low", "undecided", "")
	f = meleeDeath()
	f.Events[5] = hit(9900, boss, victim, 81, 100)
	wantDeath(t, forVictim(t, f, config()), "unresolved", "low", "undecided", "")
}
