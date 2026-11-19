package fights_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	. "github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Private comparison files must be outside the repository. No private identity
// is printed, including on failure. This test never reads the whole raw file.
func TestRealLogsR4Measurements(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("private logs not enabled")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal("private directory unavailable")
	}
	var identities [][]string
	logNo := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		logNo++
		for _, mode := range []string{"all", "index"} {
			p := realLogParams()
			if mode == "index" {
				p.DetailsFor = func(model.Fight) bool { return false }
			}
			f, err := os.Open(filepath.Join(folder, entry.Name()))
			if err != nil {
				t.Fatal("private log unavailable")
			}
			runtime.GC()
			var before, mem runtime.MemStats
			runtime.ReadMemStats(&before)
			peak := before.HeapAlloc
			sample := func() { runtime.ReadMemStats(&mem); peak = max(peak, mem.HeapAlloc) }
			events, feigns, rawAbsorbs := 0, 0, 0
			var zoneOut int64
			decode := func(r io.Reader, emit func(model.Event)) error {
				return realLogDecoder(r, func(e model.Event) {
					events++
					if e.Kind == model.Absorb {
						rawAbsorbs++
					}
					if e.Kind == model.UnitDied && e.DiedFlag == model.Some(1) {
						feigns++
					}
					if e.Kind == model.ZoneChange && e.Zone != nil && e.Zone.InstanceID == 0 && zoneOut == 0 {
						zoneOut = e.TimeMs
					}
					emit(e)
					if events%512 == 0 {
						sample()
					}
				})
			}
			res, err := Read(f, decode, p)
			f.Close()
			if err != nil {
				t.Fatal("private decode failed")
			}
			sample()
			runtime.GC()
			runtime.ReadMemStats(&mem)
			t.Logf("log=%d mode=%s bytes=%d events=%d runs=%d peakHeapDelta=%d retainedHeapDelta=%d", logNo, mode, res.Source.Bytes, events, len(res.Runs), peak-before.HeapAlloc, int64(mem.HeapAlloc)-int64(before.HeapAlloc))
			if mode == "all" {
				var ids []string
				bosses, trash, partial, kills, unowned, assigned, absent, absorbs := 0, 0, 0, 0, 0, 0, 0, 0
				roles := map[string]int{}
				var credit int64
				for _, run := range res.Runs {
					ids = append(ids, run.NightID)
					if zoneOut != 0 && run.EndMs != zoneOut {
						t.Fatal("run did not end at first zone out")
					}
					for i, fight := range run.Fights {
						if fight.Boss {
							bosses++
							ids = append(ids, fight.ID)
							if !fight.Partial && fight.Success == model.Some(1) {
								kills++
							}
						} else {
							trash++
						}
						if fight.Partial {
							partial++
						}
						for _, id := range fight.Unowned {
							if strings.HasPrefix(id, "Player-") {
								unowned++
							}
						}
						for _, a := range fight.Players {
							if role, ok := fight.Roles[a.ID]; ok {
								assigned++
								roles[role]++
							} else {
								absent++
							}
						}
						expected := map[string]int64{}
						candidates, hp := map[string]bool{}, map[string]int64{}
						for _, e := range fight.Events {
							if e.Kind == model.Absorb && e.Amount.Present && strings.HasPrefix(e.Caster.ID, "Player-") {
								absorbs++
								credit += e.Amount.V
								expected[e.Caster.ID] += e.Amount.V
							}
							if e.Kind == model.Heal && e.Amount.Present {
								id := e.Src.ID
								if o := fight.Owners[id]; o != "" {
									id = o
								}
								expected[id] += max(int64(0), e.Amount.V-max(int64(0), e.Overheal.V))
							}
							if e.Kind == model.Damage && strings.HasPrefix(e.Src.ID, "Player-") && (strings.HasPrefix(e.Dst.ID, "Creature-") || strings.HasPrefix(e.Dst.ID, "Vehicle-")) {
								candidates[e.Dst.ID] = true
							}
							if e.HPMax.Present {
								hp[e.InfoUnit] = max(hp[e.InfoUnit], e.HPMax.V)
							}
						}
						if os.Getenv("RELAX_RAID_BASELINE_WRITE") != "1" {
							for id, amount := range expected {
								if tl := run.Details[i].Timelines[id]; strings.HasPrefix(id, "Player-") && (tl == nil || tl.HealingDone != amount) {
									t.Fatal("caster healing total mismatch")
								}
							}
						}
						if fight.Boss {
							var top int64
							topID := ""
							for id := range candidates {
								if hp[id] > top || hp[id] == top && (topID == "" || id < topID) {
									top, topID = hp[id], id
								}
							}
							var rows []string
							for id := range candidates {
								if id != topID && hp[id] > 0 && hp[id]*2 >= top {
									parts := strings.Split(id, "-")
									if len(parts) >= 7 {
										npc, _ := strconv.ParseInt(parts[5], 10, 64)
										rows = append(rows, fmt.Sprintf("encounter=%d npc=%d maxHealth=%d topMaxHealth=%d", fight.EncounterID, npc, hp[id], top))
									}
								}
							}
							sort.Strings(rows)
							// Only secondary candidates; never print a private GUID.
							if len(rows) > 0 {
								for _, row := range rows {
									t.Log(row)
								}
							}
						}
					}
				}
				if os.Getenv("RELAX_RAID_BASELINE_WRITE") != "1" && unowned != 0 {
					t.Fatal("player in unowned")
				}
				identities = append(identities, ids)
				t.Logf("log=%d boss=%d trash=%d partial=%d completeKills=%d feigns=%d playerUnowned=%d rolesAssigned=%d rolesAbsent=%d roles=%v rawAbsorbs=%d absorbsWithPlayerCaster=%d casterCredit=%d", logNo, bosses, trash, partial, kills, feigns, unowned, assigned, absent, roles, rawAbsorbs, absorbs, credit)
			}
			runtime.KeepAlive(res)
		}
	}
	if logNo != 2 {
		t.Fatal("expected two private logs")
	}
	path := os.Getenv("RELAX_RAID_ID_BASELINE")
	if path != "" {
		if os.Getenv("RELAX_RAID_BASELINE_WRITE") == "1" {
			data, _ := json.Marshal(identities)
			if os.WriteFile(path, data, 0600) != nil {
				t.Fatal("cannot save private comparison")
			}
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal("private comparison unavailable")
			}
			var prior [][]string
			if json.Unmarshal(data, &prior) != nil || !reflect.DeepEqual(prior, identities) {
				t.Fatal("STOP: real identities moved")
			}
			t.Log("all complete boss identities and full night identities unchanged from baseline")
		}
	}
}
