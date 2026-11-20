package score_test

import (
	"encoding/json"
	"math/big"
	"reflect"
	"slices"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func TestNightRolePartsSumAndPurity(t *testing.T) {
	c := config(t)
	c.Scoring.Night.KillWeight = .6
	fights := []model.Fight{{ID: "one", Boss: true}, {ID: "two", Boss: true}, {ID: "three"}}
	n := model.NightScore{PlayerID: "a", Fights: []model.FightScore{nightPartFight("one", "dps", 901), nightPartFight("two", "dps", 700), nightPartFight("three", "tank", 500)}}
	before, _ := json.Marshal([]any{fights, n, c})
	want, err := score.NightRoleParts(fights, n, c)
	if err != nil || want.Role != "dps" || want.Parts["output"].Cmp(big.NewRat(1601, 20)) != 0 {
		t.Fatalf("sum %+v %v", want, err)
	}
	after, _ := json.Marshal([]any{fights, n, c})
	if string(before) != string(after) {
		t.Fatal("inputs mutated")
	}
	slices.Reverse(n.Fights)
	slices.Reverse(fights)
	got, err := score.NightRoleParts(fights, n, c)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("input order changed summary")
	}
	got.Parts["output"].SetInt64(0)
	again, err := score.NightRoleParts(fights, n, c)
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatal("returned values alias subsequent results")
	}
}
