package score_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"testing"
)

func TestMechanicsOwnership(t *testing.T) {
	f := fixture("dps", "tank")
	a, b := f.Players[0], f.Players[1]
	boss := model.Actor{ID: f.BossUnits[0]}
	f.Events = append(f.Events, mt.At(110000).Src(boss).Dst(a).Damage(1, 0).Landed().Build(), model.Event{TimeMs: 110000, Kind: model.Damage, Src: boss, Dst: a, Stream: model.SwingSource}, mt.At(111000).Src(boss).Dst(a).Damage(1, 0).Landed().Build(), model.Event{TimeMs: 112000, Kind: model.Interrupt, Src: a}, model.Event{TimeMs: 113000, Kind: model.Dispel, Src: a})
	ds := []model.Death{
		mt.DeathOf(a, f.ID, 100000, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build(),
		mt.DeathOf(b, f.ID, 160000, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build(),
		mt.DeathOf(a, f.ID, 120000, model.CauseUnhealed).Owner(model.OwnerHealers, a.Name).Build(),
		mt.DeathOf(a, f.ID, 120000, model.CauseUnresolved).Owner(model.OwnerUndecided).Build(),
		mt.DeathOf(a, f.ID, 140000, model.CauseAfterWipe).Owner(model.OwnerPlayer, a.Name).Build(),
		mt.DeathOf(a, "other", 100000, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build(),
		mt.DeathOf(a, f.ID, 100000, model.CauseAvoidable).Owner(model.OwnerUndecided, a.Name).Build(), // ownership, never a stray name, grants a charge
	}
	ns := score.Score([]model.Fight{f}, ds, nil, config(t))
	p := part(t, playerScore(t, ns, a), "mechanics")
	if p.Coverage != 70 || p.Evidence.Counts["points_tenths.deaths"] != 400 || p.Evidence.Counts["points_tenths.boss_swings"] != 960 || p.Evidence.Counts["owned_deaths"] != 2 || p.Tenths != model.Some(560) || p.Evidence.Counts["measured.avoidable"] != 0 {
		t.Fatalf("mechanics %+v", p)
	}
	tank := part(t, playerScore(t, ns, b), "mechanics")
	if tank.Coverage != 50 || tank.Tenths != model.Some(1000) {
		t.Fatalf("tank %+v", tank)
	}
	if _, ok := tank.Evidence.Counts["measured.boss_swings"]; ok {
		t.Fatal("tank has no boss-swings measure")
	}
	utility := part(t, playerScore(t, ns, a), "utility")
	if utility.Tenths.Present || utility.Evidence.Counts["interrupts"] != 1 || utility.Evidence.Counts["dispels"] != 1 {
		t.Fatalf("utility %+v", utility)
	}
}
func TestMechanicsConfigAndFloor(t *testing.T) {
	f := fixture("dps")
	a := f.Players[0]
	c := config(t)
	ds := []model.Death{mt.DeathOf(a, f.ID, 130000, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build()}
	c.Scoring.CountPenalties["owned_death"] = 20
	p := part(t, score.Score([]model.Fight{f}, ds, nil, c)[0], "mechanics")
	if p.Evidence.Counts["points_tenths.deaths"] != 850 {
		t.Fatalf("middle severity %+v", p)
	}
	for i := 0; i < 10; i++ {
		ds = append(ds, ds[0])
	}
	p = part(t, score.Score([]model.Fight{f}, ds, nil, c)[0], "mechanics")
	if p.Evidence.Counts["points_tenths.deaths"] != 0 {
		t.Fatal("not floored")
	}
	delete(c.Scoring.CountPenalties, "owned_death")
	p = part(t, score.Score([]model.Fight{f}, ds, nil, c)[0], "mechanics")
	if p.Coverage != 20 || p.Evidence.Counts["measured.deaths"] != 0 {
		t.Fatalf("unknown penalty %+v", p)
	}
}
