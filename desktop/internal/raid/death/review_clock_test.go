package death_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

var (
	secV     = modeltest.Player("Vela-Fern-PvP-EU")
	secA     = modeltest.Player("Aster-Fern-PvP-EU")
	secB     = modeltest.Player("Beryl-Fern-PvP-EU")
	secH     = modeltest.Player("Hedra-Fern-PvP-EU")
	secO     = modeltest.Player("Orin-Fern-PvP-EU")
	secBoss  = modeltest.Creature("Synthetic second-review boss", 771)
	secBoss2 = modeltest.Creature("Synthetic second-review boss two", 772)
)

func secHit(at int64, src, dst model.Actor) model.Event {
	return modeltest.At(at).Src(src).Dst(dst).Spell(883, "Synthetic probe ability").Damage(1, -1).Build()
}
func secSwing(at int64, dst model.Actor) model.Event {
	return modeltest.At(at).Src(secBoss).Dst(dst).Damage(1, -1).Landed().Build()
}
func secMiss(at int64, src, dst model.Actor) model.Event {
	return modeltest.At(at).Src(src).Dst(dst).Miss("PARRY").Build()
}
func secDied(at int64, dst model.Actor) model.Event { return modeltest.At(at).Dst(dst).Died(0).Build() }
func secFight(es ...model.Event) model.Fight {
	for i := range es {
		es[i].Line = int64(i + 1)
		es[i].Offset = int64((i + 1) * 100)
		es[i].Len = 100
	}
	f := modeltest.NewFight("second-review", model.Encounter{ID: 771, Success: model.Some(1), FightMs: model.Some(60000)}, es...).
		Players(secV, secA, secB, secH, secO).BossUnits(secBoss, secBoss2).
		Role(secV, "dps").Role(secA, "tank").Role(secB, "tank").Role(secH, "healer").Role(secO, "dps").Build()
	f.StartMs, f.EndMs = 0, 90000
	return f
}
func secConfigs(t *testing.T) []struct {
	name string
	c    cfg.Config
} {
	t.Helper()
	loaded, err := cfg.Load(filepath.Join("..", "..", "..", "..", "rules", "raid"))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Scoring.Version != "1.5.0" || loaded.Scoring.Death.PulledAggroMs != 10000 {
		t.Fatal("wrong shipped cfg")
	}
	gap, summary := int64(4000), int64(10000)
	c := cfg.Config{Scoring: cfg.Scoring{Death: cfg.Death{FullHealthFraction: .9, MinHealthSamples: 2, RaidwideShare: .6, RaidwideWindowMs: 2000, WindowMs: 10000, PulledAggroMs: 10000, WipeDeadShare: 1, OneShotWindowMs: 1500, UnhealedMinMs: 4000, LostBossGapMs: &gap, SummaryWindowMs: &summary}}}
	return []struct {
		name string
		c    cfg.Config
	}{{"shipped", *loaded}, {"test", c}}
}
func secVictim(t *testing.T, f model.Fight, c cfg.Config) model.Death {
	t.Helper()
	ds, _, _ := review(t, f, 100000, c)
	for _, d := range ds {
		if d.Player.ID == secV.ID {
			return d
		}
	}
	t.Fatal("missing victim")
	return model.Death{}
}
func secWant(t *testing.T, got model.Death, cause string, owners ...string) {
	t.Helper()
	kind := model.OwnerUndecided
	if len(owners) > 0 {
		kind = model.OwnerPlayer
	}
	if cause == model.CauseAfterWipe || cause == model.CauseUnavoidable {
		kind = model.OwnerNobody
	}
	t.Logf("cause=%q confidence=%q kind=%q owners=%q suggestion=%q evidence=%v", got.Cause, got.Confidence, got.OwnerKind, got.Owners, got.SuggestedCause, got.Evidence)
	if cause == model.CauseUnresolved && got.Confidence != "low" {
		t.Fatal("unresolved must be low")
	}
	if got.Cause != cause || got.OwnerKind != kind || got.SuggestedCause != "" || len(got.Owners) != len(owners) {
		t.Fatalf("want cause=%q kind=%q owners=%q", cause, kind, owners)
	}
	for i := range owners {
		if owners[i] != got.Owners[i] {
			t.Fatalf("wrong owner: want=%q", owners)
		}
	}
}

