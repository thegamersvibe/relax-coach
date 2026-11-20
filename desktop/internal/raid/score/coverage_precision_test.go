package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestSubHundredthPartCannotPublishZeroCoverage(t *testing.T) {
	for _, weight := range []float64{.9, 1} {
		c := config(t)
		c.Scoring.MeasureWeights["mechanics"] = map[string]float64{"deaths": weight, "avoidable": 100 - weight}
		n := score.Score([]model.Fight{fixture("tank")}, nil, nil, c)[0]
		p := part(t, n, "mechanics")
		if p.Tenths.Present != (weight == 1) || p.Coverage != int(weight) {
			t.Fatalf("weight=%v: part=%+v", weight, p)
		}
	}
}
