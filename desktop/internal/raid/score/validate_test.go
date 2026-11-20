package score_test

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestScoreOutputGuard(t *testing.T) {
	c := config(t)
	c.Scoring.MeasureWeights["output"]["boss_damage"] = 101
	defer func() {
		if recover() == nil {
			t.Fatal("invalid generated part escaped output guard")
		}
	}()
	score.Score([]model.Fight{fixture("dps", "dps")}, nil, nil, c)
}

func TestWriterGuard(t *testing.T) {
	for _, tc := range []struct {
		name, baseline string
		coverage       int
		bad            bool
	}{
		{"output", "role", 100, false}, {"activity", "guild", 1, false}, {"output", "own", 50, false},
		{"mechanics", "", 70, false}, {"preparation", "", 60, false},
		{"output", "role", 0, true}, {"mechanics", "", -1, true}, {"activity", "role", 101, true},
		{"output", "spec", 100, true}, {"output", "", 100, true}, {"activity", "", 100, true},
	} {
		p := mt.NewPart(tc.name, 700, mt.Evidence("Measured.", nil, "observations", 1)).Coverage(tc.coverage).Baseline(tc.baseline).Build()
		if err := score.ValidatePart(p); (err != nil) != tc.bad {
			t.Fatalf("%+v: %v", tc, err)
		}
		n := mt.NightScore(mt.Player("Alia-PvP-EU"), "role").Fights(mt.FightScore("f", mt.Player("Alia-PvP-EU"), "dps").Parts(p).Build()).Build()
		if err := score.Validate([]model.NightScore{n}); (err != nil) != tc.bad {
			t.Fatalf("aggregate %+v: %v", tc, err)
		}
	}
	if err := score.ValidatePart(mt.Unmeasured("output")); err != nil {
		t.Fatal(err)
	}
}
