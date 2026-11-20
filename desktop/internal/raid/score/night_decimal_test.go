package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestNightExactDecimalTrashCap(t *testing.T) {
	// Review regression: accepted .2/.2/.8 shares must not lose coverage to
	// the binary-float complement of .8 before the exact provisional gate.
	c := tunedScoring(t, func(m map[string]any) {
		n := m["night"].(map[string]any)
		n["trash_share"], n["max_trash_share"], n["min_boss_share"] = .2, .2, .8
		m["weights"].(map[string]any)["tank"] = map[string]any{"output": 0, "activity": 0, "mechanics": 100, "utility": 0, "preparation": 0}
		m["measure_weights"].(map[string]any)["mechanics"] = map[string]any{"deaths": 75, "boss_swings": 0, "avoidable": 25}
	})
	f := fixture("tank")
	f.ID, f.Boss = "invented-trash", false
	for _, e := range f.Events {
		if e.OffsetH != 0 {
			t.Fatal("need offset-zero control without preparation")
		}
	}
	nights := score.Score([]model.Fight{f}, nil, nil, c)
	if len(nights) != 1 || len(nights[0].Fights) != 1 {
		t.Fatal("need one scored player/fight")
	}
	n := nights[0]
	if n.Fights[0].Tenths != model.Some(1000) || n.Fights[0].Coverage != 75 {
		t.Fatalf("fight control: score=%v coverage=%d", n.Fights[0].Tenths, n.Fights[0].Coverage)
	}
	t.Logf("shares=.2/.2/.8 fight_coverage=%d night_score=%v night_coverage=%d rank=%v", n.Fights[0].Coverage, n.Tenths, n.Coverage, n.Rank)
	if n.Tenths != model.Some(1000) || n.Coverage != 15 || n.Rank.Present {
		t.Fatal("exact 20% of 75 must reach coverage 15 and the provisional score gate, without a rank")
	}
}
