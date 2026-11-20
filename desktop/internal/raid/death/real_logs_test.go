package death_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/death"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
)

// Integration is deliberately external: production death imports model/cfg
// only. Never print a path, error containing a path, actor, spell or fight id.
func TestRealLogsR6(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("RELAX_RAID_REAL_LOGS not set; private real logs not run")
	}
	loaded, err := cfg.Load(filepath.Join("..", "..", "..", "..", "rules", "raid"))
	if err != nil {
		t.Fatal("cannot load shipped configuration")
	}
	c := loaded.Snapshot()
	fp := c.Scoring.Fights
	params := fights.Params{PullGapMs: fp.PullGapMs, TailMs: fp.TailMs, ReentryGapMs: fp.ReentryGapMs,
		LowConfidenceFightMs: c.Scoring.Confidence.LowConfidenceFightMs, TankMinBossSwingShare: fp.TankMinBossSwingShare,
		TankMinTopRatio: fp.TankMinTopRatio, HealerRequiresNotTank: fp.HealerRequiresNotTank,
		HealerMinHealingDamageRatio: fp.HealerMinHealingDamageRatio, HealerStrictlyGreater: fp.HealerStrictlyGreater,
		BossUnitSelection: fp.BossUnitSelection, InstanceKind: c.InstanceKind, DetailsFor: func(model.Fight) bool { return true },
		BossUnits: func(id int64) []fights.CreatureSelector {
			r, _ := c.Encounter(id)
			var out []fights.CreatureSelector
			for _, b := range r.BossUnits {
				out = append(out, fights.CreatureSelector{ID: b.ID})
			}
			return out
		}}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal("cannot read private fixture folder")
	}
	logs := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		logs++
		file, err := os.Open(filepath.Join(folder, entry.Name()))
		if err != nil {
			t.Fatal("cannot open private fixture")
		}
		feignsAll, feignsInFights, expected, total, wipes, excludedTotal := 0, 0, 0, 0, 0, 0
		decode := func(r io.Reader, emit func(model.Event)) error {
			var report logfmt.ParseReport
			for e := range logfmt.Decode(r, &report) {
				if e.Kind == model.UnitDied && e.DiedFlag == model.Some(1) && strings.HasPrefix(e.Dst.ID, "Player-") {
					feignsAll++
				}
				emit(e)
			}
			for _, n := range report.Skipped {
				if n != 0 {
					return fmt.Errorf("private parser skipped lines")
				}
			}
			return report.ReadError
		}
		result, readErr := fights.Read(file, decode, params)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal("private decode failed; details withheld")
		}
		causes, suggestions, confidence := map[string]int{}, map[string]int{}, map[string]int{}
		for _, run := range result.Runs {
			for _, f := range run.Fights {
				players := map[string]bool{}
				for _, p := range f.Players {
					if strings.HasPrefix(p.ID, "Player-") {
						players[p.ID] = true
					}
				}
				type key struct {
					id string
					at int64
				}
				want := map[key]int{}
				wantExcluded := 0
				for _, e := range f.Events {
					if !players[e.Dst.ID] {
						continue
					}
					if e.IsDeath() {
						if e.TimeMs > run.EndMs {
							wantExcluded++
							continue
						}
						expected++
						want[key{e.Dst.ID, e.TimeMs}]++
					}
					if e.Kind == model.UnitDied && e.DiedFlag == model.Some(1) {
						feignsInFights++
					}
				}
				ds, ws, excluded := death.Review(f, run.EndMs, c)
				again, againW, excludedAgain := death.Review(f, run.EndMs, c)
				if excluded != wantExcluded || excludedAgain != excluded {
					t.Fatalf("outside run deaths=%d want=%d repeat=%d", excluded, wantExcluded, excludedAgain)
				}
				excludedTotal += excluded
				a, e1 := json.Marshal(struct {
					D []model.Death
					W []model.Wipe
				}{ds, ws})
				b, e2 := json.Marshal(struct {
					D []model.Death
					W []model.Wipe
				}{again, againW})
				if e1 != nil || e2 != nil || !bytes.Equal(a, b) {
					t.Fatal("private review is not byte deterministic")
				}
				wipes += len(ws)
				for _, d := range ds {
					k := key{d.Player.ID, d.TimeMs}
					if want[k] == 0 {
						t.Fatal("death not backed by a real player death line")
					}
					want[k]--
					total++
					causes[d.Cause]++
					suggestions[d.SuggestedCause]++
					confidence[d.Confidence]++
					if d.OwnerKind == model.OwnerHealers {
						living := map[string]bool{}
						for _, p := range f.Players {
							if f.Roles[p.ID] == "healer" && diagnosticAlive(f, p.ID, d.TimeMs) {
								living[p.Name] = true
							}
						}
						if len(living) == 0 || len(d.Owners) != len(living) {
							t.Fatal("healer ownership without whole living team")
						}
						for _, owner := range d.Owners {
							if !living[owner] {
								t.Fatal("dead or non-healer owner")
							}
						}
					}
					diag := diagnostics(f, d, c.Scoring.Death)
					diag.Log = logs
					diag.Death = total
					line, err := json.Marshal(diag)
					if err != nil {
						t.Fatal("cannot encode anonymous diagnostic")
					}
					t.Log(string(line))
				}
				for _, n := range want {
					if n != 0 {
						t.Fatal("player death line not reviewed exactly once")
					}
				}
			}
		}
		if total != expected || wipes != 0 {
			t.Fatalf("deaths=%d expected=%d wipes=%d", total, expected, wipes)
		}
		if feignsAll != 15 {
			t.Fatalf("feigns=%d, want 15", feignsAll)
		}
		line, _ := json.Marshal(struct {
			Log, Deaths, FeignsExcluded, FeignsAll, Wipes, ExcludedPastRun int
			Causes, SuggestedCauses, Confidence                            map[string]int
		}{logs, total, feignsInFights, feignsAll, wipes, excludedTotal, causes, suggestions, confidence})
		t.Log(string(line))
	}
	if logs != 2 {
		t.Fatalf("private logs=%d, want 2", logs)
	}
}

