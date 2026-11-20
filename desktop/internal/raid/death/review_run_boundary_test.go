package death_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func clockSpikeRun(t *testing.T, runEnd int64) fights.Run {
	t.Helper()
	c, err := cfg.Load(filepath.Join("..", "..", "..", "..", "rules", "raid"))
	if err != nil {
		t.Fatal(err)
	}
	p := c.Scoring.Fights
	params := fights.Params{PullGapMs: p.PullGapMs, TailMs: 100, ReentryGapMs: p.ReentryGapMs,
		LowConfidenceFightMs:  c.Scoring.Confidence.LowConfidenceFightMs,
		TankMinBossSwingShare: p.TankMinBossSwingShare, TankMinTopRatio: p.TankMinTopRatio,
		HealerRequiresNotTank: p.HealerRequiresNotTank, HealerMinHealingDamageRatio: p.HealerMinHealingDamageRatio,
		HealerStrictlyGreater: p.HealerStrictlyGreater, BossUnitSelection: p.BossUnitSelection,
		InstanceKind: c.InstanceKind, DetailsFor: func(model.Fight) bool { return true }}
	start, end := modeltest.Encounter(model.Encounter{ID: 71, InstanceID: model.Some(42), GroupSize: 10, FightMs: model.Some(1000), Success: model.Some(1)}, 1000, 2000)
	es := []model.Event{start, hit(1100, other, boss, 91, 1), hit(1200, victim, boss, 91, 1),
		died(50000, victim), end, died(2050, other), {TimeMs: runEnd, Kind: model.ZoneChange, Zone: &model.Zone{InstanceID: 0}}}
	r, err := fights.Build(es, params)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Runs) != 1 || len(r.Runs[0].Fights) != 1 {
		t.Fatalf("runs/fights: %+v", r)
	}
	run := r.Runs[0]
	if run.EndMs != runEnd || run.Fights[0].EndMs != 2000 {
		t.Fatalf("bounds: run=%d fight=%d", run.EndMs, run.Fights[0].EndMs)
	}
	return run
}

func TestRunEndClockSpikePendingContract(t *testing.T) {
	// The requested exclusion was first run RED: it returned both deaths,
	// not only the valid tail at 2050. Do not silently substitute Fight.EndMs:
	// that is 2000 and would wrongly drop the valid tail too.
	short, long := clockSpikeRun(t, 2100), clockSpikeRun(t, 60000)
	if !reflect.DeepEqual(short.Fights[0], long.Fights[0]) {
		t.Fatal("the same Fight input must demonstrate the missing run-boundary information")
	}
	ds, _ := death.Review(short.Fights[0], config())
	again, _ := death.Review(long.Fights[0], config())
	if !reflect.DeepEqual(ds, again) || len(ds) != 2 || ds[0].TimeMs != 2050 || ds[1].TimeMs != 50000 {
		t.Fatal("pending contract: current fight-only behavior changed")
	}
	inside := func(run fights.Run) int {
		n := 0
		for _, d := range ds {
			if d.TimeMs >= run.StartMs && d.TimeMs <= run.EndMs {
				n++
			}
		}
		return n
	}
	if inside(short) != 1 || inside(long) != 2 {
		t.Fatal("run-scope oracle did not distinguish the two boundaries")
	}
	t.Log("pending contract: identical Fight inputs require 1 versus 2 reviewed deaths; only fights.Run carries final EndMs")
}

func TestFightEndDoesNotExcludeValidTailDeath(t *testing.T) {
	f := fight(hit(1999, boss, victim, 81, 1), died(2050, victim))
	f.EndMs = 2000
	ds, _ := death.Review(f, config())
	if len(ds) != 1 || ds[0].TimeMs != 2050 {
		t.Fatal("valid post-encounter tail death was dropped")
	}
}