// Later log lines must not manufacture evidence for an earlier death. All
// inputs have explicit increasing line/byte identities despite clock rollback.
func TestSecondPostDeathClockRollback(t *testing.T) {
	for _, cc := range secConfigs(t) {
		t.Run(cc.name, func(t *testing.T) {
			f := secFight(secSwing(10000, secV), secDied(10000, secV), model.Event{TimeMs: 9998, Kind: model.Resurrect, Dst: secV}, secHit(9999, secV, secBoss))
			secWant(t, secVictim(t, f, cc.c), model.CauseUnresolved)
		})
	}
}

// The last actual boss target before the run is a non-tank. A slightly
// earlier timestamp on that later line must not change holder identity.
func TestSecondHolderLogOrderClockRollback(t *testing.T) {
	for _, cc := range secConfigs(t) {
		t.Run(cc.name, func(t *testing.T) {
			f := secFight(secHit(1000, secA, secBoss), secHit(1001, secB, secBoss), secSwing(9000, secB), secSwing(8999, secO), secSwing(10000, secV), secDied(10001, secV))
			secWant(t, secVictim(t, f, cc.c), model.CauseUnresolved)
		})
	}
}

func TestSecondHolderRollbackThroughFights(t *testing.T) {
	c := secConfigs(t)[0].c
	p := c.Scoring.Fights
	params := fights.Params{PullGapMs: p.PullGapMs, TailMs: p.TailMs, ReentryGapMs: p.ReentryGapMs, LowConfidenceFightMs: c.Scoring.Confidence.LowConfidenceFightMs, TankMinBossSwingShare: p.TankMinBossSwingShare, TankMinTopRatio: p.TankMinTopRatio, HealerRequiresNotTank: p.HealerRequiresNotTank, HealerMinHealingDamageRatio: p.HealerMinHealingDamageRatio, HealerStrictlyGreater: p.HealerStrictlyGreater, BossUnitSelection: p.BossUnitSelection, InstanceKind: c.InstanceKind, DetailsFor: func(model.Fight) bool { return true }}
	for _, rollback := range []bool{false, true} {
		t.Run(fmt.Sprintf("rollback_%v", rollback), func(t *testing.T) {
			start, end := modeltest.Encounter(model.Encounter{ID: 771, InstanceID: model.Some(42), GroupSize: 10, Success: model.Some(1), FightMs: model.Some(20000)}, 0, 20000)
			h := secHit(1000, secA, secBoss)
			h.InfoUnit = secBoss.ID
			h.HPMax = model.Some(10000)
			es := []model.Event{start, h, secHit(1001, secB, secBoss), secHit(2000, secV, secBoss), secHit(2001, secO, secBoss), modeltest.At(2100).Src(secH).Dst(secA).Heal(2, 0).Build()}
			for i := 0; i < 20; i++ {
				dst := secA
				if i%2 != 0 {
					dst = secB
				}
				es = append(es, secSwing(3000+int64(i)*100, dst))
			}
			at := int64(9001)
			if rollback {
				at = 8999
			}
			es = append(es, secSwing(9000, secB), secSwing(at, secO), secSwing(10000, secV), secDied(10001, secV), end)
			for i := range es {
				es[i].Line = int64(i + 1)
				es[i].Offset = int64((i + 1) * 100)
				es[i].Len = 100
			}
			out, err := fights.Build(es, params)
			if err != nil {
				t.Fatal(err)
			}
			if len(out.Runs) != 1 || len(out.Runs[0].Fights) != 1 {
				t.Fatalf("runs=%d", len(out.Runs))
			}
			f := out.Runs[0].Fights[0]
			if f.Roles[secA.ID] != "tank" || f.Roles[secB.ID] != "tank" || f.Roles[secV.ID] != "dps" || f.Roles[secO.ID] != "dps" {
				t.Fatalf("fixture roles=%v", f.Roles)
			}
			if len(f.BossUnits) != 1 || f.BossUnits[0] != secBoss.ID {
				t.Fatalf("fixture bosses=%v", f.BossUnits)
			}
			kept := false
			for i, e := range f.Events {
				if e.TimeMs == 9000 && e.Dst.ID == secB.ID && i+1 < len(f.Events) {
					next := f.Events[i+1]
					kept = next.TimeMs == at && next.Dst.ID == secO.ID
				}
			}
			if !kept {
				t.Fatal("producer did not retain the log-order pair")
			}
			t.Logf("producer retained B@9000 then non-tank@%d; inferred roles A=tank B=tank V=dps other=dps", at)
			secWant(t, secVictim(t, f, c), model.CauseUnresolved)
		})
	}
}
