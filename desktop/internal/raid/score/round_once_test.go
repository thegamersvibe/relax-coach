package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestMeasuresRoundOnlyAtPart(t *testing.T) {
	f := fixture("dps", "dps", "dps")
	a := f.Players[0]
	c := config(t)
	ds := []model.Death{mt.DeathOf(a, f.ID, f.StartMs+90, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build()}
	p := part(t, playerScore(t, score.Score([]model.Fight{f}, ds, nil, c), a), "mechanics")
	// Raw death points 60.03; (50*60.03 + 20*100)/70 = 71.45 -> 71.5.
	// The rounded display measure is 60.0 but is never an aggregation input.
	if p.Tenths != model.Some(715) || p.Evidence.Counts["points_tenths.deaths"] != 600 {
		t.Fatalf("double-rounded measures: part=%v evidence=%v", p.Tenths, p.Evidence.Counts)
	}
}
