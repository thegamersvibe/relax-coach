package score_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"testing"
)

func playerScore(t *testing.T, ns []model.NightScore, p model.Actor) model.NightScore {
	t.Helper()
	for _, n := range ns {
		if n.PlayerID == p.ID {
			return n
		}
	}
	t.Fatal("missing player")
	return model.NightScore{}
}
func TestOutputRoleMedian(t *testing.T) {
	c := config(t)
	f := fixture("tank", "tank", "dps")
	a, b := f.Players[0], f.Players[1]
	f.Specs = map[string]string{a.ID: "WARRIOR.protection", b.ID: "DRUID.feral"}
	boss := model.Actor{ID: f.BossUnits[0]}
	pet := mt.Creature("Clockwork Hound", 43)
	f.Owners = map[string]string{pet.ID: a.ID}
	f.Events = append(f.Events,
		mt.At(120000).Src(a).Dst(boss).Damage(2000, 1000).Landed().Build(),
		model.Event{TimeMs: 120000, Src: a, Dst: boss, Kind: model.Damage, Stream: model.SwingSource, Amount: model.Some(2000), Overkill: model.Some(1000)},
		mt.At(121000).Src(pet).Dst(boss).Damage(1000, -1).Build(),
		mt.At(122000).Src(a).Dst(mt.Creature("Padding", 44)).Damage(900000, 0).Build(),
		mt.At(123000).Src(b).Dst(boss).Damage(4000, -1).Build())
	got := score.Score([]model.Fight{f}, nil, nil, c)
	pa := part(t, playerScore(t, got, a), "output")
	pb := part(t, playerScore(t, got, b), "output")
	if pa.Evidence.Counts["boss_damage"] != 8000 || pb.Evidence.Counts["boss_damage"] != 10000 || pa.Evidence.Counts["alive_ms"] != 60000 {
		t.Fatalf("raw: %+v / %+v", pa, pb)
	}
	if pa.Tenths != model.Some(544) || pb.Tenths != model.Some(811) || pa.Coverage != 100 || !pa.Provisional || pa.Baseline != "role" {
		t.Fatalf("median output: %+v / %+v", pa, pb)
	}
	if part(t, playerScore(t, got, f.Players[2]), "output").Tenths.Present {
		t.Fatal("lone DPS got baseline")
	}
}
func TestOutputAliveAndStreams(t *testing.T) {
	f := fixture("dps", "dps", "dps")
	a := f.Players[0]
	boss := model.Actor{ID: f.BossUnits[0]}
	f.Events = append(f.Events, mt.At(120000).Dst(a).Died(1).Build(), mt.At(130000).Dst(a).Died(0).Build(), model.Event{TimeMs: 140000, Kind: model.Resurrect, Dst: a}, mt.At(150000).Src(a).Dst(boss).Damage(1000, 2000).Build())
	p := part(t, playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a), "output")
	if p.Evidence.Counts["alive_ms"] != 50000 || p.Evidence.Counts["boss_damage"] != 6000 || p.Provisional || p.Tenths != model.Some(900) {
		t.Fatalf("alive %+v", p)
	}
}
