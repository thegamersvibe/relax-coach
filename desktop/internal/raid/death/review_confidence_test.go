package death_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
)

func TestZZConfidenceRuling(t *testing.T) {
	t.Run("after_wipe_also_avoidable", func(t *testing.T) {
		f := quickDeath()
		f.Roles[tankB.ID] = "tank"
		f.Players = append(f.Players, tankB)
		f.Events = append(f.Events, died(1000, tank), died(1001, tankB))
		c := config()
		c.Scoring.Death.WipeAllTanksDead = true
		mechanic(&c, 81)
		// PHASE1-BRIEF R6 item 2: the first cause wins, but a second actual
		// rule match makes confidence medium; no wipe exception is stated.
		wantDeath(t, forChronologicalVictim(t, f, c), death.AfterWipeCall, "medium", model.OwnerNobody, "")
	})
	t.Run("avoidable_ambiguous_aggro_is_not_second_match", func(t *testing.T) {
		f := reviewMelee(9000, 9001)
		f.Events = append(f.Events, hit(9500, boss, victim, 81, 100))
		c := config()
		mechanic(&c, 81)
		wantDeath(t, forChronologicalVictim(t, f, c), death.Avoidable, "high", model.OwnerPlayer, "", victim.Name)
	})
}

func TestRecapBossKeepsRecentTen(t *testing.T) {
	// Ambiguous aggro uses the same most-recent-ten bound as ordinary recaps.
	f := reviewFight(hit(5000, tank, boss, 91, 1), hit(5001, victim, boss, 92, 1))
	want := []int64{}
	for i := int64(0); i < 12; i++ {
		at := 6000 + i*200
		want = append(want, at)
		f.Events = append(f.Events, swing(at, boss, victim, 1))
	}
	f.Events = append(f.Events, died(10000, victim))
	d := forChronologicalVictim(t, f, config())
	wantDeath(t, d, death.Unresolved, "low", model.OwnerUndecided, "")
	want = want[2:]
	if !reflect.DeepEqual(d.Evidence, want) {
		t.Fatalf("evidence=%v want most recent ten=%v", d.Evidence, want)
	}
}
