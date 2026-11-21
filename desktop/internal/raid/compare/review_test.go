package compare

import (
	"math/big"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestCompareNoScoredFightsReason(t *testing.T) {
	for _, noScores := range []bool{true, false} {
		name := "zero-weight-wipes-only"
		if noScores {
			name = "no-fight-scores"
		}
		t.Run(name, func(t *testing.T) {
			ns, roster, fights := fixture(6)
			c := config(t)
			if noScores {
				ns[5].Fights = nil
			} else {
				c.Scoring.Night.WipeWeight = 0
				fights = append(fights, model.Fight{ID: "wipe", Boss: true, Success: model.Some(0)})
				ns[5].Fights[0].FightID = "wipe"
			}
			summary, err := score.NightRoleParts(fights, ns[5], c)
			if err != nil || summary.Role != "" || summary.Split {
				t.Fatalf("fixture must have no scored night role, not a split: %+v %v", summary, err)
			}
			got, err := Compare(ns, roster, c, officer(roster[5].Name, roster[0].Name), fights)
			if err != nil || len(got.Top) != 1 {
				t.Fatalf("compare: %+v %v", got, err)
			}
			e := got.Top[0]
			if e.Verdict != "disagreement" || e.Part != nil || e.PartReason != "no scored fights, no part comparison" {
				t.Fatalf("ranked player with no scored night role: %+v", e)
			}
		})
	}
}

func TestComparePartExplainsDisagreement(t *testing.T) {
	// Adapt the review's fixed-rank repros: rank need not agree with any
	// individual role-relative part. Only a delta in the right direction explains it.
	for _, tc := range []struct {
		name             string
		top, zeroWeights bool
		output, activity int64
		wantPart         string
		wantImpact       int64
	}{
		{name: "review-top-above-every-median", top: true, output: 900, activity: 800},
		{name: "review-bottom-below-every-median", output: 100, activity: 200},
		{name: "top-at-median", top: true, output: 500, activity: 500},
		{name: "bottom-at-median", output: 500, activity: 500},
		{name: "top-zero-weight", top: true, zeroWeights: true, output: 100, activity: 200},
		{name: "bottom-zero-weight", zeroWeights: true, output: 900, activity: 800},
		{name: "top-negative-control", top: true, output: 100, activity: 200, wantPart: "output", wantImpact: -1600},
		{name: "bottom-positive-control", output: 900, activity: 800, wantPart: "output", wantImpact: 1600},
		{name: "top-mixed-sign-control", top: true, output: 900, activity: 200, wantPart: "activity", wantImpact: -750},
		{name: "bottom-mixed-sign-control", output: 100, activity: 800, wantPart: "activity", wantImpact: 750},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ns, roster, fights := fixture(6)
			c := config(t)
			c.Scoring.Weights.DPS = cfg.PartWeights{Output: 40, Activity: 25, Mechanics: 35}
			if tc.zeroWeights {
				c.Scoring.Weights.DPS = cfg.PartWeights{Mechanics: 100}
			}
			for i := range ns {
				ns[i].Fights[0].Parts = []model.Part{{Name: "output", Tenths: model.Some(500)}, {Name: "activity", Tenths: model.Some(500)}}
			}
			pick := 0
			if tc.top {
				pick = 5
			}
			ns[pick].Fights[0].Parts[0].Tenths = model.Some(tc.output)
			ns[pick].Fights[0].Parts[1].Tenths = model.Some(tc.activity)
			got, err := Compare(ns, roster, c, officer(roster[5].Name, roster[0].Name), fights)
			if err != nil || len(got.Top) != 1 || len(got.Bottom) != 1 {
				t.Fatalf("compare: %+v %v", got, err)
			}
			e := got.Bottom[0]
			if tc.top {
				e = got.Top[0]
			}
			if e.Verdict != "disagreement" {
				t.Fatalf("fixture must disagree: %+v", e)
			}
			if tc.wantPart == "" {
				if e.Part != nil || e.PartReason != "no single part explains it" {
					t.Fatalf("non-explanatory delta returned as the reason: part=%+v reason=%q", e.Part, e.PartReason)
				}
			} else if e.Part == nil || e.PartReason != "" || e.Part.Name != tc.wantPart || e.Part.Median.Cmp(big.NewRat(50, 1)) != 0 || e.Part.Impact.Cmp(big.NewRat(tc.wantImpact, 1)) != 0 {
				t.Fatalf("lost explanatory part %q with impact %d: part=%+v reason=%q", tc.wantPart, tc.wantImpact, e.Part, e.PartReason)
			}
		})
	}
}
