package fights

import (
	"fmt"
	"math"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func tankFight(counts []int) ([]model.Event, []model.Actor) {
	start, end := modeltest.Encounter(model.Encounter{ID: 7, Name: giant.Name, InstanceID: model.Some(43), Difficulty: 2, GroupSize: int64(len(counts)), Success: model.Some(1), FightMs: model.Some(50000)}, 1000, 51000)
	es := []model.Event{start}
	var players []model.Actor
	for i := range counts {
		a := modeltest.Player(fmt.Sprintf("Invented%02d-Willow-PvP-EU", i))
		players = append(players, a)
		es = append(es, modeltest.At(int64(1001+i)).Info(model.Combatant{PlayerID: a.ID}).Build())
	}
	hp := modeltest.At(1100).Src(players[0]).Dst(giant).Damage(1, -1).Build()
	hp.InfoUnit, hp.HPMax = giant.ID, model.Some(900)
	es = append(es, hp)
	at := int64(1200)
	for i, count := range counts {
		for j := 0; j < count; j++ {
			es = append(es, modeltest.At(at).Src(giant).Dst(players[i]).Damage(10, -1).Landed().Build())
			at++
		}
	}
	// A different distribution of source-side swings, misses and add swings
	// must not change either the boss-landed denominator or its top player.
	add := modeltest.Creature("Invented Small Attendant", 82)
	for i := 0; i < 100; i++ {
		e := modeltest.At(at).Src(giant).Dst(players[len(players)-1]).Damage(10, -1).Build()
		e.Stream = model.SwingSource
		es = append(es, e, modeltest.At(at+1).Src(giant).Dst(players[len(players)-1]).Miss("DODGE").Build(),
			modeltest.At(at+2).Src(add).Dst(players[len(players)-1]).Damage(10, -1).Landed().Build())
		at += 3
	}
	return append(es, end), players
}

func TestTankLandedShareAndTopRatio(t *testing.T) {
	for _, tc := range []struct {
		name   string
		counts []int
		tanks  []bool
	}{
		{"ninety-percent-and-five-percent-melee", []int{90, 5, 5}, []bool{true, false, false}},
		{"four-equal-tanks", []int{25, 25, 25, 25}, []bool{true, true, true, true}},
		{"share-alone-is-not-enough", []int{70, 30}, []bool{true, false}},
		{"exact-half-of-top", []int{40, 20, 20, 20}, []bool{true, true, true, true}},
		{"exact-minimum-share", []int{30, 15, 15, 15, 15, 10}, []bool{true, true, true, true, true, false}},
		{"no-landed-swings", []int{0, 0}, []bool{false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			es, players := tankFight(tc.counts)
			f := only(t, build(t, es, params())).Fights[0]
			for i, a := range players {
				want := "" // No output evidence: a non-tank is not automatically DPS.
				if i == 0 && tc.counts[0] > 0 {
					want = "dps"
				}
				if tc.tanks[i] {
					want = "tank"
				}
				if f.Roles[a.ID] != want {
					t.Errorf("player %d count=%d: role=%s want=%s", i, tc.counts[i], f.Roles[a.ID], want)
				}
			}
		})
	}
}

func TestTankTopRatioConfigAndValidation(t *testing.T) {
	es, players := tankFight([]int{40, 20, 20, 20})
	p := params()
	p.TankMinTopRatio = .51
	f := only(t, build(t, es, p)).Fights[0]
	for i, a := range players {
		if (f.Roles[a.ID] == "tank") != (i == 0) {
			t.Errorf("configured top ratio ignored for player %d", i)
		}
	}
	for _, ratio := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1), 1.01} {
		p = params()
		p.TankMinTopRatio = ratio
		if _, err := Build(nil, p); err == nil {
			t.Errorf("invalid top ratio %v accepted", ratio)
		}
	}
}

func TestTankTopTakerIsPlayerNotPet(t *testing.T) {
	es, players := tankFight([]int{20, 10})
	end := es[len(es)-1]
	es = es[:len(es)-1]
	pet := modeltest.Pet("Invented Protector", players[1])
	for i := 0; i < 70; i++ {
		es = append(es, modeltest.At(int64(3000+i)).Src(giant).Dst(pet).Damage(10, -1).Landed().Build())
	}
	f := only(t, build(t, append(es, end), params())).Fights[0]
	if f.Roles[players[0].ID] != "tank" || f.Roles[players[1].ID] != "" {
		t.Fatal("pet swings must count in the total, not as a player or the top player")
	}
}

func TestTankShareMustBePositive(t *testing.T) {
	for _, share := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1), 1.01} {
		p := params()
		p.TankMinBossSwingShare = share
		if _, err := Build(nil, p); err == nil {
			t.Errorf("invalid tank share %v accepted", share)
		}
	}
}
