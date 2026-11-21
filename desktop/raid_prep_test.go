package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	mt "github.com/thegamersvibe/relax-coach/desktop/internal/raid/model/modeltest"
)

func raidPrepConfig(t *testing.T) cfg.Config {
	t.Helper()
	c, err := cfg.Load("../rules/raid")
	if err != nil {
		t.Fatal(err)
	}
	return c.Snapshot()
}
func raidPrepFixture(start, end int64, offset int8) model.Fight {
	p := mt.Player("Alia-Stone-PvP-EU")
	boss := mt.Creature("Training Colossus", 42)
	return model.Fight{ID: fmt.Sprint(start), StartMs: start, EndMs: end, DurationMs: model.Some(end - start), Boss: true, Success: model.Some(1), InstanceID: 99, GroupSize: 10, Players: []model.Actor{p}, Roles: map[string]string{p.ID: "tank"}, BossUnits: []string{boss.ID}, Events: []model.Event{{TimeMs: start, OffsetH: offset, Kind: model.Damage, Src: p, Dst: boss, Amount: model.Some(10)}}}
}
func raidPrepStore(stamps []int64, names []string) []byte {
	var reports, windows strings.Builder
	for i, at := range stamps {
		name := names[i]
		key := strings.ToLower(name)
		body := fmt.Sprintf("1\t0.2.0\t%s\tMAGE\t60\t123\tjoin\t%d\tpotion,101,1;food,102,1\t-\t-", name, at)
		fmt.Fprintf(&reports, "[%q]={[1]=%q},", key, body)
		fmt.Fprintf(&windows, "[%d]={[\"openedAt\"]=%d,[\"reports\"]={[%q]={[\"receivedAt\"]=%d,[\"report\"]=1}}},", i+1, at, key, at)
	}
	return []byte(fmt.Sprintf("RelaxRaidDB = {version=1,prep={version=2,nights={{day=20832,reports={%s},windows={%s}}}}}", reports.String(), windows.String()))
}
func TestRaidPrepUTCUnorderedFights(t *testing.T) {
	const hour = int64(time.Hour / time.Millisecond)
	const sec = int64(time.Second / time.Millisecond)
	start := int64(1800000000) * sec
	a := raidPrepFixture(start+3*hour, start+60000+3*hour, 3)
	b := raidPrepFixture(start+2*hour-4*hour, start+2*hour+60000-4*hour, -4)
	a.Events = append(a.Events, model.Event{OffsetH: -8})
	b.Events = append(b.Events, model.Event{OffsetH: 9})
	first := start / sec
	last := (start + 2*hour + 60000) / sec
	data := raidPrepStore([]int64{first}, []string{"Alia-Stone-PvP-EU"})
	for _, fs := range [][]model.Fight{{a, b}, {b, a}} {
		got, err := raidPrepScores(data, fs, nil, raidPrepConfig(t))
		if err != nil || got.WindowStartS != first-2*int64(time.Hour/time.Second) || got.WindowEndS != last || len(got.Reports) != 1 || got.Reports[0].Stamped != first {
			t.Fatalf("order/offset/bounds %+v %v", got, err)
		}
	}
}

func TestRaidPrepRejectsUnusableFights(t *testing.T) {
	c := raidPrepConfig(t)
	valid := raidPrepFixture(1800000000000, 1800000060000, 0)
	missing := valid
	missing.Events = nil
	reversed := valid
	reversed.EndMs = reversed.StartMs - 1
	beforeEpoch := raidPrepFixture(0, 60000, 3)
	for _, tc := range []struct {
		name   string
		fights []model.Fight
	}{{"empty", nil}, {"no offset", []model.Fight{missing}}, {"reversed", []model.Fight{reversed}}, {"negative UTC", []model.Fight{beforeEpoch}}} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() != nil {
					t.Fatal("invalid bounds panicked instead of returning an error")
				}
			}()
			got, err := raidPrepScores(nil, tc.fights, nil, c)
			if err == nil || len(got.Reports) != 0 || len(got.Scores) != 0 {
				t.Fatal("unusable fights must return an error and no result")
			}
		})
	}
}

func TestRaidPrepDiagnosticsDeathsAndPurity(t *testing.T) {
	f := raidPrepFixture(1800000000000, 1800000060000, 0)
	c := raidPrepConfig(t)
	data := []byte(`RelaxRaidDB = {version=1,prep={version=1}}`)
	deaths := []model.Death{mt.DeathOf(f.Players[0], f.ID, f.StartMs, model.CauseAvoidable).Owner(model.OwnerPlayer, f.Players[0].Name).Build()}
	before, _ := json.Marshal([]any{data, f, deaths, c})
	got, err := raidPrepScores(data, []model.Fight{f}, deaths, c)
	if err != nil || len(got.Diagnostics.Warnings) != 1 || len(got.Reports) != 0 || len(got.Scores) != 1 {
		t.Fatal("diagnostics or scores discarded")
	}
	for _, p := range got.Scores[0].Fights[0].Parts {
		if p.Name == "mechanics" && (p.Evidence.Counts["owned_deaths"] != 1 || p.Tenths != model.Some(600)) {
			t.Fatal("deaths did not reach score")
		}
		if p.Name == "preparation" && p.Tenths.Present {
			t.Fatal("invalid store fabricated preparation")
		}
	}
	after, _ := json.Marshal([]any{data, f, deaths, c})
	if string(before) != string(after) {
		t.Fatal("glue changed caller inputs")
	}
}

func TestRaidPrepUTCWindowAndScore(t *testing.T) {
	const sec = int64(time.Second / time.Millisecond)
	const hour = int64(time.Hour / time.Millisecond)
	start := int64(1800000000)*sec + 321
	end := start + 60000
	f := raidPrepFixture(start+3*hour, end+3*hour, 3)
	firstS := start / sec
	lastS := (end + sec - 1) / sec
	left := firstS - 2*int64(time.Hour/time.Second)
	stamps := []int64{left - 1, left, lastS, lastS + 1}
	names := []string{"Before-PvP-EU", "alia-stone-pvp-eu", "Guest-PvP-EU", "After-PvP-EU"}
	got, err := raidPrepScores(raidPrepStore(stamps, names), []model.Fight{f}, nil, raidPrepConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if got.WindowStartS != left || got.WindowEndS != lastS || len(got.Reports) != 2 || got.Reports[0].Stamped != left || got.Reports[1].Stamped != lastS || got.Diagnostics.Skipped["outside night"] != 2 {
		t.Fatalf("bounds/reports %+v", got)
	}
	if len(got.Unmatched) != 1 || got.Unmatched[0].Name != "Guest-PvP-EU" || len(got.Matched[f.Players[0].Name]) != 1 {
		t.Fatal("MatchPrep not fed actual roster")
	}
	if len(got.Scores) != 1 || len(got.Scores[0].Fights) != 1 {
		t.Fatal("score missing")
	}
	for _, p := range got.Scores[0].Fights[0].Parts {
		if p.Name == "preparation" {
			if !p.Tenths.Present || p.Tenths.V != 1000 || p.Evidence.Counts["prep_stamped_s"] != left || p.Evidence.Counts["fight_end_utc_s"] != end/sec {
				t.Fatalf("clock/score %+v", p)
			}
			return
		}
	}
	t.Fatal("preparation missing")
}
