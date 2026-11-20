package death_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func mechanic(c *cfg.Config, spell int64, roles ...string) {
	c.Encounters.Encounters = []cfg.EncounterRule{{ID: 71, Mechanics: []cfg.Mechanic{{ID: "synthetic", Spell: cfg.Spell{ID: spell}, Avoidable: true, Roles: roles, AffectedPlayers: "display only", ValidResponse: "display only"}}}}
}
func quickDeath() model.Fight {
	return fight(sample(8000, 800, 1000), sample(9000, 500, 1000), hit(9900, boss, victim, 81, 100), died(10000, victim))
}
func TestAvoidableMechanicRoles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		roles   []string
		matches bool
	}{
		{"same", []string{"dps"}, true}, {"different", []string{"tank", "healer"}, false}, {"empty", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := config()
			mechanic(&c, 81, tc.roles...)
			d := forChronologicalVictim(t, quickDeath(), c)
			if tc.matches {
				wantDeath(t, d, model.CauseAvoidable, "high", model.OwnerPlayer, "", victim.Name)
			} else {
				wantDeath(t, d, model.CauseUnresolved, "low", model.OwnerUndecided, "")
			}
		})
	}
	c := config()
	mechanic(&c, 81)
	f := quickDeath()
	delete(f.Roles, victim.ID)
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseAvoidable, "high", model.OwnerPlayer, "", victim.Name)
	c.Encounters.Encounters[0].Mechanics[0].Roles = []string{"dps"}
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestAvoidableEnabledOnlyAndTextIsNotPredicate(t *testing.T) {
	c := config()
	mechanic(&c, 81, "dps")
	c.Encounters.Encounters[0].Mechanics[0].AffectedPlayers = model.OwnerNobody
	c.Encounters.Encounters[0].Mechanics[0].ValidResponse = "already done"
	wantDeath(t, forChronologicalVictim(t, quickDeath(), c), model.CauseAvoidable, "high", model.OwnerPlayer, "", victim.Name)
	c.Encounters.Encounters[0].ID++
	wantDeath(t, forChronologicalVictim(t, quickDeath(), c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	c = config()
	mechanic(&c, 81)
	c.Encounters.Encounters[0].Mechanics[0].Avoidable = false
	wantDeath(t, forChronologicalVictim(t, quickDeath(), c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	c = config()
	c.Candidates.Encounters = []cfg.Candidate{{Name: "Synthetic", Abilities: []cfg.Spell{{ID: 81}}}}
	wantDeath(t, forChronologicalVictim(t, quickDeath(), c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
func TestAvoidableKillingHitOrMajority(t *testing.T) {
	for _, tc := range []struct {
		name              string
		first, over, last int64
		matches           bool
	}{
		{"majority", 51, -1, 49, true}, {"exactly half", 50, -1, 50, false}, {"overkill removed", 150, 51, 100, false},
		{"negative overkill clamped", 50, -1, 50, false}, {"positive remainder", 151, 50, 100, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := config()
			mechanic(&c, 81)
			f := quickDeath()
			e := hit(9500, boss, victim, 81, tc.first)
			e.Overkill = model.Some(tc.over)
			f.Events[2] = hit(9900, boss, victim, 82, tc.last)
			f.Events = append(f.Events, e)
			d := forChronologicalVictim(t, f, c)
			if tc.matches {
				wantDeath(t, d, model.CauseAvoidable, "high", model.OwnerPlayer, "", victim.Name)
			} else {
				wantDeath(t, d, model.CauseUnresolved, "low", model.OwnerUndecided, "")
			}
		})
	}
	c := config()
	mechanic(&c, 81)
	f := quickDeath()
	f.Events[2].Src = victim
	f.Events = append(f.Events, modeltest.At(9900).Dst(victim).Spell(81, "Synthetic ability").Aura(model.AuraRemoved, "DEBUFF").Build())
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseAvoidable, "high", model.OwnerPlayer, "", victim.Name)
	f = quickDeath()
	f.Events[2].TimeMs = -1
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f = quickDeath()
	f.Events = append(f.Events, hit(9950, boss, victim, 82, 200))
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
	f = quickDeath()
	f.Events[2].Stream = model.SwingSource
	wantDeath(t, forChronologicalVictim(t, f, c), model.CauseUnresolved, "low", model.OwnerUndecided, "")
}
