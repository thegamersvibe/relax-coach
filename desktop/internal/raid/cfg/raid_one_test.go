package cfg_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

// Check the public JSON projection as well as Load, so a missing typed field
// fails behaviorally before the field is implemented (not as a build failure).
func scoringObject(t *testing.T, value any) map[string]any {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEnergyGCDDecode(t *testing.T) {
	for _, value := range []any{1000, nil} {
		t.Run("explicit_"+nameValue(value), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d["gcd_energy_ms"] = value })
			got := scoringObject(t, load(t, dir).Scoring)
			want := any(nil)
			if value != nil {
				want = float64(1000)
			}
			if actual, ok := got["gcd_energy_ms"]; !ok || !reflect.DeepEqual(actual, want) {
				t.Fatalf("gcd_energy_ms = %v, present=%v; want %v", actual, ok, want)
			}
		})
	}
	t.Run("required_key", func(t *testing.T) {
		dir := copyValidationData(t)
		change(t, dir, "scoring.json", func(d map[string]any) { delete(d, "gcd_energy_ms") })
		err := refused(t, dir, "scoring.json")
		if !strings.Contains(err.Error(), "gcd_energy_ms is required") {
			t.Fatalf("missing energy GCD diagnostic: %v", err)
		}
	})
}

func TestEnergyGCDRange(t *testing.T) {
	for _, value := range []any{-1, 0, uint64(math.MaxInt64) + 1, 1.5, "1000"} {
		t.Run("reject_"+nameValue(value), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d["gcd_energy_ms"] = value })
			refusedField(t, dir, "scoring.json", "gcd_energy_ms")
		})
	}
	for _, value := range []int64{1, math.MaxInt64} {
		t.Run("accept_"+nameValue(value), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d["gcd_energy_ms"] = value })
			got := load(t, dir).Scoring.GCDEnergyMs
			if got == nil || *got != value {
				t.Fatalf("energy GCD lost exact integer %d: %v", value, got)
			}
		})
	}
}

func TestEnergyGCDSnapshotCopies(t *testing.T) {
	c := load(t, seedDir)
	if c.Scoring.GCDEnergyMs == nil || *c.Scoring.GCDEnergyMs != 1000 {
		t.Fatal("seed must supply the energy GCD")
	}
	before := c.Snapshot()
	*c.Scoring.GCDEnergyMs = 7
	if got := c.Snapshot(); *got.Scoring.GCDEnergyMs != 1000 {
		t.Error("Load view aliases energy GCD in the validated snapshot")
	}
	*before.Scoring.GCDEnergyMs = 9
	if got := c.Snapshot(); *got.Scoring.GCDEnergyMs != 1000 {
		t.Error("Snapshot aliases energy GCD in the validated snapshot")
	}
}

func TestProvisionalCoverageDecode(t *testing.T) {
	t.Run("explicit_value", func(t *testing.T) {
		dir := copyValidationData(t)
		change(t, dir, "scoring.json", func(d map[string]any) {
			d["confidence"].(map[string]any)["provisional_min_coverage"] = .25
		})
		got := scoringObject(t, load(t, dir).Scoring.Confidence)
		if got["provisional_min_coverage"] != .25 {
			t.Fatalf("provisional_min_coverage = %v; want .25", got["provisional_min_coverage"])
		}
	})
	t.Run("required_key", func(t *testing.T) {
		dir := copyValidationData(t)
		change(t, dir, "scoring.json", func(d map[string]any) {
			delete(d["confidence"].(map[string]any), "provisional_min_coverage")
		})
		err := refused(t, dir, "scoring.json")
		if !strings.Contains(err.Error(), "confidence.provisional_min_coverage is required") {
			t.Fatalf("missing provisional threshold diagnostic: %v", err)
		}
	})
	t.Run("null_refused", func(t *testing.T) {
		dir := copyValidationData(t)
		change(t, dir, "scoring.json", func(d map[string]any) {
			d["confidence"].(map[string]any)["provisional_min_coverage"] = nil
		})
		err := refused(t, dir, "scoring.json")
		if !strings.Contains(err.Error(), "confidence.provisional_min_coverage cannot be null") {
			t.Fatalf("null provisional threshold diagnostic: %v", err)
		}
	})
}

