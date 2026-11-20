package compare_test

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/compare"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// Opt-in integration only. Never print a path, name, ID or raw line, including
// failures. The officer file is synthetic; the log scores are not fabricated.
func TestRealLogsComparison(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("private counts-only integration not enabled")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal("cannot read private fixture folder")
	}
	loaded, err := cfg.Load("../../../../rules/raid")
	if err != nil {
		t.Fatal("cannot load rules")
	}
	c := loaded.Snapshot()
	p := c.Scoring.Fights
	params := fights.Params{PullGapMs: p.PullGapMs, TailMs: p.TailMs, LowConfidenceFightMs: c.Scoring.Confidence.LowConfidenceFightMs, ReentryGapMs: p.ReentryGapMs, TankMinBossSwingShare: p.TankMinBossSwingShare, TankMinTopRatio: p.TankMinTopRatio, HealerRequiresNotTank: p.HealerRequiresNotTank, HealerMinHealingDamageRatio: p.HealerMinHealingDamageRatio, HealerStrictlyGreater: p.HealerStrictlyGreater, BossUnitSelection: p.BossUnitSelection, InstanceKind: c.InstanceKind, DetailsFor: func(model.Fight) bool { return true }, BossUnits: func(id int64) []fights.CreatureSelector {
		rule, _ := c.Encounter(id)
		var out []fights.CreatureSelector
		for _, u := range rule.BossUnits {
			out = append(out, fights.CreatureSelector{ID: u.ID, Name: u.Name, Source: u.Source})
		}
		return out
	}}
	officer, err := compare.ParseOfficer([]byte(`{"version":1,"officer":"synthetic prediction","top":["Alia-Stone-PvP-EU"],"bottom":["Bram-Oak-Hill-PvP-EU"]}`))
	if err != nil {
		t.Fatal("synthetic officer file failed")
	}
	logs := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		file, err := os.Open(filepath.Join(folder, entry.Name()))
		if err != nil {
			t.Fatal("cannot open private fixture")
		}
		decoder := func(r io.Reader, emit func(model.Event)) error {
			var report logfmt.ParseReport
			for e := range logfmt.Decode(r, &report) {
				emit(e)
			}
			return report.ReadError
		}
		result, readErr := fights.Read(file, decoder, params)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal("private decode failed")
		}
		logs++
		rows, ranked, reports, noReports, bosses := 0, 0, 0, 0, 0
		for _, run := range result.Runs {
			var deaths []model.Death
			var roster []model.Actor
			seen := map[string]bool{}
			for _, f := range run.Fights {
				if f.Boss {
					bosses++
				}
				for _, a := range f.Players {
					if !seen[a.ID] {
						roster = append(roster, a)
						seen[a.ID] = true
					}
				}
				ds, _, _, err := death.Review(f, run.EndMs, c)
				if err != nil {
					t.Fatal("private death review failed")
				}
				deaths = append(deaths, ds...)
			}
			ns := score.Score(run.Fights, deaths, nil, c)
			for _, n := range ns {
				if n.Rank.Present {
					ranked++
				}
			}
			rows += len(ns)
			got, err := compare.Compare(ns, roster, c, officer, run.Fights)
			if err != nil {
				t.Fatal("private comparison failed")
			}
			if got.NoReportReason != "" {
				noReports++
			} else {
				reports++
			}
			if got.Ranked != 0 || got.NoReportReason != "fewer than 4 ranked players" || len(got.Top)+len(got.Bottom)+len(got.ScoreTop)+len(got.ScoreBottom) != 0 {
				t.Fatal("private unranked night produced a report")
			}
			again, err := compare.Compare(ns, roster, c, officer, run.Fights)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("private comparison nondeterminism")
			}
		}
		if bosses != 13 || len(result.Runs) == 0 || rows == 0 || ranked != 0 {
			t.Fatal("unexpected private counts")
		}
		t.Logf("log %d: runs=%d bosses=%d night_scores=%d ranked=%d reports=%d no_report=%d (synthetic officer file)", logs, len(result.Runs), bosses, rows, ranked, reports, noReports)
	}
	if logs != 2 {
		t.Fatalf("private fixture count=%d, want 2", logs)
	}
}
