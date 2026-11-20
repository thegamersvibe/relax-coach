package cfg_test

import (
	"strings"
	"testing"
)

var deathPredicateValues = map[string]any{
	"full_health_fraction": .9,
	"min_health_samples":   float64(2),
	"raidwide_share":       .6,
	"raidwide_window_ms":   float64(2000),
}

func TestDeathPredicatesDecode(t *testing.T) {
	for key, value := range deathPredicateValues {
		t.Run(key, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { setScoringValue(d, "death."+key, value) })
			got := scoringObject(t, load(t, dir).Scoring.Death)
			if got[key] != value {
				t.Fatalf("%s = %v, want %v", key, got[key], value)
			}
		})
		t.Run(key+"_required", func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { delete(d["death"].(map[string]any), key) })
			err := refused(t, dir, "scoring.json")
			if !strings.Contains(err.Error(), "death."+key+" is required") {
				t.Fatalf("missing field diagnostic: %v", err)
			}
		})
		t.Run(key+"_nonnull", func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { setScoringValue(d, "death."+key, nil) })
			err := refused(t, dir, "scoring.json")
			if !strings.Contains(err.Error(), "death."+key+" cannot be null") {
				t.Fatalf("null field diagnostic: %v", err)
			}
		})
	}
}
