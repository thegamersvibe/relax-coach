package death_test

import (
	"fmt"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func raidwide(players, hits int) model.Fight {
	f := quickDeath()
	f.Players = []model.Actor{victim}
	f.Roles = map[string]string{victim.ID: "dps"}
	for i := 1; i < players; i++ {
		p := modeltest.Player(fmt.Sprintf("Synthetic%d-PvP-EU", i))
		f.Players = append(f.Players, p)
		if i < hits {
			f.Events = append(f.Events, hit(9900, boss, p, 81, 1))
		}
	}
	return f
}
func TestRaidwideShareAndMinimum(t *testing.T) {
	for _, tc := range []struct {
		players, hits int
		match         bool
	}{{100, 59, false}, {100, 60, true}, {2, 2, false}, {5, 2, false}, {5, 3, true}, {5, 4, true}} {
		d := forChronologicalVictim(t, raidwide(tc.players, tc.hits), config())
		if tc.match {
			wantDeath(t, d, model.CauseUnavoidable, "high", model.OwnerNobody, "")
		} else {
			wantDeath(t, d, model.CauseUnresolved, "low", model.OwnerUndecided, "")
		}
	}
	c := config()
	c.Scoring.Death.RaidwideShare = .61
	wantDeath(t, forChronologicalVictim(t, raidwide(5, 3), c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestRaidwideWindowEdgesAndSources(t *testing.T) {
	for _, at := range []int64{7899, 7900, 11900, 11901} {
		f := raidwide(5, 3)
		f.Events[len(f.Events)-1].TimeMs = at
		d := forChronologicalVictim(t, f, config())
		if at == 7900 || at == 11900 {
			wantDeath(t, d, model.CauseUnavoidable, "high", model.OwnerNobody, "")
		} else {
			wantDeath(t, d, model.CauseUnresolved, "low", model.OwnerUndecided, "")
		}
	}
	f := raidwide(5, 3)
	f.Events[len(f.Events)-1].Src = hostile("Synthetic add", 72)
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnavoidable, "high", model.OwnerNobody, "")
	f.Events[len(f.Events)-1].Src = victim
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f = raidwide(5, 3)
	f.Events = append(f.Events, f.Events[4], f.Events[4])
	f.Events[5].Dst = f.Events[4].Dst
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	c := config()
	c.Scoring.Death.RaidwideWindowMs = 1999
	f = raidwide(5, 3)
	f.Events[5].TimeMs = 7900
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestRaidwideExclusions(t *testing.T) {
	candidateSwing := raidwide(5, 4)
	candidateSwing.Roles[victim.ID] = "tank"
	candidateSwing.Events[2].Stream = model.SwingLanded
	wantDeath(t, forChronologicalVictim(t, candidateSwing, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	countedSwing := raidwide(5, 3)
	countedSwing.Events[5].Stream = model.SwingLanded
	wantDeath(t, forChronologicalVictim(t, countedSwing, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	for _, tc := range []struct {
		name string
		edit func(*model.Fight, *cfg.Config)
	}{
		{"swing", func(f *model.Fight, c *cfg.Config) {
			f.Roles[victim.ID] = "tank" // isolate raid-wide exclusion from the aggro rules
			for i := range f.Events {
				if f.Events[i].Kind == model.Damage {
					f.Events[i].Stream = model.SwingLanded
				}
			}
		}},
		{"friendly source", func(f *model.Fight, c *cfg.Config) { f.Events[2].Src.Flags = 0x10 }},
		{"player source", func(f *model.Fight, c *cfg.Config) { f.Events[2].Src = victim }},
		{"avoidable even other role", func(f *model.Fight, c *cfg.Config) { mechanic(c, 81, "tank") }},
		{"pet target", func(f *model.Fight, c *cfg.Config) {
			pet := modeltest.Pet("Synthetic", victim)
			f.Players = append(f.Players, pet)
			f.Owners = map[string]string{pet.ID: victim.ID}
			f.Events[5].Dst = pet
		}},
		{"guardian source", func(f *model.Fight, c *cfg.Config) {
			pet := hostile("Synthetic guardian", 73)
			f.Owners = map[string]string{pet.ID: victim.ID}
			f.Events[5].Src = pet
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := raidwide(5, 3)
			c := config()
			tc.edit(&f, &c)
			wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
		})
	}
	f := raidwide(5, 3)
	pet := modeltest.Pet("Synthetic", victim)
	f.Players = append(f.Players, pet)
	f.Owners = map[string]string{pet.ID: victim.ID}
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnavoidable, "high", model.OwnerNobody, "")
}
func TestRaidwideAliveDenominatorAndVictim(t *testing.T) {
	f := raidwide(6, 3)
	f.Events = append(f.Events, died(7899, f.Players[5]))
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnavoidable, "high", model.OwnerNobody, "")
	f.Events[len(f.Events)-1].TimeMs = 7901
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f = raidwide(6, 3) // dividing by the five survivors at death would incorrectly pass
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f = raidwide(6, 3)
	f.Events = append(f.Events, modeltest.At(7000).Dst(f.Players[5]).Died(1).Build())
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestRaidwideMajorityAndMediumConfidence(t *testing.T) {
	f := raidwide(5, 3)
	f.Events[2].Amount = model.Some(101)
	f.Events = append(f.Events, hit(9950, boss, victim, 82, 100))
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnavoidable, "high", model.OwnerNobody, "")
	f = slowDeath(6000)
	f.Events[1].SpellID = 82
	f.Events = append(f.Events, hit(9999, boss, tank, 82, 1), hit(9999, boss, other, 82, 1))
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnavoidable, "medium", model.OwnerNobody, "")
	c := config()
	mechanic(&c, 82)
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseAvoidable, "medium", model.OwnerPlayer, "", victim.Name)
}
func TestOneShotThresholdsAndWindow(t *testing.T) {
	for _, tc := range []struct {
		at, amount, over int64
		match            bool
	}{{8500, 1000, -1, true}, {8500, 999, -1, false}, {8500, 1001, 2, false}, {8499, 1000, -1, false}} {
		f := quickDeath()
		f.Events[2] = hit(tc.at, boss, victim, 81, tc.amount)
		f.Events[2].Overkill = model.Some(tc.over)
		d := forChronologicalVictim(t, f, config())
		if tc.match {
			wantDeath(t, d, model.CauseUnavoidable, "high", model.OwnerNobody, "")
		} else {
			wantDeath(t, d, model.CauseUnresolved, "low", model.OwnerUndecided, "")
		}
	}
	f := quickDeath()
	f.Events[2].Amount = model.Some(500)
	f.Events = append(f.Events, hit(8500, boss, victim, 82, 500))
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnavoidable, "high", model.OwnerNobody, "")
	c := config()
	c.Scoring.Death.OneShotWindowMs = 1499
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f.Events[0].InfoUnit = boss.ID
	f.Events[1].InfoUnit = boss.ID
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestEnvironmentalNeverChargesHealers(t *testing.T) {
	f := slowDeath(6000)
	f.Events[1].Src = model.Actor{}
	f.Events[1].SpellID = 0
	f.Events[3].Src = model.Actor{}
	f.Events[3].SpellID = 0
	f.Events[3].SpellName = "FALLING"
	wantDeath(t, forChronologicalVictim(t, f, config()), model.CauseUnavoidable, "high", model.OwnerNobody, "")
	c := config()
	mechanic(&c, 0)
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
