package fights

import (
	"io"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

// Boss swings that land only on non-players (a pet, another NPC or an empty
// target) are no tank evidence: no player may receive any role.
func TestBossRolesRequirePlayerSwingEvidence(t *testing.T) {
	wolf := modeltest.Pet("Invented wolf", alia)
	lateWolf := modeltest.Pet("Invented late wolf", alia)
	lateWolf.OwnerID = ""
	add := modeltest.Creature("Invented Pebble", 82)
	for _, tc := range []struct {
		name      string
		dst       model.Actor
		lateOwner bool
		wantTank  bool
	}{
		{"pet-only", wolf, false, false},
		{"pet-late-owner", lateWolf, true, false},
		{"npc-only", add, false, false},
		{"empty-target", model.Actor{}, false, false},
		{"real-tank", alia, false, true},
	} {
		for _, mode := range []string{"Build", "Read"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				es := boss(7, 1000)
				es[3].InfoUnit, es[3].HPMax = giant.ID, model.Some(900)
				swing := modeltest.At(1010).Src(giant).Dst(tc.dst).Damage(10, -1).Landed().Build()
				es = append(es[:4], swing,
					modeltest.At(1012).Src(bram).Dst(alia).Heal(30, 0).Build(), es[4])
				if tc.lateOwner {
					es = append(es, model.Event{TimeMs: 60000, InfoUnit: lateWolf.ID, InfoOwnerID: alia.ID})
				}
				var run Run
				if mode == "Build" {
					run = only(t, build(t, es, params()))
				} else {
					r, err := Read(strings.NewReader("synthetic"), func(_ io.Reader, emit func(model.Event)) error {
						for _, e := range es {
							emit(e)
						}
						return nil
					}, params())
					if err != nil {
						t.Fatal(err)
					}
					run = only(t, r)
				}
				f, d := run.Fights[0], run.Details[0]
				if len(f.BossUnits) == 0 || d.Timelines[alia.ID].DamageDone != 100 || d.Timelines[bram.ID].HealingDone != 30 || !d.RolesComputed {
					t.Fatal("fixture lost boss selection, positive output or role evaluation")
				}
				if !tc.wantTank {
					if len(f.Roles) != 0 {
						t.Fatalf("roles=%v: boss swings on no player are not tank evidence", f.Roles)
					}
					return
				}
				if f.Roles[alia.ID] != "tank" || f.Roles[bram.ID] != "healer" {
					t.Fatalf("roles=%v: a player taking boss swings must stay tank", f.Roles)
				}
			})
		}
	}
}
