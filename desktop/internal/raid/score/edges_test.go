package score_test

import (
	"encoding/json"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"testing"
)

func TestZeroBaselinesAndNullableConfig(t *testing.T) {
	f := fixture("dps", "dps")
	f.Events = f.Events[:2]
	c := config(t)
	for _, n := range score.Score([]model.Fight{f}, nil, nil, c) {
		if n.Tenths.Present || n.Coverage != 0 {
			t.Fatalf("zero median invented baseline: %v/%d", n.Tenths, n.Coverage)
		}
	}
	f = fixture("dps", "dps")
	c.Scoring.GCDMs = nil
	for _, n := range score.Score([]model.Fight{f}, nil, nil, c) {
		if part(t, n, "activity").Tenths.Present || n.Coverage != 49 {
			t.Fatal("null gcd fabricated")
		}
	}
	c.Scoring.RoundingStep = nil
	for _, n := range score.Score([]model.Fight{f}, nil, nil, c) {
		if n.Tenths.Present || n.Coverage != 0 {
			t.Fatal("null step scored")
		}
	}
}
func TestRoundingAndClockShift(t *testing.T) {
	f := fixture("tank")
	c := config(t)
	c.Scoring.CountPenalties["owned_death"] = 10.05
	a := f.Players[0]
	ds := []model.Death{mt.DeathOf(a, f.ID, f.StartMs, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build()}
	ns := score.Score([]model.Fight{f}, ds, nil, c)
	if part(t, ns[0], "mechanics").Tenths != model.Some(900) {
		t.Fatal("half-away decimal rounding")
	}
	before, _ := json.Marshal(ns)
	f.StartMs += 1234567
	f.EndMs += 1234567
	for i := range f.Events {
		f.Events[i].TimeMs += 1234567
	}
	ds[0].TimeMs += 1234567
	after, _ := json.Marshal(score.Score([]model.Fight{f}, ds, nil, c))
	if string(before) != string(after) {
		t.Fatal("clock origin changes score")
	}
}
func TestUnhealedAtEndAndTail(t *testing.T) {
	f := healingFixture()
	c := config(t)
	a, b := f.Players[0], f.Players[1]
	f.Events = append(f.Events, mt.At(159000).Dst(b).Died(0).Build())
	for _, at := range []int64{160000, 160035} {
		ds := []model.Death{mt.DeathOf(f.Players[2], f.ID, at, model.CauseUnhealed).Owner(model.OwnerHealers, a.Name).Build()}
		ns := score.Score([]model.Fight{f}, ds, nil, c)
		if part(t, playerScore(t, ns, a), "output").Evidence.Counts["unhealed_deaths"] != 1 || part(t, playerScore(t, ns, b), "output").Evidence.Counts["unhealed_deaths"] != 0 {
			t.Fatal("end/tail must charge only living healers")
		}
	}
}
func TestTrashHostileOnly(t *testing.T) {
	f := fixture("dps", "dps")
	f.Boss = false
	a := f.Players[0]
	npc := mt.Creature("Hostile Target", 45)
	friendly := mt.Creature("Friendly Target", 46)
	friendly.Flags = 0x10
	pet := mt.Creature("Owned Guardian", 47)
	f.Owners = map[string]string{pet.ID: f.Players[1].ID}
	f.Events = append(f.Events, mt.At(120000).Src(a).Dst(npc).Damage(1000, -1).Build(), mt.At(121000).Src(a).Dst(friendly).Damage(9000, 0).Build(), mt.At(122000).Src(a).Dst(pet).Damage(9000, 0).Build())
	p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a), "output")
	if p.Evidence.Counts["boss_damage"] != 7000 {
		t.Fatalf("trash %+v", p)
	}
}
