package cfg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

func load(t *testing.T, dir string) *cfg.Config {
	t.Helper()
	c, err := cfg.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func copyData(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range dataFiles {
		b, err := os.ReadFile(filepath.Join(seedDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func change(t *testing.T, dir, name string, edit func(map[string]any)) {
	t.Helper()
	path := filepath.Join(dir, name)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	edit(doc)
	b, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfig(t *testing.T) {
	c := load(t, seedDir)
	s := c.Scoring
	if s.Fights.PullGapMs != 10000 || s.Fights.TailMs != 2500 || s.Confidence.LowConfidenceFightMs != 45000 {
		t.Fatalf("fight defaults: %+v, %+v", s.Fights, s.Confidence)
	}
	if s.Fights.TankMinBossSwingShare != .15 || s.Fights.TankMinTopRatio != .5 || s.Fights.ReentryGapMs != 1800000 || !s.Fights.HealerRequiresNotTank || s.Fights.HealerMinHealingDamageRatio != 1 || !s.Fights.HealerStrictlyGreater {
		t.Fatalf("role defaults: %+v", s.Fights)
	}
	if !reflect.DeepEqual(s.Weights.DPS, cfg.PartWeights{Output: 35, Activity: 25, Mechanics: 20, Utility: 10, Preparation: 10}) ||
		!reflect.DeepEqual(s.Weights.Healer, cfg.PartWeights{Output: 25, Activity: 25, Mechanics: 20, Utility: 20, Preparation: 10}) ||
		!reflect.DeepEqual(s.Weights.Tank, cfg.PartWeights{Output: 10, Activity: 20, Mechanics: 35, Utility: 25, Preparation: 10}) {
		t.Fatalf("weights: %+v", s.Weights)
	}
	if s.Night.TrashShare != .05 || s.Night.MinBossShare != .95 || s.Night.MaxTrashShare != .05 || s.Night.KillWeight != 1 || s.Night.WipeWeight != .5 || s.Confidence.MinCoverage != .6 || s.Confidence.ShortFightWeight != .5 {
		t.Fatalf("night/confidence defaults: %+v %+v", s.Night, s.Confidence)
	}
	if s.Confidence.MinBaselinePlayers != 3 || s.Confidence.MinMeasurablePlayers != 2 {
		t.Fatal("R7 small cohorts")
	}
	if !reflect.DeepEqual(s.RatioPoints, []cfg.RatioPoint{{Ratio: .5, Score: 0}, {Ratio: 1, Score: 70}, {Ratio: 1.3, Score: 100}}) {
		t.Fatalf("ratio mapping: %+v", s.RatioPoints)
	}
	if s.Healer.TeamShare != .5 || s.Healer.IndividualShare != .5 || s.Healer.UnhealedTeamScore != 0 || s.Healer.UnhealedOutputCap != 50 || s.CountStartScore != 100 {
		t.Fatal("healer guard/count defaults")
	}
	if s.Death.WindowMs != 10000 || s.Death.OneShotWindowMs != 1500 || s.Death.UnhealedMinMs != 4000 || s.Death.WipeDeadShare != .4 || !s.Death.WipeAllTanksDead || !s.Death.WipeAllHealersDead {
		t.Fatal("death defaults")
	}
	if s.Toggles.Readiness || s.Toggles.RequiredConsumables || s.Toggles.SupportCredit || s.Toggles.History {
		t.Fatal("phase 2 toggles start off")
	}
	if len(s.Assignments) != 5 || s.Assignments[2].OutputToUtility != 15 {
		t.Fatal("job table")
	}
	if len(c.Specs.Profiles) != 33 || len(c.SpecIDs.Classes) != 9 || len(c.Support.Effects) == 0 || len(c.Candidates.Encounters) != 22 || len(c.Encounters.Encounters) != 1 || c.Encounters.DefaultTargetRule != "boss_only" || len(c.Consumables.Categories) == 0 {
		t.Fatal("missing catalogue")
	}
	if !reflect.DeepEqual(c.Realms.Labels, []string{"PvP-EU"}) || c.Instances.RaidGroupSizeAbove != 5 || len(c.Instances.Instances) != 1 || c.Instances.Instances[0].ID != 329 || c.Instances.Instances[0].Kind != "dungeon" {
		t.Fatal("realm/instance seed")
	}
}

// A small independent consumer, not the R7 engine. Re-score one stored synthetic
// input and persist its result in the SAME running test binary after each edit.
// R7 must repeat these four contracts through its full scorer on integration.
type sampleNight struct{ BossDamage, AddDamage, Activity, Mechanics float64 }
type sampleResult struct {
	Score, Coverage float64
	Ranked          bool
}

func rescoreStored(t *testing.T, dir, nightPath, resultPath string) sampleResult {
	t.Helper()
	b, err := os.ReadFile(nightPath)
	if err != nil {
		t.Fatal(err)
	}
	var n sampleNight
	if err := json.Unmarshal(b, &n); err != nil {
		t.Fatal(err)
	}
	c := load(t, dir)
	w := c.Scoring.Weights.DPS
	output := n.BossDamage
	for _, rule := range c.Encounters.Encounters {
		if rule.ID == 77 && rule.TargetRule != nil && rule.TargetRule.Mode == "boss_and_units" {
			for _, unit := range rule.TargetRule.Units {
				if unit.Matches(78) {
					output += n.AddDamage
				}
			}
		}
	}
	for _, p := range c.Specs.Profiles {
		if p.ID == "WARRIOR.arms" && p.Uncalibrated {
			w.Output = 0
		}
	}
	measured := w.Output + w.Activity + w.Mechanics
	r := sampleResult{Score: (output*w.Output + n.Activity*w.Activity + n.Mechanics*w.Mechanics) / measured, Coverage: measured / 100}
	r.Ranked = r.Coverage >= c.Scoring.Confidence.MinCoverage
	b, err = json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultPath, b, 0600); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(resultPath)
	if err != nil {
		t.Fatal(err)
	}
	var stored sampleResult
	if err := json.Unmarshal(b, &stored); err != nil {
		t.Fatal(err)
	}
	return stored
}

func reloadCase(t *testing.T, file string, edit func(map[string]any)) (sampleResult, sampleResult) {
	t.Helper()
	dir := copyData(t)
	night := filepath.Join(t.TempDir(), "night.json")
	result := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(night, []byte(`{"BossDamage":60,"AddDamage":20,"Activity":20,"Mechanics":100}`), 0600); err != nil {
		t.Fatal(err)
	}
	before := rescoreStored(t, dir, night, result)
	change(t, dir, file, edit)
	after := rescoreStored(t, dir, night, result)
	if reflect.DeepEqual(before, after) {
		t.Fatalf("file edit had no effect: %+v", after)
	}
	return before, after
}
func TestReloadWeight(t *testing.T) {
	b, a := reloadCase(t, "scoring.json", func(d map[string]any) {
		w := d["weights"].(map[string]any)["dps"].(map[string]any)
		w["output"] = 25
		w["activity"] = 35
	})
	if b.Score != 57.5 || a.Score != 52.5 || a.Coverage != b.Coverage {
		t.Fatalf("before=%+v after=%+v", b, a)
	}
}
func TestReloadThreshold(t *testing.T) {
	b, a := reloadCase(t, "scoring.json", func(d map[string]any) { d["confidence"].(map[string]any)["min_coverage"] = .9 })
	if !b.Ranked || a.Ranked || a.Score != b.Score {
		t.Fatalf("before=%+v after=%+v", b, a)
	}
}
func TestReloadProfile(t *testing.T) {
	b, a := reloadCase(t, "specs.json", func(d map[string]any) {
		for _, v := range d["profiles"].([]any) {
			p := v.(map[string]any)
			if p["id"] == "WARRIOR.arms" {
				p["uncalibrated"] = true
			}
		}
	})
	if a.Ranked || a.Coverage >= b.Coverage {
		t.Fatalf("before=%+v after=%+v", b, a)
	}
}
func TestReloadEncounter(t *testing.T) {
	b, a := reloadCase(t, "encounters-enabled.json", func(d map[string]any) {
		d["encounters"] = []any{map[string]any{"id": 77, "name": "Synthetic boss", "source": "synthetic test evidence", "boss_units": []any{}, "mechanics": []any{}, "target_rule": map[string]any{"mode": "boss_and_units", "units": []any{map[string]any{"id": 78, "name": "Synthetic add", "source": "synthetic test evidence"}}}}}
	})
	if a.Score <= b.Score || a.Coverage != b.Coverage {
		t.Fatalf("before=%+v after=%+v", b, a)
	}
}
