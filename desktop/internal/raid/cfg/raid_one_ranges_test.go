package cfg_test

import (
	"math"
	"strings"
	"testing"
)

func setScoringValue(d map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		d = d[part].(map[string]any)
	}
	d[parts[len(parts)-1]] = value
}

func TestRaidOneScalarRanges(t *testing.T) {
	type boundary struct {
		value any
		valid bool
	}
	for _, tc := range []struct {
		path   string
		values []boundary
	}{
		{"rounding_step", []boundary{{-.1, false}, {0, false}, {.001, true}, {100, true}, {100.001, false}, {nil, true}}},
		{"healer.low_health_fraction", []boundary{{-.1, false}, {0, false}, {.001, true}, {1, true}, {1.001, false}, {nil, true}}},
	} {
		for _, b := range tc.values {
			t.Run(tc.path+"/"+nameValue(b.value), func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, "scoring.json", func(d map[string]any) { setScoringValue(d, tc.path, b.value) })
				if b.valid {
					load(t, dir)
				} else {
					refusedField(t, dir, "scoring.json", tc.path)
				}
			})
		}
	}
	// Durations retain the existing positive-int64 domain, not a new guessed
	// tuning ceiling. Check both sides of zero and of the decoder's upper bound.
	for _, path := range []string{"gcd_ms", "death.lost_boss_gap_ms", "death.summary_window_ms"} {
		for _, b := range []boundary{{-1, false}, {0, false}, {1, true}, {int64(math.MaxInt64), true}, {uint64(math.MaxInt64) + 1, false}, {nil, true}} {
			t.Run(path+"/"+nameValue(b.value), func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, "scoring.json", func(d map[string]any) { setScoringValue(d, path, b.value) })
				if b.valid {
					load(t, dir)
				} else {
					refusedField(t, dir, "scoring.json", path)
				}
			})
		}
	}
}

func TestRaidOneMeasureWeightRanges(t *testing.T) {
	for part, measures := range map[string][]string{
		"output": {"boss_damage"}, "activity": {"active_time"},
		"mechanics":   {"deaths", "avoidable", "boss_swings"},
		"preparation": {"stock", "bag_gear", "used"},
	} {
		for _, measure := range measures {
			for _, value := range []float64{-.01, 0, 100, 100.01} {
				path := "measure_weights." + part + "." + measure
				t.Run(path+"/"+nameValue(value), func(t *testing.T) {
					dir := copyValidationData(t)
					change(t, dir, "scoring.json", func(d map[string]any) {
						weights := d["measure_weights"].(map[string]any)[part].(map[string]any)
						weights["boundary_control"] = 1 // Keep total positive; isolate this component's guard.
						weights[measure] = value
					})
					if value >= 0 && value <= 100 {
						load(t, dir)
					} else {
						refusedField(t, dir, "scoring.json", path)
					}
				})
			}
		}
	}
}

func TestRaidOneCountPenaltyRanges(t *testing.T) {
	for _, id := range []string{"owned_death", "boss_swing_taken", "better_item_in_bags"} {
		for _, value := range []float64{-.01, 0, 100, 100.01} {
			path := "count_penalties." + id
			t.Run(path+"/"+nameValue(value), func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, "scoring.json", func(d map[string]any) { setScoringValue(d, path, value) })
				if value >= 0 && value <= 100 {
					load(t, dir)
				} else {
					refusedField(t, dir, "scoring.json", path)
				}
			})
		}
	}
}
