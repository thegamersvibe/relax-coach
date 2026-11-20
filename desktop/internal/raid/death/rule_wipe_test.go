package death_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func TestWipeShareFirstMomentAndStrictAfter(t *testing.T) {
	exactConfig := config()
	exactConfig.Scoring.Death.WipeDeadShare = .4
	exact := fight(died(1000, other), died(2000, tank), died(2001, victim))
	wantDeath(t, forVictim(t, exact, exactConfig), "after the wipe call", "high", "nobody", "")
	c := config()
	c.Scoring.Death.WipeDeadShare = .4
	f := fight(died(1000, other), died(2000, tank), died(2000, healer), died(2001, victim))
	ds, _ := death.Review(f, c)
	for i := 0; i < 3; i++ {
		wantDeath(t, ds[i], "unresolved", "low", "undecided", "")
	}
	wantDeath(t, ds[3], "after the wipe call", "high", "nobody", "")
	c.Scoring.Death.WipeDeadShare = .61
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
	f.Events = append(f.Events, model.Event{TimeMs: 2000, Kind: model.Resurrect, Dst: other}, model.Event{TimeMs: 2000, Kind: model.Resurrect, Dst: tank})
	c.Scoring.Death.WipeDeadShare = .4
	wantDeath(t, forVictim(t, f, c), "after the wipe call", "high", "nobody", "") // first moment stays latched
}
func TestWipeRoleTogglesAndNoVacuousRoles(t *testing.T) {
	emptyRoles := fight(died(5000, other), died(10000, victim))
	emptyRoles.Roles = map[string]string{victim.ID: "dps"}
	emptyConfig := config()
	emptyConfig.Scoring.Death.WipeAllTanksDead = true
	emptyConfig.Scoring.Death.WipeAllHealersDead = true
	wantDeath(t, forVictim(t, emptyRoles, emptyConfig), "unresolved", "low", "undecided", "")
	for _, role := range []string{"tank", "healer"} {
		f := fight(died(5000, tank), died(5000, healer), died(5000, healer2), died(10000, victim))
		c := config()
		c.Scoring.Death.WipeAllTanksDead = role == "tank"
		c.Scoring.Death.WipeAllHealersDead = role == "healer"
		wantDeath(t, forVictim(t, f, c), "after the wipe call", "high", "nobody", "")
		c.Scoring.Death.WipeAllTanksDead = false
		c.Scoring.Death.WipeAllHealersDead = false
		wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
	}
	c := config()
	c.Scoring.Death.WipeAllTanksDead = true
	c.Scoring.Death.WipeAllHealersDead = true
	f := quickDeath()
	f.Roles = map[string]string{victim.ID: "dps"}
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
	f = quickDeath()
	f.Events = append(f.Events, died(5000, healer))
	c.Scoring.Death.WipeAllTanksDead = false
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "") // one healer still alive
}
func TestWipePetsFeignsDuplicatesAndResurrection(t *testing.T) {
	feigns := fight(modeltest.At(5000).Dst(tank).Died(1).Build(), modeltest.At(5001).Dst(other).Died(1).Build(), died(10000, victim))
	feignConfig := config()
	feignConfig.Scoring.Death.WipeDeadShare = .4
	wantDeath(t, forVictim(t, feigns, feignConfig), "unresolved", "low", "undecided", "")
	c := config()
	c.Scoring.Death.WipeDeadShare = .4
	f := quickDeath()
	pet := modeltest.Pet("Synthetic pet", victim)
	f.Owners = map[string]string{pet.ID: victim.ID}
	f.Events = append(f.Events, died(5000, pet), modeltest.At(5000).Dst(tank).Died(1).Build())
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
	f = quickDeath()
	f.Events = append(f.Events, died(5000, tank), died(5001, tank))
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
	f = quickDeath()
	f.Events = append(f.Events, died(5000, tank), model.Event{TimeMs: 6000, Kind: model.Resurrect, Dst: tank}, died(7000, other))
	wantDeath(t, forVictim(t, f, c), "unresolved", "low", "undecided", "")
}
func TestWipePriorityAndSummary(t *testing.T) {
	c := config()
	c.Scoring.Death.WipeDeadShare = .4
	f := quickDeath()
	f.Success = model.Some(0)
	f.Events = append(f.Events, died(0, tank), died(1, other), died(10001, healer))
	mechanic(&c, 81)
	ds, ws := death.Review(f, c)
	wantDeath(t, forVictim(t, f, c), "after the wipe call", "high", "nobody", "")
	if !reflect.DeepEqual(ws, []model.Wipe{modeltest.Wipe(f.ID, 0, 1, 2)}) {
		t.Fatalf("wipes=%v deaths=%d", ws, len(ds))
	}
	c.Scoring.Death.SummaryWindowMs = ptr(10001)
	_, ws = death.Review(f, c)
	if !reflect.DeepEqual(ws, []model.Wipe{modeltest.Wipe(f.ID, 0, 1, 2, 3)}) {
		t.Fatalf("wipes=%v", ws)
	}
	c.Scoring.Death.SummaryWindowMs = nil
	_, ws = death.Review(f, c)
	if len(ws) != 0 {
		t.Fatal("unconfigured summary window guessed")
	}
	c = config()
	f.Success = model.Some(1)
	_, ws = death.Review(f, c)
	if len(ws) != 0 {
		t.Fatal("kill has wipe summary")
	}
	f.Success = model.Opt{}
	_, ws = death.Review(f, c)
	if len(ws) != 0 {
		t.Fatal("unknown outcome has wipe summary")
	}
	f.Success = model.Some(0)
	f.Boss = false
	_, ws = death.Review(f, c)
	if len(ws) != 0 {
		t.Fatal("trash has wipe summary")
	}
}
func TestWipeSummaryNoDeathBeforeEnd(t *testing.T) {
	_, end := modeltest.Encounter(model.Encounter{Success: model.Some(0)}, 0, 5000)
	f := fight(end)
	f.Success = model.Some(0)
	for _, tail := range []bool{false, true} {
		if tail {
			f.Events = append(f.Events, died(5001, victim))
		}
		_, ws := death.Review(f, config())
		if len(ws) != 1 || ws[0].FirstDeath != -1 || len(ws[0].Following) != 0 {
			t.Fatalf("empty wipe=%v", ws)
		}
	}
}
