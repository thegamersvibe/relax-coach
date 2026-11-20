package score_test

import (
	"fmt"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestExactCoverageEdges(t *testing.T) {
	for _, tc := range []struct {
		coverage    float64
		stored      int
		score, rank bool
	}{{14.73, 14, false, false}, {15, 15, true, false}, {59.90, 59, true, false}, {60, 60, true, true}} {
		t.Run(fmt.Sprint(tc.coverage), func(t *testing.T) {
			c := config(t)
			c.Scoring.Weights.DPS = cfg.PartWeights{Mechanics: 100}
			c.Scoring.MeasureWeights["mechanics"] = map[string]float64{"deaths": tc.coverage, "avoidable": 100 - tc.coverage, "boss_swings": 0}
			f := fixture("dps", "dps")
			n := score.Score([]model.Fight{f}, nil, nil, c)[0]
			p := part(t, n, "mechanics")
			if p.Coverage != tc.stored || n.Coverage != tc.stored || n.Fights[0].Coverage != tc.stored || n.Tenths.Present != tc.score || n.Fights[0].Tenths.Present != tc.score || n.Rank.Present != tc.rank {
				t.Fatalf("exact=%v part/fight/night=%d/%d/%d scored=%v/%v ranked=%v", tc.coverage, p.Coverage, n.Fights[0].Coverage, n.Coverage, n.Fights[0].Tenths.Present, n.Tenths.Present, n.Rank.Present)
			}
		})
	}
}

func TestExactNightCoverageBeforeFloor(t *testing.T) {
	for _, tc := range []struct {
		measured, partial, stored int
		scored, ranked            bool
	}{
		{9, 2, 14, false, false}, // 18*9/11 = 14.727...; part/fight exactly 18.
		{5, 1, 15, true, false},
		{17, 4, 59, true, false}, // 74*17/21 = 59.904...
		{30, 7, 60, true, true},
	} {
		c := config(t)
		coverage := 18.0
		if tc.stored >= 59 {
			coverage = 74
		}
		c.Scoring.Weights.DPS = cfg.PartWeights{Output: coverage, Utility: 100 - coverage}
		var fs []model.Fight
		for i := 0; i < tc.measured+tc.partial; i++ {
			f := fixture("dps", "dps")
			f.ID = fmt.Sprint(i)
			f.Partial = i >= tc.measured
			fs = append(fs, f)
		}
		n := score.Score(fs, nil, nil, c)[0]
		if n.Coverage != tc.stored || n.Tenths.Present != tc.scored || n.Rank.Present != tc.ranked {
			t.Fatalf("%+v: night=%d score=%v rank=%v", tc, n.Coverage, n.Tenths.Present, n.Rank.Present)
		}
	}
}

func TestExactFractionalThresholdSurvivesStoredFloor(t *testing.T) {
	c := config(t)
	c.Scoring.Confidence.MinCoverage = .599
	c.Scoring.Weights.DPS = cfg.PartWeights{Mechanics: 100}
	c.Scoring.MeasureWeights["mechanics"] = map[string]float64{"deaths": 59.9, "avoidable": 40.1}
	n := score.Score([]model.Fight{fixture("dps")}, nil, nil, c)[0]
	if n.Coverage != 59 || !n.Rank.Present {
		t.Fatalf("exact threshold lost through stored coverage: coverage=%d rank=%v", n.Coverage, n.Rank)
	}
}
