package cfg_test

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestActivityCastCapConfig(t *testing.T) {
	for _, tc := range []struct {
		value any
		valid bool
	}{
		{-1, false}, {0, false}, {1, true}, {10000, true},
		{int64(math.MaxInt64), true}, {uint64(math.MaxInt64) + 1, false},
		{nil, false}, {1.5, false}, {"10000", false}, {true, false},
	} {
		t.Run(nameValue(tc.value), func(t *testing.T) {
			dir := copyScoringData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d["activity_cast_cap_ms"] = tc.value })
			if !tc.valid {
				refusedField(t, dir, "scoring.json", "activity_cast_cap_ms")
				return
			}
			got := reflect.ValueOf(load(t, dir).Scoring).FieldByName("ActivityCastCapMs")
			if !got.IsValid() || got.Kind() != reflect.Int64 || nameValue(got.Int()) != nameValue(tc.value) {
				t.Fatalf("cast cap did not preserve %v as int64", tc.value)
			}
		})
	}
	for _, mode := range []string{"required", "case", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			dir := copyScoringData(t)
			change(t, dir, "scoring.json", func(d map[string]any) {
				delete(d, "activity_cast_cap_ms")
				if mode == "case" {
					d["Activity_cast_cap_ms"] = 10000
				}
				if mode == "unknown" {
					d["activity_cast_cap_ms"] = 10000
					d["activity_cast_cap_ms_extra"] = 10000
				}
			})
			err := refused(t, dir, "scoring.json")
			if !strings.Contains(strings.ToLower(err.Error()), "activity_cast_cap_ms") {
				t.Fatal(err)
			}
		})
	}
}

func TestActivityCastCapSeedAndCopies(t *testing.T) {
	c := load(t, seedDir)
	if c.Scoring.Version != "1.5.0" || scoringObject(t, c.Scoring)["activity_cast_cap_ms"] != float64(10000) {
		t.Fatal("missing scoring 1.5.0 cast cap seed")
	}
	if !strings.Contains(c.Scoring.Source, "activity_cast_cap_ms=10000") {
		t.Fatal("cast cap lacks provenance")
	}
	want := c.Snapshot().Scoring
	snapshot := c.Snapshot()
	for _, s := range []reflect.Value{reflect.ValueOf(&c.Scoring).Elem(), reflect.ValueOf(&snapshot.Scoring).Elem()} {
		field := s.FieldByName("ActivityCastCapMs")
		if !field.IsValid() || field.Kind() != reflect.Int64 {
			t.Fatal("missing int64 ActivityCastCapMs")
		}
		field.SetInt(7)
		if !reflect.DeepEqual(c.Snapshot().Scoring, want) {
			t.Fatal("cast cap aliases validated snapshot")
		}
	}
}
