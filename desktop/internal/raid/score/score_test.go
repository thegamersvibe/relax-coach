package score_test

import (
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
	"testing"
)

func config(t *testing.T) cfg.Config {
	t.Helper()
	c, err := cfg.Load("../../../../rules/raid")
	if err != nil {
		t.Fatal(err)
	}
	return c.Snapshot()
}
func fixture(roles ...string) model.Fight {
	boss := mt.Creature("Training Colossus", 42)
	start, end := mt.Encounter(model.Encounter{ID: 7, GroupSize: 10, InstanceID: model.Some(99), Success: model.Some(1), FightMs: model.Some(60000)}, 100000, 160000)
	b := mt.NewFight("fight", *end.Enc, start, end).BossUnits(boss)
	for i, role := range roles {
		p := mt.Player([]string{"Alia-Stone-PvP-EU", "Bram-Oak-Hill-PvP-EU", "Cato-PvP-EU", "Dara-PvP-EU", "Evan-PvP-EU", "Fara-PvP-EU", "Gale-PvP-EU"}[i])
		b.Players(p).Role(p, role)
	}
	f := b.Build()
	f.InstanceID = 99
	for _, p := range f.Players {
		f.Events = append(f.Events, mt.At(110000).Src(p).Dst(boss).Damage(6000, -1).Build(), model.Event{TimeMs: 110000, Kind: model.CastSuccess, Src: p})
	}
	return f
}
func part(t *testing.T, n model.NightScore, name string) model.Part {
	t.Helper()
	for _, p := range n.Fights[0].Parts {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("missing part %s", name)
	return model.Part{}
}
func stock(p model.Actor) model.PrepReport {
	return mt.Prep(p.Name, "", 60, 1, "test").Gear().Consumable("potion", 1, 1).Consumable("food", 2, 1).Clock(80, 90).Build()
}
func TestLoneTankCoverage(t *testing.T) {
	f := fixture("tank")
	c := config(t)
	for _, tc := range []struct {
		reports []model.PrepReport
		cov     int
	}{{nil, 17}, {[]model.PrepReport{stock(f.Players[0])}, 23}} {
		got := score.Score([]model.Fight{f}, nil, tc.reports, c)
		if len(got) != 1 || got[0].Coverage != tc.cov || !got[0].Tenths.Present || got[0].Rank.Present {
			t.Fatalf("want provisional %d got %+v", tc.cov, got)
		}
		for _, name := range []string{"output", "activity"} {
			if part(t, got[0], name).Tenths.Present {
				t.Fatalf("lone tank %s invented baseline", name)
			}
		}
		p := part(t, got[0], "preparation")
		if p.Tenths.Present != (tc.reports != nil) {
			t.Fatalf("prep %+v", p)
		}
	}
	c.Scoring.Confidence.ProvisionalMinCoverage = .19
	if got := score.Score([]model.Fight{f}, nil, nil, c)[0]; got.Tenths.Present || got.Coverage != 17 {
		t.Fatalf("below threshold %+v", got)
	}
	f.Events = nil
	for _, n := range score.Score([]model.Fight{f}, nil, nil, c) {
		if n.Tenths.Present || len(n.Fights) != 0 {
			t.Fatal("index-only fight scored")
		}
	}
}
func TestPrepSelectionAndMeasures(t *testing.T) {
	f := fixture("tank")
	p := f.Players[0]
	c := config(t)
	early := stock(p)
	early.Stamped = 101
	early.SenderClock = 11
	middle := mt.Prep(p.Name, "", 60, 1, "test").Gear(model.PrepGear{Slot: 1}, model.PrepGear{Slot: 1}, model.PrepGear{Slot: 2}).NotScored(2).Clock(22, 160).Build()
	late := stock(p)
	late.Stamped = 161
	late.SenderClock = 33
	reports := []model.PrepReport{late, early, middle}
	got := part(t, score.Score([]model.Fight{f}, nil, reports, c)[0], "preparation")
	counts := got.Evidence.Counts
	if got.Coverage != 60 || got.Tenths != model.Some(875) || counts["points_tenths.bag_gear"] != 750 || counts["points_tenths.stock"] != 1000 || counts["measured.used"] != 0 || counts["prep_stamped_s"] != 160 || counts["prep_sender_clock_s"] != 22 || counts["better_gear_slots"] != 1 {
		t.Fatalf("prep %+v", got)
	}
	for i := range f.Events {
		f.Events[i].TimeMs -= f.StartMs
	}
	f.StartMs = 0
	f.EndMs = 100000
	got = part(t, score.Score([]model.Fight{f}, nil, reports, c)[0], "preparation")
	if got.Evidence.Counts["prep_stamped_s"] != 101 {
		t.Fatalf("earliest fallback %+v", got)
	}
}
func TestPrepMarkersAndFlask(t *testing.T) {
	f := fixture("tank")
	c := config(t)
	p := f.Players[0]
	for _, tc := range []struct {
		report      model.PrepReport
		cov         int
		stock, gear bool
	}{
		{mt.Prep(p.Name, "", 60, 1, "").Gear().Consumable("flask", 9, 1).Build(), 60, true, true},
		{mt.Prep(p.Name, "", 60, 1, "").GearUnreadable().ConsumablesUnreadable().Build(), 0, false, false},
		{mt.Prep(p.Name, "", 60, 1, "").Build(), 30, true, false},
		{mt.Prep(p.Name, "", 60, 1, "").Gear().ConsumablesUnreadable().Build(), 30, false, true},
	} {
		got := part(t, score.Score([]model.Fight{f}, nil, []model.PrepReport{tc.report}, c)[0], "preparation")
		if got.Coverage != tc.cov || (got.Evidence.Counts["measured.stock"] == 1) != tc.stock || (got.Evidence.Counts["measured.bag_gear"] == 1) != tc.gear {
			t.Fatalf("markers %+v", got)
		}
		if tc.cov == 60 && (got.Tenths != model.Some(500) || got.Evidence.Counts["points_tenths.stock"] != 0 || got.Evidence.Counts["flask_count"] != 1) {
			t.Fatalf("flask %+v", got)
		}
	}
}
