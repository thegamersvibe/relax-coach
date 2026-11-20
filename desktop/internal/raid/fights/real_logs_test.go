package fights_test

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	. "github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Integration stays in the external test package: production fights imports
// only model. TestImports explicitly excludes _test.go files.
var realLogDecoder Decoder = func(r io.Reader, emit func(model.Event)) error {
	var report logfmt.ParseReport
	for e := range logfmt.Decode(r, &report) {
		emit(e)
	}
	return report.ReadError
}
var realLogExpectedNightPrefix = os.Getenv("RELAX_RAID_NIGHT_PREFIX")

func realLogParams() Params {
	return Params{PullGapMs: 10000, TailMs: 2500, LowConfidenceFightMs: 45000, ReentryGapMs: 1800000,
		TankMinBossSwingShare: .15, TankMinTopRatio: .5, HealerRequiresNotTank: true,
		HealerMinHealingDamageRatio: 1, HealerStrictlyGreater: true,
		BossUnitSelection: "highest_max_health_damaged_creature",
		DetailsFor:        func(model.Fight) bool { return true }, // Explicit full-detail integration coverage.
		InstanceKind: func(id, size int64) (model.RunKind, bool) {
			if id == 329 {
				return model.RunDungeon, true
			}
			if size > 5 {
				return model.RunRaid, false
			}
			return model.RunUnknown, false
		}}
}

func TestRealLogsR4(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("RELAX_RAID_REAL_LOGS not set; private real logs not run")
	}
	if realLogDecoder == nil {
		t.Skip("R3 decoder/config integration hook not installed; private real logs not run")
	}
	if len(realLogExpectedNightPrefix) != 8 {
		t.Fatal("private expected night-prefix hook required")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal("cannot read private fixture folder")
	}
	type observed struct {
		run              Run
		ids              []string
		feigns           int
		zoneOut          int64
		last             int64
		outside, missing int64
	}
	var all []observed
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		f, err := os.Open(filepath.Join(folder, entry.Name()))
		if err != nil {
			t.Fatal("cannot open private fixture")
		}
		obs := observed{}
		var playerDeaths []model.Event
		decode := func(r io.Reader, emit func(model.Event)) error {
			return realLogDecoder(r, func(e model.Event) {
				obs.last = e.TimeMs
				if e.IsDeath() && strings.HasPrefix(e.Dst.ID, "Player-") {
					playerDeaths = append(playerDeaths, e)
				}
				if e.Kind == model.UnitDied && e.DiedFlag == model.Some(1) {
					obs.feigns++
				}
				// The run ends at the FIRST change to the open world; the second log has a later one too.
				if e.Kind == model.ZoneChange && e.Zone != nil && e.Zone.InstanceID == 0 && obs.zoneOut == 0 {
					obs.zoneOut = e.TimeMs
				}
				emit(e)
			})
		}
		result, readErr := Read(f, decode, realLogParams())
		closeErr := f.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal("private decode failed (details withheld)")
		}
		if len(result.Runs) != 1 {
			t.Fatalf("runs=%d, want 1", len(result.Runs))
		}
		obs.run = result.Runs[0]
		assignedDeaths := map[int64]bool{}
		for _, fight := range obs.run.Fights {
			for _, e := range fight.Events {
				if e.IsDeath() && strings.HasPrefix(e.Dst.ID, "Player-") {
					assignedDeaths[e.Offset] = true
				}
			}
			if fight.Boss {
				if fight.Partial || fight.Success != model.Some(1) {
					t.Fatal("incomplete or non-kill boss")
				}
				obs.ids = append(obs.ids, fight.ID)
			}
		}
		// Independent oracle: account for every decoded player death, not only
		// the ones that survived segmentation. Byte offsets identify lines,
		// including distinct deaths at the same time and post-END tail events.
		for _, death := range playerDeaths {
			if death.TimeMs < obs.run.StartMs || death.TimeMs > obs.run.EndMs {
				obs.outside++
			} else if !assignedDeaths[death.Offset] {
				obs.missing++
			}
		}
		if result.PlayerDeathsOutsideRuns != obs.outside || result.PlayerDeathsInRunsWithoutFight != obs.missing {
			t.Fatalf("death counters disagree with decoded-line oracle: outside=%d/%d missing=%d/%d", result.PlayerDeathsOutsideRuns, obs.outside, result.PlayerDeathsInRunsWithoutFight, obs.missing)
		}
		if len(obs.ids) != 13 || obs.run.Kind != model.RunDungeon {
			t.Fatalf("bosses=%d; expected dungeon", len(obs.ids))
		}
		if obs.feigns != 15 {
			t.Fatalf("feign lines=%d", obs.feigns)
		}
		for _, d := range obs.run.Details {
			for _, tl := range d.Timelines {
				for _, death := range tl.Deaths {
					if death.DiedFlag != model.Some(0) {
						t.Fatal("feign death entered timeline deaths")
					}
				}
			}
		}
		if !strings.HasPrefix(obs.run.NightID, realLogExpectedNightPrefix) {
			t.Fatal("night prefix mismatch (ids withheld)")
		}
		sort.Strings(obs.ids)
		all = append(all, obs)
	}
	if len(all) != 2 {
		t.Fatalf("fixtures=%d, want 2", len(all))
	}
	sort.Slice(all, func(i, j int) bool { return all[i].run.StartMs < all[j].run.StartMs })
	for i, obs := range all {
		t.Logf("log=%d playerDeathsOutsideRuns=%d playerDeathsInRunsWithoutFight=%d", i+1, obs.outside, obs.missing)
		if obs.outside != int64(i) || obs.missing != 0 {
			t.Errorf("log=%d death scope: outside=%d missing=%d, want %d / 0", i+1, obs.outside, obs.missing, i)
		}
	}
	if !reflect.DeepEqual(all[0].ids, all[1].ids) || all[0].run.NightID != all[1].run.NightID {
		t.Fatal("cross-logger identity mismatch (ids withheld)")
	}
	if len(all[0].run.Fights) == 0 || !all[0].run.Fights[0].Partial {
		t.Fatal("first log not partial at start")
	}
	second := all[1]
	// The brief's "ten minutes after" is approximate: the second log runs 589 s past its first zone-out.
	if second.zoneOut == 0 || second.run.EndMs != second.zoneOut || second.last-second.zoneOut < 580000 {
		t.Fatal("zone-out boundary / post-run interval mismatch")
	}
	for _, fight := range second.run.Fights {
		for _, e := range fight.Events {
			if e.TimeMs > second.zoneOut {
				t.Fatal("post-zone event in run")
			}
		}
	}
	t.Log("2 private logs: 13 complete dungeon kills each; 15 feign lines each; identities and zone-out boundary agree")
}
