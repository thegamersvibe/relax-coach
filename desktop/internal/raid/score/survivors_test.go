package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestSwingStreamsAreSeparate(t *testing.T) {
	f := fixture("dps", "dps", "dps")
	a, boss := f.Players[0], model.Actor{ID: f.BossUnits[0]}
	f.Events = append(f.Events,
		model.Event{TimeMs: 120000, Kind: model.Damage, Src: a, Dst: boss, Stream: model.SwingSource, Amount: model.Some(9000)},
		mt.At(121000).Src(a).Dst(boss).Damage(3000, -1).Landed().Build(),
		model.Event{TimeMs: 130000, Kind: model.Damage, Src: boss, Dst: a, Stream: model.SwingSource, Amount: model.Some(200)},
		mt.At(131000).Src(boss).Dst(a).Damage(100, -1).Landed().Build())
	n := playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a)
	if got := part(t, n, "output").Evidence.Counts["boss_damage"]; got != 9000 {
		t.Fatalf("source damage counted: %d", got)
	}
	if got := part(t, n, "activity").Evidence.Counts["active_ms"]; got != 3000 {
		t.Fatalf("source activity counted: %d", got)
	}
	if got := part(t, n, "mechanics").Evidence.Counts["boss_swings_taken"]; got != 1 {
		t.Fatalf("source boss swings counted: %d", got)
	}
}

func TestZeroAliveHasNoRelativeOrHealerOutput(t *testing.T) {
	for _, role := range []string{"dps", "healer"} {
		f := fixture(role, role, role)
		a := f.Players[0]
		f.Events = append(f.Events, mt.At(f.StartMs).Dst(a).Died(0).Build())
		n := playerScore(t, score.Score([]model.Fight{f}, nil, nil, config(t)), a)
		for _, name := range []string{"output", "activity"} {
			p := part(t, n, name)
			if p.Tenths.Present || p.Coverage != 0 {
				t.Fatalf("zero alive %s %s measured: %+v", role, name, p)
			}
		}
	}
}
