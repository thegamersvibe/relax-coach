package score_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"testing"
)

func healingFixture() model.Fight {
	f := fixture("healer", "healer", "dps")
	a, b, target := f.Players[0], f.Players[1], f.Players[2]
	pet := mt.Pet("Healing Spring", a)
	f.Owners = map[string]string{pet.ID: a.ID}
	f.Events = append(f.Events,
		model.Event{TimeMs: 119000, InfoUnit: target.ID, HP: model.Some(50), HPMax: model.Some(100)},
		mt.At(120000).Src(pet).Dst(target).Heal(100, 40).Build(),
		model.Event{TimeMs: 121000, Kind: model.Absorb, Src: model.Actor{ID: f.BossUnits[0]}, Dst: target, Caster: a, Amount: model.Some(40)},
		model.Event{TimeMs: 121000, Kind: model.Damage, Src: model.Actor{ID: f.BossUnits[0]}, Dst: target, Absorbed: model.Some(40)},
		model.Event{TimeMs: 121000, Kind: model.Missed, Src: target, Dst: target, Amount: model.Some(40), MissType: "ABSORB"},
		mt.At(122000).Src(b).Dst(target).Heal(100, -1).Build(),
		model.Event{TimeMs: 123000, InfoUnit: target.ID, HP: model.Some(51), HPMax: model.Some(100)},
		mt.At(124000).Src(a).Dst(target).Heal(9000, 0).Build(),
		mt.At(125000).Src(a).Dst(b).Heal(9000, 0).Build()) // target without samples is unknown, not low.
	return f
}
func TestHealerHalvesAndAbsorbs(t *testing.T) {
	f := healingFixture()
	c := config(t)
	a, b := f.Players[0], f.Players[1]
	ns := score.Score([]model.Fight{f}, nil, nil, c)
	for _, actor := range []model.Actor{a, b} {
		p := part(t, playerScore(t, ns, actor), "output")
		if p.Tenths != model.Some(850) || p.Coverage != 100 || !p.Provisional || p.Baseline != "role" || p.Evidence.Counts["low_health_effective"] != 100 || p.Evidence.Counts["team_low_health_effective"] != 200 {
			t.Fatalf("healer %+v", p)
		}
	}
	ds := []model.Death{mt.DeathOf(f.Players[2], f.ID, 130000, model.CauseUnhealed).Owner(model.OwnerHealers, a.Name, b.Name).Build()}
	ns = score.Score([]model.Fight{f}, ds, nil, c)
	p := part(t, playerScore(t, ns, a), "output")
	if p.Tenths != model.Some(350) || p.Evidence.Counts["unhealed_deaths"] != 1 || part(t, playerScore(t, ns, a), "mechanics").Evidence.Counts["points_tenths.deaths"] != 1000 {
		t.Fatalf("double death charge %+v", p)
	}
	f.Events = append(f.Events, mt.At(126000).Dst(a).Died(0).Build())
	// Only R6-produced ownership may change the charge, not score survival.
	ds[0].Owners = []string{b.Name}
	p = part(t, playerScore(t, score.Score([]model.Fight{f}, ds, nil, c), a), "output")
	if p.Evidence.Counts["points_tenths.healer_team"] != 1000 {
		t.Fatal("dead healer charged")
	}
}
func TestHealerUnmeasuredAndCap(t *testing.T) {
	f := fixture("healer")
	c := config(t)
	p := part(t, score.Score([]model.Fight{f}, nil, nil, c)[0], "output")
	if p.Coverage != 50 || p.Tenths != model.Some(1000) || p.Evidence.Counts["measured.healer_individual"] != 0 {
		t.Fatalf("lone healer %+v", p)
	}
	f = healingFixture()
	a := f.Players[0]
	c.Scoring.Healer.UnhealedTeamScore = 80
	c.Scoring.Healer.UnhealedOutputCap = 50
	ds := []model.Death{mt.DeathOf(f.Players[2], f.ID, 130000, model.CauseUnhealed).Owner(model.OwnerHealers, a.Name).Build()}
	p = part(t, playerScore(t, score.Score([]model.Fight{f}, ds, nil, c), a), "output")
	if p.Tenths != model.Some(500) {
		t.Fatalf("cap %+v", p)
	}
	c.Scoring.Healer.LowHealthFraction = nil
	p = part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, c), a), "output")
	if p.Coverage != 50 {
		t.Fatalf("unknown cutoff %+v", p)
	}
}