type exposure struct {
	Candidate                int
	Eligible                 bool
	PlayersHit, PlayersAlive int
	Share                    float64
}
type diagnostic struct {
	Log, Death, HealthSamples int
	RaidwideCandidates        []exposure
	RaidwideShare             *float64
}

func diagnosticAlive(f model.Fight, id string, at int64) bool {
	es := append([]model.Event(nil), f.Events...)
	sort.SliceStable(es, func(i, j int) bool { return es[i].TimeMs < es[j].TimeMs })
	alive := true
	for _, e := range es {
		if e.TimeMs > at {
			break
		}
		if e.Dst.ID == id {
			if e.IsDeath() {
				alive = false
			}
			if e.Kind == model.Resurrect {
				alive = true
			}
		}
	}
	return alive
}

// Independently recompute tunable inputs from selected model events. No ids or
// names enter the returned diagnostic, including when a candidate is ineligible.
func diagnostics(f model.Fight, d model.Death, p cfg.Death) diagnostic {
	out := diagnostic{}
	es := append([]model.Event(nil), f.Events...)
	sort.SliceStable(es, func(i, j int) bool { return es[i].TimeMs < es[j].TimeMs })
	var damage []model.Event
	amounts := map[int64]int64{}
	total := int64(0)
	for _, e := range es {
		if e.TimeMs < d.TimeMs-p.WindowMs || e.TimeMs > d.TimeMs {
			continue
		}
		if e.InfoUnit == d.Player.ID && e.HPMax.Present && e.HPMax.V > 0 {
			out.HealthSamples++
		}
		if e.Kind != model.Damage || e.Stream == model.SwingSource || e.Dst.ID != d.Player.ID {
			continue
		}
		damage = append(damage, e)
		n := int64(0)
		if e.Amount.Present {
			n = e.Amount.V
		}
		if e.Overkill.Present {
			n -= max(int64(0), e.Overkill.V)
		}
		n = max(int64(0), n)
		amounts[e.SpellID] += n
		total += n
	}
	if len(damage) == 0 {
		return out
	}
	last := damage[len(damage)-1]
	candidates := []model.Event{last}
	for _, e := range damage {
		if e.SpellID != last.SpellID && amounts[e.SpellID] > total-amounts[e.SpellID] {
			candidates = append(candidates, e)
			break
		}
	}
	players := map[string]bool{}
	for _, a := range f.Players {
		if strings.HasPrefix(a.ID, "Player-") {
			players[a.ID] = true
		}
	}
	creature := func(a model.Actor) bool {
		if !strings.HasPrefix(a.ID, "Creature-") && !strings.HasPrefix(a.ID, "Vehicle-") {
			return false
		}
		if f.Owners[a.ID] != "" || byte(a.Flags>>12)&3 != 0 {
			return false
		}
		for _, id := range f.Unowned {
			if id == a.ID {
				return false
			}
		}
		return true
	}
	for i, candidate := range candidates {
		from, to := last.TimeMs-p.RaidwideWindowMs, last.TimeMs+p.RaidwideWindowMs
		x := exposure{Candidate: i + 1, Eligible: candidate.Stream == model.StreamNone && creature(candidate.Src) && candidate.Src.Flags&0x40 != 0}
		for id := range players {
			if id == d.Player.ID || diagnosticAlive(f, id, from) {
				x.PlayersAlive++
			}
		}
		hits := map[string]bool{}
		for _, e := range es {
			if e.TimeMs >= from && e.TimeMs <= to && e.Kind == model.Damage && e.Stream == model.StreamNone && e.SpellID == candidate.SpellID && creature(e.Src) && players[e.Dst.ID] {
				hits[e.Dst.ID] = true
			}
		}
		x.PlayersHit = len(hits)
		if x.PlayersAlive > 0 {
			x.Share = float64(x.PlayersHit) / float64(x.PlayersAlive)
		}
		out.RaidwideCandidates = append(out.RaidwideCandidates, x)
		if x.Eligible && (out.RaidwideShare == nil || x.Share > *out.RaidwideShare) {
			share := x.Share
			out.RaidwideShare = &share
		}
	}
	return out
}
