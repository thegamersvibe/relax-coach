package score_test

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fights"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/logfmt"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/model"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/score"
)

// External integration only: production score imports neither reader nor fights.
// No path, line, player name or identifier is emitted, even on failure.
func TestRealLogsR7(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("private counts-only integration not enabled")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal("cannot read private fixture folder")
	}
	c := config(t)
	p := c.Scoring.Fights
	params := fights.Params{PullGapMs: p.PullGapMs, TailMs: p.TailMs, LowConfidenceFightMs: c.Scoring.Confidence.LowConfidenceFightMs, ReentryGapMs: p.ReentryGapMs, TankMinBossSwingShare: p.TankMinBossSwingShare, TankMinTopRatio: p.TankMinTopRatio, HealerRequiresNotTank: p.HealerRequiresNotTank, HealerMinHealingDamageRatio: p.HealerMinHealingDamageRatio, HealerStrictlyGreater: p.HealerStrictlyGreater, BossUnitSelection: p.BossUnitSelection, InstanceKind: c.InstanceKind, DetailsFor: func(model.Fight) bool { return true }, BossUnits: func(id int64) []fights.CreatureSelector {
		rule, _ := c.Encounter(id)
		var out []fights.CreatureSelector
		for _, u := range rule.BossUnits {
			out = append(out, fights.CreatureSelector{ID: u.ID, Name: u.Name, Source: u.Source})
		}
		return out
	}}
	var comparisons []map[string]int64
	logs := 0
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		file, err := os.Open(filepath.Join(folder, entry.Name()))
		if err != nil {
			t.Fatal("cannot open private fixture")
		}
		var report logfmt.ParseReport
		decoder := func(r io.Reader, emit func(model.Event)) error {
			for e := range logfmt.Decode(r, &report) {
				emit(e)
			}
			return report.ReadError
		}
		result, readErr := fights.Read(file, decoder, params)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			t.Fatal("private decode failed; details withheld")
		}
		logs++
		nightCounts := [3]int{}
		bosses := 0
		byRole := map[string]map[string]int{}
		coverage := map[string][]int{}
		fightCoverage := map[string][]int{}
		active := map[string][]float64{}
		comp := map[string]int64{}
		var names []string
		for _, run := range result.Runs {
			if run.Kind != model.RunDungeon {
				t.Fatal("expected private dungeon run")
			}
			// R6 is not in this branch. Death classification input is deliberately empty.
			ns := score.Score(run.Fights, nil, nil, c)
			if err := evidenceCheck(ns, c); err != nil {
				t.Fatalf("private score evidence: %v", err)
			}
			b1, _ := json.Marshal(ns)
			b2, _ := json.Marshal(score.Score(run.Fights, nil, nil, c))
			if !bytes.Equal(b1, b2) {
				t.Fatal("private score nondeterminism")
			}
			for _, f := range run.Fights {
				if f.Boss {
					bosses++
				}
				for _, a := range f.Players {
					names = append(names, a.Name)
				}
			}
			for _, n := range ns {
				if n.Rank.Present {
					t.Fatal("private dungeon ranked")
				}
				status := 2
				if n.Tenths.Present {
					status = 1
					if float64(n.Coverage) >= c.Scoring.Confidence.MinCoverage*100 {
						status = 0
					}
				}
				nightCounts[status]++
				roles := map[string]bool{}
				for _, fs := range n.Fights {
					roles[fs.Role] = true
					fightCoverage[fs.Role] = append(fightCoverage[fs.Role], fs.Coverage)
					if fs.Tenths.Present {
						comp[fs.FightID+"/"+n.PlayerID] = fs.Tenths.V
					}
					for _, part := range fs.Parts {
						if part.Name == "activity" && part.Evidence.Counts["alive_ms"] > 0 {
							active[fs.Role] = append(active[fs.Role], float64(part.Evidence.Counts["active_ms"])/float64(part.Evidence.Counts["alive_ms"]))
						}
					}
				}
				if len(roles) == 0 {
					roles["unknown"] = true
				}
				for role := range roles {
					if byRole[role] == nil {
						byRole[role] = map[string]int{}
					}
					byRole[role][n.PlayerID] = status
					coverage[role] = append(coverage[role], n.Coverage)
				}
			}
		}
		if bosses != 13 {
			t.Fatalf("private log %d: boss count %d, want 13", logs, bosses)
		}
		_, unmatched := score.MatchPrep(nil, names)
		t.Logf("log %d: runs=%d bosses=%d prep_reports=0 prep_unmatched=%d classified_deaths=0 (R6 absent)", logs, len(result.Runs), bosses, len(unmatched))
		t.Logf("log %d unique players normal/provisional/none=%v; role rows use any detailed fight, so memberships may overlap", logs, nightCounts)
		for _, role := range []string{"tank", "healer", "dps", "unknown"} {
			counts := [3]int{}
			for _, status := range byRole[role] {
				counts[status]++
			}
			values := active[role]
			sort.Float64s(values)
			lo, med, hi := 0.0, 0.0, 0.0
			if len(values) > 0 {
				lo, hi = values[0], values[len(values)-1]
				med = values[len(values)/2]
				if len(values)%2 == 0 {
					med = (med + values[len(values)/2-1]) / 2
				}
			}
			t.Logf("log %d role=%s players normal/provisional/none=%v night_coverage=%v fight_coverage=%v active_samples=%d active_share_min/median/max=%.4f/%.4f/%.4f", logs, role, counts, unique(coverage[role]), unique(fightCoverage[role]), len(values), lo, med, hi)
		}
		t.Logf("log %d range/cast unmatched count skipped: model.Event merges RANGE damage/miss with SPELL; Raw is absent on both", logs)
		comparisons = append(comparisons, comp)
	}
	if logs != 2 {
		t.Fatalf("private fixture count %d, want 2", logs)
	}
	pairs := 0
	maxDifference := int64(0)
	for key, a := range comparisons[0] {
		if b, ok := comparisons[1][key]; ok {
			pairs++
			maxDifference = max(maxDifference, int64(math.Abs(float64(a-b))))
		}
	}
	t.Logf("cross-log scored player/fight pairs=%d largest_difference_tenths=%d (not asserted equal)", pairs, maxDifference)
}
func unique(values []int) []int {
	sort.Ints(values)
	out := []int{}
	for _, v := range values {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}
