package fights

import (
	"fmt"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func TestUnlistedInstanceGroupSize(t *testing.T) {
	for _, size := range []int64{1, 5, 6, 10, 20, 40} {
		t.Run(fmt.Sprintf("players-%d", size), func(t *testing.T) {
			// Only two players are observed: classification must use the encounter
			// group size, not the observed roster, and retain the unlisted id.
			es := instanceBoss(7, 1000, 99, size)
			r := only(t, build(t, es, params()))
			wantKind, wantReason := model.RunUnknown, "unknownInstance"
			if size > 5 {
				wantKind, wantReason = model.RunRaid, ""
			}
			if r.Kind != wantKind || r.Listed || r.InstanceID != 99 || r.Fights[0].InstanceID != 99 || r.Fights[0].GroupSize != int(size) {
				t.Fatalf("unlisted classification: kind=%v listed=%v instance=%d", r.Kind, r.Listed, r.InstanceID)
			}
			if r.Details[0].UnscoredReason != wantReason {
				t.Fatalf("unscored reason=%q want=%q", r.Details[0].UnscoredReason, wantReason)
			}
			if _, ok := r.RaidNight(); ok != (size > 5) {
				t.Fatal("unlisted raid admission did not follow encounter group size")
			}
		})
	}
}

func TestFortyPlayersAndPetsRetained(t *testing.T) {
	var players, pets []model.Actor
	for i := 0; i < 40; i++ {
		a := modeltest.Player(fmt.Sprintf("Invented%02d-Meadow-PvP-EU", i))
		p := modeltest.Pet("Invented Companion", a)
		p.OwnerID = "" // Summon evidence below supplies ownership, after the damage.
		players, pets = append(players, a), append(pets, p)
	}
	start, end := modeltest.Encounter(model.Encounter{ID: 7, Name: giant.Name, InstanceID: model.Some(99), Difficulty: 2, GroupSize: 40, Success: model.Some(1), FightMs: model.Some(50000)}, 1000, 51000)
	es := []model.Event{start}
	for i, a := range players {
		at := int64(1001 + i*4)
		es = append(es, modeltest.At(at).Info(model.Combatant{PlayerID: a.ID}).Build(),
			modeltest.At(at+1).Src(a).Dst(giant).Damage(1, -1).Build(),
			modeltest.At(at+2).Src(pets[i]).Dst(giant).Damage(2, -1).Build())
	}
	for i, a := range players {
		es = append(es, model.Event{Kind: model.Summon, TimeMs: int64(1300 + i), Src: a, Dst: pets[i]})
	}
	es = append(es, model.Event{TimeMs: 1400, InfoUnit: giant.ID, HP: model.Some(800), HPMax: model.Some(900)},
		modeltest.At(1401).Src(giant).Dst(players[0]).Damage(10, -1).Landed().Build(),
		modeltest.At(1402).Src(players[1]).Dst(players[0]).Heal(20, 0).Build(), end)
	r := only(t, build(t, es, params()))
	f := r.Fights[0]
	if r.Kind != model.RunRaid || len(r.Roster) != 40 || len(f.Players) != 40 || len(f.Combatants) != 40 || len(f.Roles) != 40 || len(f.Owners) != 40 || len(f.Unowned) != 0 || len(f.Events) != len(es) || len(r.Details[0].Timelines) != 40 {
		t.Fatalf("loss: roster=%d players=%d combatants=%d roles=%d pets=%d unowned=%d events=%d/%d timelines=%d", len(r.Roster), len(f.Players), len(f.Combatants), len(f.Roles), len(f.Owners), len(f.Unowned), len(f.Events), len(es), len(r.Details[0].Timelines))
	}
	seen := map[string]bool{}
	for _, a := range f.Players {
		seen[a.ID] = true
	}
	for i, a := range players {
		wantRole := "dps"
		if i == 0 {
			wantRole = "tank"
		} else if i == 1 {
			wantRole = "healer"
		}
		if !seen[a.ID] || f.Roles[a.ID] != wantRole || f.Owners[pets[i].ID] != a.ID {
			t.Fatalf("player %d lost, role incorrect, or pet not owned", i)
		}
		if tl := r.Details[0].Timelines[a.ID]; tl == nil || tl.DamageDone != 3 {
			t.Fatalf("player %d lost owned pet damage", i)
		}
	}
}

func TestBossOrderDoesNotChangeNightIdentity(t *testing.T) {
	var reference Run
	for i, order := range [][]int64{{9, 3, 7}, {7, 9, 3}} {
		var es []model.Event
		for j, id := range order {
			es = append(es, instanceBoss(id, int64(j+1)*100000, 99, 20)...)
		}
		r := only(t, build(t, es, params()))
		if len(r.Fights) != len(order) {
			t.Fatal("branching encounter order lost fights")
		}
		for j, id := range order {
			if r.Fights[j].EncounterID != id {
				t.Fatal("fight order did not preserve file order")
			}
		}
		if i == 0 {
			reference = r
		} else if r.NightID != reference.NightID {
			t.Fatal("boss order changed sorted night identity")
		}
	}
}
