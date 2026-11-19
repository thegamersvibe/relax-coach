package fights

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

var alia = modeltest.Player("Alia-Stone-PvP-EU")
var bram = modeltest.Player("Bram-Oak-Hill-PvP-EU")
var cato = modeltest.Player("Cato-PvP-EU")
var giant = modeltest.Creature("Invented Granite Keeper", 81)

func params() Params {
	return Params{PullGapMs: 10000, TailMs: 2500, LowConfidenceFightMs: 45000, ReentryGapMs: 1800000,
		TankMinBossSwingShare: .15, TankMinTopRatio: .5, HealerRequiresNotTank: true, HealerMinHealingDamageRatio: 1,
		HealerStrictlyGreater: true, BossUnitSelection: "highest_max_health_damaged_creature",
		DetailsFor: func(model.Fight) bool { return true }, // These synthetic fixtures request full detail.
		InstanceKind: func(id, size int64) (model.RunKind, bool) {
			if id == 42 {
				return model.RunDungeon, true
			}
			if id == 43 {
				return model.RunRaid, true
			}
			if size > 5 {
				return model.RunRaid, false
			}
			return model.RunUnknown, false
		}}
}
func boss(id, start int64) []model.Event {
	a, b := modeltest.Encounter(model.Encounter{ID: id, Name: giant.Name, InstanceID: model.Some(42), Difficulty: 2, GroupSize: 5, Success: model.Some(1), FightMs: model.Some(50000)}, start, start+50146)
	return []model.Event{a, modeltest.At(start + 1).Info(model.Combatant{PlayerID: bram.ID}).Build(), modeltest.At(start + 2).Info(model.Combatant{PlayerID: alia.ID}).Build(), modeltest.At(start+3).Src(alia).Dst(giant).Damage(100, -1).Build(), b}
}
func build(t *testing.T, es []model.Event, p Params) Result {
	t.Helper()
	r, err := Build(es, p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func only(t *testing.T, r Result) Run {
	t.Helper()
	if len(r.Runs) != 1 {
		t.Fatalf("runs=%d", len(r.Runs))
	}
	return r.Runs[0]
}
func TestBossIdentityServerFields(t *testing.T) {
	es := boss(7, 1000)
	run := only(t, build(t, es, params()))
	if len(run.Fights) != 1 {
		t.Fatalf("fights=%d", len(run.Fights))
	}
	f := run.Fights[0]
	ids := []string{alia.ID, bram.ID}
	sort.Strings(ids)
	expected := fmt.Sprintf("%x", sha256.Sum256([]byte("boss\n42\n7\n2\n5\n1\n50000\n"+strings.Join(ids, "\n"))))
	if f.ID != expected || !f.Boss || f.Partial || f.DurationMs != model.Some(50000) || f.Success != model.Some(1) {
		t.Fatalf("wrong encounter result: %+v", f)
	}
	if run.NightID != fmt.Sprintf("%x", sha256.Sum256([]byte(expected))) {
		t.Fatal("night identity")
	}
	shifted := append([]model.Event(nil), es...)
	for i := range shifted {
		shifted[i].TimeMs += 2000
		shifted[i].Offset += 90
	}
	shifted = append(shifted[:3], append([]model.Event{{TimeMs: 3002, Kind: model.CastFailed, Src: bram}}, shifted[3:]...)...)
	other := only(t, build(t, shifted, params()))
	if other.NightID != run.NightID || other.Fights[0].ID != f.ID {
		t.Fatal("logger differences changed identity")
	}
	if !reflect.DeepEqual(run, only(t, build(t, es, params()))) {
		t.Fatal("not deterministic")
	}
}
