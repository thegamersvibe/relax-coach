package fights

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Local integration adapter only: provide these from an untracked test file once
// R3 is integrated. Keep the private expected prefix out of the repository.
// The hook must wrap the real decoder, not a replacement synthetic parser.
var realLogDecoder Decoder
var realLogParams func() Params
var realLogExpectedNightPrefix string

func TestRealLogsR4(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("RELAX_RAID_REAL_LOGS not set; private real logs not run")
	}
	if realLogDecoder == nil || realLogParams == nil {
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
		run     Run
		ids     []string
		feigns  int
		zoneOut int64
		last    int64
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
		decode := func(r io.Reader, emit func(model.Event)) error {
			return realLogDecoder(r, func(e model.Event) {
				obs.last = e.TimeMs
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
		for _, fight := range obs.run.Fights {
			if fight.Boss {
				if fight.Partial || fight.Success != model.Some(1) {
					t.Fatal("incomplete or non-kill boss")
				}
				obs.ids = append(obs.ids, fight.ID)
			}
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
