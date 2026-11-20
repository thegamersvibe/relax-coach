package cfg_test

import (
	"strings"
	"testing"
)

func TestDeathPredicateSeed(t *testing.T) {
	s := load(t, seedDir).Scoring
	if s.Version != "1.4.0" {
		t.Errorf("scoring version = %s, want 1.4.0", s.Version)
	}
	got := scoringObject(t, s.Death)
	for key, want := range deathPredicateValues {
		if got[key] != want {
			t.Errorf("%s = %v, want %v", key, got[key], want)
		}
	}
	for _, text := range []string{"R7: three gaps found by the builder", "confidence.provisional_min_coverage=0.15", "R6: four predicates DESIGN 8 left open", "full_health_fraction=0.9", "min_health_samples=2", "raidwide_share=0.6", "raidwide_window_ms=2000", "architect guesses to tune after raid 1"} {
		if !strings.Contains(s.Source, text) {
			t.Errorf("source lacks %q", text)
		}
	}
}
