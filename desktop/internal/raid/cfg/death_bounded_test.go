package cfg_test

import (
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPulledAggroConfig(t *testing.T) {
	for _, tc := range []struct {
		value any
		valid bool
	}{
		{-1, false}, {0, false}, {1, true}, {10000, true},
		{int64(math.MaxInt64), true}, {uint64(math.MaxInt64) + 1, false},
		{nil, false}, {1.5, false}, {"1", false}, {true, false},
	} {
		t.Run(nameValue(tc.value), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { setScoringValue(d, "death.pulled_aggro_ms", tc.value) })
			if !tc.valid {
				refusedField(t, dir, "scoring.json", "death.pulled_aggro_ms")
				return
			}
			got := reflect.ValueOf(load(t, dir).Scoring.Death).FieldByName("PulledAggroMs")
			if !got.IsValid() || got.Kind() != reflect.Int64 || nameValue(got.Int()) != nameValue(tc.value) {
				t.Fatalf("pulled_aggro_ms did not preserve %v as int64", tc.value)
			}
		})
	}
	t.Run("required", func(t *testing.T) {
		dir := copyValidationData(t)
		change(t, dir, "scoring.json", func(d map[string]any) { delete(d["death"].(map[string]any), "pulled_aggro_ms") })
		err := refused(t, dir, "scoring.json")
		if !strings.Contains(err.Error(), "death.pulled_aggro_ms is required") {
			t.Fatalf("required-field diagnostic: %v", err)
		}
	})
}

func TestPulledAggroSeedAndCopies(t *testing.T) {
	c := load(t, seedDir)
	if c.Scoring.Version != "1.5.0" || scoringObject(t, c.Scoring.Death)["pulled_aggro_ms"] != float64(10000) {
		t.Fatalf("missing scoring 1.5.0 pulled-aggro seed: %+v", c.Scoring.Death)
	}
	if !strings.Contains(c.Scoring.Source, "R6: rules 3 and 4 bounded") || !strings.Contains(c.Scoring.Source, "death.pulled_aggro_ms=10000: architect guess") {
		t.Fatal("bounded aggro value lacks its guess provenance")
	}
	want := c.Snapshot().Scoring.Death
	snapshot := c.Snapshot()
	for _, d := range []reflect.Value{reflect.ValueOf(&c.Scoring.Death).Elem(), reflect.ValueOf(&snapshot.Scoring.Death).Elem()} {
		field := d.FieldByName("PulledAggroMs")
		if !field.IsValid() || field.Kind() != reflect.Int64 {
			t.Fatal("missing int64 PulledAggroMs")
		}
		field.SetInt(7)
		if !reflect.DeepEqual(c.Snapshot().Scoring.Death, want) {
			t.Fatal("pulled aggro field aliases validated snapshot")
		}
	}
}

func TestRaidwideWindowBound(t *testing.T) {
	for _, window := range []int64{4000, 10000, 12000} {
		for _, value := range []int64{-1, 0, 1, window - 1, window, window + 1} {
			t.Run(nameValue(value)+"_of_"+nameValue(window), func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, "scoring.json", func(d map[string]any) {
					setScoringValue(d, "death.window_ms", window)
					setScoringValue(d, "death.raidwide_window_ms", value)
				})
				if value <= 0 || value > window {
					refusedField(t, dir, "scoring.json", "death.raidwide_window_ms")
				} else if got := load(t, dir).Scoring.Death.RaidwideWindowMs; got != value {
					t.Fatalf("raidwide_window_ms=%d, want %d", got, value)
				}
			})
		}
	}
}

func TestRaidOneProsePins(t *testing.T) {
	const gcdSource = "gcd_ms=1500 and gcd_energy_ms=1000: architect reports second-log cast-gap peaks at 1.4-1.5 s and 0.9-1.0 s respectively; energy interval for cast PowerType 3."
	if strings.Count(load(t, seedDir).Scoring.Source, gcdSource) != 1 {
		t.Fatal("GCD source statement changed; reconcile with the architect")
	}
	b, err := os.ReadFile(filepath.Join(seedDir, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, sentence := range []string{
		"The validation rule stays `0 < provisional_min_coverage < min_coverage`.",
		"Below 0.15, or without a scored fight, the night score is null.",
	} {
		if strings.Count(string(b), sentence) != 1 {
			t.Fatalf("threshold prose changed: %q", sentence)
		}
	}
}