func TestProvisionalCoverageRange(t *testing.T) {
	for _, tc := range []struct {
		provisional, minimum float64
		valid                bool
	}{
		{-.01, .6, false}, {0, .6, false}, {math.SmallestNonzeroFloat64, .6, true},
		{math.Nextafter(.6, 0), .6, true}, {.6, .6, false}, {math.Nextafter(.6, 1), .6, false},
		{.25, .2, false}, {.25, .25, false}, {.25, .3, true}, {.75, .9, true},
	} {
		t.Run(nameValue(tc.provisional)+"_of_"+nameValue(tc.minimum), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) {
				c := d["confidence"].(map[string]any)
				c["provisional_min_coverage"], c["min_coverage"] = tc.provisional, tc.minimum
			})
			if !tc.valid {
				refusedField(t, dir, "scoring.json", "confidence.provisional_min_coverage")
				return
			}
			c := load(t, dir)
			if c.Scoring.Confidence.ProvisionalMinCoverage != tc.provisional {
				t.Fatal("provisional threshold was replaced by a default")
			}
			snapshot := c.Snapshot()
			c.Scoring.Confidence.ProvisionalMinCoverage = -1
			snapshot.Scoring.Confidence.ProvisionalMinCoverage = -1
			if c.Snapshot().Scoring.Confidence.ProvisionalMinCoverage != tc.provisional {
				t.Fatal("provisional threshold was not detached")
			}
		})
	}
}

func TestRaidOneScoringSeed(t *testing.T) {
	s := load(t, seedDir).Scoring
	got := scoringObject(t, s)
	for key, want := range map[string]any{
		"version": "1.3.0", "rounding_step": .1, "gcd_ms": float64(1500),
		"gcd_energy_ms": float64(1000), "new_at_60_seconds": nil,
	} {
		if actual, ok := got[key]; !ok || !reflect.DeepEqual(actual, want) {
			t.Errorf("%s = %v (present=%v), want %v", key, actual, ok, want)
		}
	}
	if s.Healer.LowHealthFraction == nil || *s.Healer.LowHealthFraction != .5 {
		t.Error("healer.low_health_fraction must be .5")
	}
	if s.Death.LostBossGapMs == nil || *s.Death.LostBossGapMs != 4000 {
		t.Error("death.lost_boss_gap_ms must be 4000")
	}
	if s.Death.SummaryWindowMs == nil || *s.Death.SummaryWindowMs != 10000 {
		t.Error("death.summary_window_ms must be 10000")
	}
	if s.Confidence.ProvisionalMinCoverage != .15 || s.Confidence.MinCoverage != .6 {
		t.Errorf("confidence thresholds: %+v", s.Confidence)
	}
	wantWeights := map[string]map[string]float64{
		"output":      {"boss_damage": 100}, // Same measure for DPS and tank peers.
		"activity":    {"active_time": 100},
		"mechanics":   {"deaths": 50, "avoidable": 30, "boss_swings": 20},
		"preparation": {"stock": 30, "bag_gear": 30, "used": 40},
	}
	if !reflect.DeepEqual(s.MeasureWeights, wantWeights) {
		t.Errorf("measure_weights = %v, want %v", s.MeasureWeights, wantWeights)
	}
	wantPenalties := map[string]float64{"owned_death": 40, "boss_swing_taken": 2, "better_item_in_bags": 25}
	if !reflect.DeepEqual(s.CountPenalties, wantPenalties) {
		t.Errorf("count_penalties = %v, want %v", s.CountPenalties, wantPenalties)
	}
	for _, provenance := range []string{
		"R6 and R7 on main 6208b16", "Raid 1 gets a score", "architect ruling",
		"starting values", "guesses", "not calibrated", "PLAN 4A row 7",
		"second-log cast-gap peaks", "PowerType 3", "prep-report label, not a score",
	} {
		if !strings.Contains(s.Source, provenance) {
			t.Errorf("source lacks provenance %q", provenance)
		}
	}
}

func nameValue(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
