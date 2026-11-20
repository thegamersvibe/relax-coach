package death

import (
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

// Test the rule's defensive holder-time boundary below classify's chronology
// guard. Otherwise that independent guard masks deletion of e.TimeMs > at.
func TestLostBossDoesNotSkipInvalidHolder(t *testing.T) {
	victim := modeltest.Player("Synthetic-victim-PvP-EU")
	tank := modeltest.Player("Synthetic-tank-PvP-EU")
	other := modeltest.Player("Synthetic-other-PvP-EU")
	boss := modeltest.Creature("Synthetic boss", 71)
	gap := int64(4000)
	f := model.Fight{Players: []model.Actor{victim, tank, other}, BossUnits: []string{boss.ID}, Roles: map[string]string{victim.ID: "dps", tank.ID: "tank", other.ID: "dps"}, Events: []model.Event{
		modeltest.At(10001).Src(boss).Dst(tank).Damage(1, -1).Landed().Build(),
		modeltest.At(9999).Src(boss).Dst(other).Damage(1, -1).Landed().Build(),
		modeltest.At(20000).Src(boss).Dst(victim).Damage(1, -1).Landed().Build(),
		modeltest.At(20001).Dst(victim).Died(0).Build(),
	}}
	v := newView(f)
	c := context{view: v, victim: victim, at: 20001, end: 3, params: cfg.Death{WindowMs: 9999, LostBossGapMs: &gap}}
	c.start, c.window = v.deathWindow(victim.ID, 10002, c.end)
	got := lostBoss(&c)
	if got.cause != Unresolved || got.owner != model.OwnerUndecided || len(got.owners) != 0 {
		t.Fatalf("skipped actual last holder for an earlier tank: %+v", got)
	}
}

func TestLostBossRejectsFutureStampedHolder(t *testing.T) {
	victim := modeltest.Player("Synthetic-victim-PvP-EU")
	tank := modeltest.Player("Synthetic-tank-PvP-EU")
	boss := modeltest.Creature("Synthetic boss", 71)
	gap := int64(4000)
	for _, at := range []int64{9999, 10000, 10001} {
		f := model.Fight{Players: []model.Actor{victim, tank}, BossUnits: []string{boss.ID}, Roles: map[string]string{victim.ID: "dps", tank.ID: "tank"}, Events: []model.Event{
			modeltest.At(at).Src(boss).Dst(tank).Damage(1, -1).Landed().Build(),
			modeltest.At(10000).Src(boss).Dst(victim).Damage(1, -1).Landed().Build(),
			modeltest.At(20000).Dst(victim).Died(0).Build(),
		}}
		v := newView(f)
		c := context{view: v, victim: victim, at: 20000, end: 2, params: cfg.Death{WindowMs: 10000, LostBossGapMs: &gap}}
		c.start, c.window = v.deathWindow(victim.ID, 10000, c.end)
		got := lostBoss(&c)
		if at > 10000 {
			if got.cause != Unresolved || got.owner != model.OwnerUndecided || len(got.owners) != 0 || got.low {
				t.Fatalf("future-stamped holder was charged: %+v", got)
			}
		} else if got.cause != TankLostBoss || got.owner != model.OwnerPlayer || len(got.owners) != 1 || got.owners[0] != tank.Name {
			t.Fatalf("valid preceding/equal-time holder control: %+v", got)
		}
	}
}
