package score_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestNightRolePartsConfigSnapshot(t *testing.T) {
	// Keep the Load result, not config(t)'s already-detached snapshot. Like
	// Score, the summary must use Load's validated data, not its editable view.
	c, err := cfg.Load("../../../../rules/raid")
	if err != nil {
		t.Fatal(err)
	}
	fights := []model.Fight{{ID: "kill", Boss: true, Success: model.Some(1)}}
	n := model.NightScore{PlayerID: "a", Fights: []model.FightScore{nightPartFight("kill", "dps", 900)}}
	want, err := score.NightRoleParts(fights, n, *c)
	if err != nil || want.Role != "dps" || want.Split || len(want.Parts) != 1 || want.Parts["output"].Cmp(big.NewRat(90, 1)) != 0 {
		t.Fatalf("snapshot fixture: %+v %v", want, err)
	}
	// After constructing the inputs, edit the caller's public config. Zeroing
	// the kill weight would remove the role if the summary skipped Snapshot.
	c.Scoring.Night.KillWeight = 0
	c.Scoring.RatioPoints = nil
	clear(c.Scoring.MeasureWeights)
	got, err := score.NightRoleParts(fights, n, *c)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("summary read the editable view instead of the validated snapshot: %+v %v", got, err)
	}
	// A detached snapshot is intentionally editable and has no hidden defaults.
	detached := c.Snapshot()
	detached.Scoring.Night.KillWeight = 0
	got, err = score.NightRoleParts(fights, n, detached)
	if err != nil || got.Role != "" || got.Split || len(got.Parts) != 0 {
		t.Fatalf("caller-constructed snapshot ignored: %+v %v", got, err)
	}
}
