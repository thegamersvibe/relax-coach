package score_test

import (
	"encoding/json"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"reflect"
	"slices"
	"testing"
)

func TestCorrectedCoverageTable(t *testing.T) {
	for _, tc := range []struct {
		roles         []string
		with, without int
	}{{[]string{"dps", "dps"}, 80, 74}, {[]string{"healer", "healer"}, 70, 64}, {[]string{"healer"}, 32, 26}, {[]string{"tank", "tank"}, 53, 47}, {[]string{"tank"}, 23, 17}} {
		f := fixture(tc.roles...)
		var reports []model.PrepReport
		for _, a := range f.Players {
			reports = append(reports, stock(a))
			if f.Roles[a.ID] == "healer" {
				f.Events = append(f.Events, model.Event{TimeMs: 120000, InfoUnit: a.ID, HP: model.Some(40), HPMax: model.Some(100)}, mt.At(121000).Src(a).Dst(a).Heal(100, 0).Build())
			}
		}
		for _, prep := range []bool{false, true} {
			rs := reports
			want := tc.with
			if !prep {
				rs = nil
				want = tc.without
			}
			for _, n := range score.Score([]model.Fight{f}, nil, rs, config(t)) {
				if n.Coverage != want || !n.Tenths.Present || n.Rank.Present != (want >= 60) {
					t.Fatalf("roles=%v prep=%v want=%d got=%+v", tc.roles, prep, want, n)
				}
			}
		}
	}
}
func TestRanksAndThresholds(t *testing.T) {
	f := fixture("tank", "tank", "dps", "dps", "dps")
	var reports []model.PrepReport
	for _, a := range f.Players {
		reports = append(reports, stock(a))
	}
	ds := []model.Death{mt.DeathOf(f.Players[4], f.ID, 100000, model.CauseAvoidable).Owner(model.OwnerPlayer, f.Players[4].Name).Build()}
	ns := score.Score([]model.Fight{f}, ds, reports, config(t))
	for i, a := range f.Players {
		n := playerScore(t, ns, a)
		if i < 2 {
			if n.Coverage != 53 || !n.Tenths.Present || n.Rank.Present {
				t.Fatalf("provisional tank %+v", n)
			}
		} else {
			want := int64(1)
			if i == 4 {
				want = 3
			}
			if n.Rank != model.Some(want) {
				t.Fatalf("rank %+v", n)
			}
		}
	}
	c := config(t)
	c.Scoring.Weights.Tank.Mechanics = 28
	c.Scoring.Weights.Tank.Utility = 32
	lone := fixture("tank")
	n := score.Score([]model.Fight{lone}, nil, nil, c)[0]
	if n.Coverage != 14 || n.Tenths.Present || n.Rank.Present {
		t.Fatalf("under .15 %+v", n)
	}
	c.Scoring.Weights.Tank.Mechanics = 30
	c.Scoring.Weights.Tank.Utility = 30
	n = score.Score([]model.Fight{lone}, nil, nil, c)[0]
	if n.Coverage != 15 || !n.Tenths.Present || n.Rank.Present {
		t.Fatalf("at .15 %+v", n)
	}
	c = config(t)
	c.Scoring.Confidence.MinCoverage = .535
	ns = score.Score([]model.Fight{f}, nil, reports, c)
	if !playerScore(t, ns, f.Players[0]).Rank.Present {
		t.Fatal("normal boundary excluded")
	}
}
func TestNightWeightsAndConfidence(t *testing.T) {
	c := config(t)
	c.Scoring.Weights.Tank = c.Scoring.Weights.DPS
	c.Scoring.Weights.Tank.Output = 0
	c.Scoring.Weights.Tank.Activity = 0
	c.Scoring.Weights.Tank.Mechanics = 100
	c.Scoring.Weights.Tank.Utility = 0
	c.Scoring.Weights.Tank.Preparation = 0
	kill := fixture("tank")
	a := kill.Players[0]
	kill.ID = "kill"
	wipe := fixture("tank")
	wipe.ID = "wipe"
	wipe.Success = model.Some(0)
	wipe.DurationMs = model.Some(44999)
	ds := []model.Death{mt.DeathOf(a, wipe.ID, wipe.StartMs, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build()}
	ns := score.Score([]model.Fight{kill, wipe}, ds, nil, c)
	n := ns[0]
	// 100 at weight 1, 60 at weight 0.5*0.5; both have coverage .5.
	if n.Tenths != model.Some(920) || n.Coverage != 50 || !slices.Contains(n.Fights[1].Flags, "lowConfidenceFight") {
		t.Fatalf("weighted %+v", n)
	}
	// Local span is 60s in both: the low-confidence flag must use server duration.
	wipe.DurationMs = model.Some(45000)
	n = score.Score([]model.Fight{kill, wipe}, ds, nil, c)[0]
	if n.Tenths != model.Some(867) || slices.Contains(n.Fights[1].Flags, "lowConfidenceFight") {
		t.Fatalf("duration boundary %+v", n)
	}
	trash := fixture("tank")
	trash.ID = "trash"
	trash.Boss = false
	ds = []model.Death{mt.DeathOf(a, trash.ID, trash.StartMs, model.CauseAvoidable).Owner(model.OwnerPlayer, a.Name).Build()}
	n = score.Score([]model.Fight{kill, trash, trash}, ds, nil, c)[0]
	if n.Tenths != model.Some(980) {
		t.Fatalf("trash cap %+v", n)
	}
}
func TestFractionalCoverageThreshold(t *testing.T) {
	c := config(t)
	c.Scoring.Weights.Tank.Mechanics = 30
	c.Scoring.Weights.Tank.Utility = 30
	c.Scoring.Confidence.ProvisionalMinCoverage = .151
	n := score.Score([]model.Fight{fixture("tank")}, nil, nil, c)[0]
	if n.Tenths.Present || n.Fights[0].Tenths.Present || n.Coverage != 15 {
		t.Fatal("fight and night thresholds diverged")
	}
}
func TestGatesPurityAndDungeon(t *testing.T) {
	c := config(t)
	f := fixture("dps", "dps", "dps")
	f.InstanceID = 329
	f.GroupSize = 5
	before, _ := json.Marshal(f)
	ns := score.Score([]model.Fight{f}, nil, nil, c)
	after, _ := json.Marshal(f)
	for _, n := range ns {
		if n.Rank.Present || !n.Tenths.Present {
			t.Fatal("dungeon rank or missing score")
		}
	}
	if string(before) != string(after) || !reflect.DeepEqual(ns, score.Score([]model.Fight{f}, nil, nil, c)) {
		t.Fatal("impure/non-deterministic")
	}
	delete(f.Roles, f.Players[0].ID)
	n := playerScore(t, score.Score([]model.Fight{f}, nil, nil, c), f.Players[0])
	if n.Tenths.Present || len(n.Fights) != 0 {
		t.Fatal("missing role scored")
	}
	f.Events = nil
	for _, n := range score.Score([]model.Fight{f}, nil, nil, c) {
		if n.Tenths.Present || n.Coverage != 0 || len(n.Fights) != 0 {
			t.Fatal("index scored or emitted an unselected fight")
		}
	}
	f = fixture("dps", "dps")
	f.Partial = true
	for _, n := range score.Score([]model.Fight{f}, nil, nil, c) {
		if n.Tenths.Present || len(n.Fights) != 1 || !slices.Contains(n.Fights[0].Flags, "partialFight") {
			t.Fatal("partial scored or hidden")
		}
	}
	f.Partial = false
	f.InstanceID = 99
	f.GroupSize = 5
	for _, n := range score.Score([]model.Fight{f}, nil, nil, c) {
		if n.Tenths.Present {
			t.Fatal("unknown run scored")
		}
	}
}
