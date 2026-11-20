package compare

import (
	"math/big"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

func parts(n *model.NightScore, output, mechanics int64) {
	n.Fights[0].Parts = []model.Part{{Name: "output", Tenths: model.Some(output)}, {Name: "mechanics", Tenths: model.Some(mechanics)}}
}

func TestCompareWeightedParts(t *testing.T) {
	ns, roster, fights := fixture(6)
	for i, p := range [][2]int64{{800, 900}, {600, 500}, {700, 500}, {400, 490}, {0, 0}, {0, 0}} {
		parts(&ns[i], p[0], p[1])
	}
	ns[4].Fights[0].Role = "healer" // ranked but not a DPS peer
	ns[5].Rank = model.Opt{}        // measured parts of provisional players aren't peers
	c := config(t)
	c.Scoring.Weights.DPS = cfg.PartWeights{Output: 10, Activity: 20, Mechanics: 35, Utility: 25, Preparation: 10}
	f := officer(roster[3].Name, roster[0].Name)
	got, err := Compare(ns, roster, c, f, fights)
	if err != nil || len(got.Top) != 1 || len(got.Bottom) != 1 {
		t.Fatalf("compare: %+v %v", got, err)
	}
	a, b := got.Top[0].Part, got.Bottom[0].Part
	if a == nil || b == nil {
		t.Fatal("disagreement has no part explanation")
	}
	if a.Name != "output" || a.Role != "dps" || a.Score.Cmp(big.NewRat(40, 1)) != 0 || a.Median.Cmp(big.NewRat(65, 1)) != 0 || a.Weight.Cmp(big.NewRat(10, 1)) != 0 || a.Impact.Cmp(big.NewRat(-250, 1)) != 0 {
		t.Fatalf("top: %+v", a)
	}
	if b.Name != "mechanics" || b.Median.Cmp(big.NewRat(50, 1)) != 0 || b.Impact.Cmp(big.NewRat(1400, 1)) != 0 {
		t.Fatalf("bottom: %+v", b)
	}
	c.Scoring.Weights.DPS = cfg.PartWeights{Output: 1, Activity: 14, Mechanics: 50, Utility: 25, Preparation: 10}
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Name != "mechanics" {
		t.Fatal("configured role weights ignored")
	}
}

func TestCompareRoleAvailability(t *testing.T) {
	ns, roster, fights := fixture(4)
	c := config(t)
	f := officer(roster[3].Name, roster[0].Name)
	ns[3].Fights[0].Role = "tank"
	ns[3].Fights[0].Parts[0].Tenths = model.Some(500) // below the eventual tank median
	got, err := Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part != nil || got.Top[0].PartReason != "no comparison in his role" {
		t.Fatal("lone role fabricated median")
	}
	// Two ranked tanks suffice, but each part independently requires two values.
	ns[2].Fights[0].Role = "tank"
	ns[2].Fights[0].Parts[0].Tenths = model.Opt{}
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part != nil || got.Top[0].PartReason != "no comparison in his role" {
		t.Fatal("unmeasured peer fabricated zero")
	}
	ns[2].Fights[0].Parts[0].Tenths = model.Some(700)
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Median.Cmp(big.NewRat(60, 1)) != 0 || got.Top[0].Part.Weight.Cmp(big.NewRat(10, 1)) != 0 {
		t.Fatal("two measured ranked tanks should compare")
	}
	ns[3].Fights = append(ns[3].Fights, model.FightScore{FightID: "second", Role: "healer", Tenths: model.Some(600)})
	fights = append(fights, model.Fight{ID: "second", Boss: true, Success: model.Some(1)})
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part != nil || got.Top[0].PartReason != "role split evenly, no part comparison" {
		t.Fatal("even role split fabricated explanation")
	}
	got, err = Compare(ns, roster, c, f, nil)
	if err == nil || got.Ranked != 0 || len(got.Top)+len(got.Bottom) != 0 {
		t.Fatal("missing original fights returned partial comparison")
	}
}

func TestComparePartTiesAndExactMedian(t *testing.T) {
	ns, roster, fights := fixture(4)
	c := config(t)
	f := officer(roster[3].Name, roster[0].Name)
	for i, p := range [][2]int64{{800, 750}, {600, 650}, {600, 650}, {400, 550}} {
		parts(&ns[i], p[0], p[1])
	}
	c.Scoring.Weights.DPS = cfg.PartWeights{Output: 20, Mechanics: 40, Activity: 20, Utility: 10, Preparation: 10}
	got, err := Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Name != "mechanics" || got.Bottom[0].Part.Name != "mechanics" {
		t.Fatal("equal impact must choose larger role weight")
	}
	c.Scoring.Weights.DPS = cfg.PartWeights{Output: 30, Mechanics: 30, Activity: 20, Utility: 10, Preparation: 10}
	for i := range ns {
		ns[i].Fights[0].Parts[1].Tenths = ns[i].Fights[0].Parts[0].Tenths
	}
	for range 20 {
		got, err = Compare(ns, roster, c, f, fights)
		if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Name != "mechanics" || got.Bottom[0].Part.Name != "mechanics" {
			t.Fatal("equal impact/weight must choose ASCII part name")
		}
	}
	// One part's median is 60.05, not 60.0 or 60.1 during comparison.
	parts(&ns[1], 600, 600)
	parts(&ns[2], 601, 600)
	parts(&ns[0], 800, 800)
	parts(&ns[3], 400, 400)
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Name != "output" || got.Top[0].Part.Median.Cmp(big.NewRat(1201, 20)) != 0 || got.Top[0].Part.Impact.Cmp(big.NewRat(-1203, 2)) != 0 {
		t.Fatal("median rounded before choosing part")
	}
	if got.Top[0].Part.Median.FloatString(1) != "60.1" {
		t.Fatal("display tenth")
	}
	// Odd role count: middle, not a mean. Unranked peer is not a median input.
	ns[2].Rank = model.Opt{}
	extra, rr, _ := fixture(5)
	extra[4].Fights[0].Role = "healer"
	ns = append(ns, extra[4])
	roster = append(roster, rr[4])
	got, err = Compare(ns, roster, c, f, fights)
	if err != nil || got.Top[0].Part == nil || got.Top[0].Part.Median.Cmp(big.NewRat(60, 1)) != 0 {
		t.Fatal("odd role median")
	}
}
