package score_test

import (
	"math/big"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

func nightPartFight(id, role string, tenths int64) model.FightScore {
	return model.FightScore{FightID: id, PlayerID: "a", Role: role, Tenths: model.Some(700), Parts: []model.Part{{Name: "output", Tenths: model.Some(tenths)}}}
}
func TestNightRolePartsWeightAndSplit(t *testing.T) {
	c := config(t)
	c.Scoring.Night.KillWeight = 2
	c.Scoring.Night.WipeWeight = .5
	c.Scoring.Confidence.ShortFightWeight = .25
	fights := []model.Fight{{ID: "kill", Boss: true, Success: model.Some(1)}, {ID: "wipe", Boss: true, Success: model.Some(0)}, {ID: "trash"}, {ID: "unscored", Boss: true, Success: model.Some(1)}}
	n := model.NightScore{PlayerID: "a", Fights: []model.FightScore{nightPartFight("kill", "tank", 700), nightPartFight("wipe", "healer", 400), nightPartFight("trash", "dps", 800), nightPartFight("unscored", "dps", 800)}}
	n.Fights[0].Flags = []string{"lowConfidenceFight"}
	n.Fights[3].Tenths = model.Opt{}
	got, err := score.NightRoleParts(fights, n, c)
	if err != nil || got.Role != "dps" || got.Split {
		t.Fatalf("fight weights %+v %v", got, err)
	}
	n.Fights[0].Flags = nil
	got, err = score.NightRoleParts(fights, n, c)
	if err != nil || got.Role != "tank" || got.Split {
		t.Fatalf("kill weight %+v %v", got, err)
	}
	n.Fights[0].Flags = []string{"lowConfidenceFight"}
	n.Fights[2].Tenths = model.Opt{}
	got, err = score.NightRoleParts(fights, n, c)
	if err != nil || got.Role != "" || !got.Split || len(got.Parts) != 0 {
		t.Fatalf("equal roles %+v %v", got, err)
	}
	for i := range n.Fights {
		n.Fights[i].Tenths = model.Opt{}
	}
	got, err = score.NightRoleParts(fights, n, c)
	if err != nil || got.Role != "" || got.Split || len(got.Parts) != 0 {
		t.Fatalf("no scored fights %+v %v", got, err)
	}
}

func TestNightRolePartsGroups(t *testing.T) {
	c := config(t)
	c.Scoring.Night.KillWeight = 2
	c.Scoring.Night.WipeWeight = .5
	c.Scoring.Confidence.ShortFightWeight = .25
	c.Scoring.Night.TrashShare = .2
	c.Scoring.Night.MaxTrashShare = .1
	c.Scoring.Night.MinBossShare = .95
	fights := []model.Fight{{ID: "kill", Boss: true, Success: model.Some(1)}, {ID: "wipe", Boss: true, Success: model.Some(0)}, {ID: "trash"}, {ID: "missing", Boss: true}, {ID: "offrole"}}
	n := model.NightScore{PlayerID: "a", Fights: []model.FightScore{nightPartFight("kill", "dps", 900), nightPartFight("wipe", "dps", 300), nightPartFight("trash", "dps", 500), nightPartFight("missing", "dps", 0), nightPartFight("offrole", "tank", 0)}}
	n.Fights[1].Flags = []string{"lowConfidenceFight"}
	n.Fights[2].Coverage = 1 // part means do not acquire an extra coverage weight
	n.Fights[3].Parts[0].Tenths = model.Opt{}
	n.Fights[0].Parts = append(n.Fights[0].Parts, model.Part{Name: "mechanics", Tenths: model.Some(641)}, model.Part{Name: "utility"})
	n.Fights[2].Parts = append(n.Fights[2].Parts, model.Part{Name: "preparation", Tenths: model.Some(777)})
	got, err := score.NightRoleParts(fights, n, c)
	// Boss (90*2 + 30*.125)/2.125 = 1470/17. Join .95 boss + .05*50 = 1439/17.
	if err != nil || got.Role != "dps" || got.Parts["output"].Cmp(big.NewRat(1439, 17)) != 0 || len(got.Parts) != 3 || got.Parts["mechanics"].Cmp(big.NewRat(641, 10)) != 0 || got.Parts["preparation"].Cmp(big.NewRat(777, 10)) != 0 {
		t.Fatalf("exact groups %+v error %v", got, err)
	}
	// Retain a measured part from an otherwise unscored fight in the night role.
	n.Fights[2].Tenths = model.Opt{}
	got, err = score.NightRoleParts(fights, n, c)
	if err != nil || got.Parts["output"].Cmp(big.NewRat(1439, 17)) != 0 {
		t.Fatalf("measured part discarded %+v %v", got, err)
	}
}

func TestNightRolePartsMissingMetadata(t *testing.T) {
	n := model.NightScore{PlayerID: "a", Fights: []model.FightScore{nightPartFight("missing", "dps", 700)}}
	got, err := score.NightRoleParts(nil, n, config(t))
	if err == nil || got.Role != "" || len(got.Parts) != 0 {
		t.Fatalf("missing metadata invented a group: %+v %v", got, err)
	}
}

func TestNightRolePartsSingle(t *testing.T) {
	n := model.NightScore{PlayerID: "a", Fights: []model.FightScore{nightPartFight("kill", "dps", 753)}}
	got, err := score.NightRoleParts([]model.Fight{{ID: "kill", Boss: true, Success: model.Some(1)}}, n, config(t))
	if err != nil || got.Role != "dps" || got.Split || len(got.Parts) != 1 || got.Parts["output"].Cmp(big.NewRat(753, 10)) != 0 {
		t.Fatalf("single summary %+v error %v", got, err)
	}
}
