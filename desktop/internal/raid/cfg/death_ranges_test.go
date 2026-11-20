package cfg_test

import (
	"math"
	"reflect"
	"testing"
)

func TestDeathPredicateRanges(t *testing.T) {
	type edge struct {
		v  any
		ok bool
	}
	for _, key := range []string{"full_health_fraction", "raidwide_share", "min_health_samples", "raidwide_window_ms"} {
		edges := []edge{{-.01, false}, {0, false}, {math.SmallestNonzeroFloat64, true}, {1, true}, {math.Nextafter(1, 2), false}, {"1", false}, {true, false}}
		if key == "min_health_samples" || key == "raidwide_window_ms" {
			maximum := int64(math.MaxInt64)
			if key == "min_health_samples" {
				maximum = int64(math.MaxInt)
			}
			edges = []edge{{-1, false}, {0, false}, {1, true}, {maximum, true}, {uint64(maximum) + 1, false}, {1.5, false}, {"1", false}, {true, false}}
		}
		for _, e := range edges {
			t.Run(key+"/"+nameValue(e.v), func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, "scoring.json", func(d map[string]any) {
					// Keep the duration representation edge independent of its new cross-field bound.
					if key == "raidwide_window_ms" {
						setScoringValue(d, "death.window_ms", int64(math.MaxInt64))
					}
					setScoringValue(d, "death."+key, e.v)
				})
				if !e.ok {
					refusedField(t, dir, "scoring.json", "death."+key)
					return
				}
				got := load(t, dir).Scoring.Death
				fields := map[string]any{"full_health_fraction": got.FullHealthFraction, "raidwide_share": got.RaidwideShare, "min_health_samples": got.MinHealthSamples, "raidwide_window_ms": got.RaidwideWindowMs}
				if nameValue(fields[key]) != nameValue(e.v) {
					t.Fatalf("%s lost configured value: %v != %v", key, fields[key], e.v)
				}
			})
		}
	}
}

func TestDeathPredicateCopies(t *testing.T) {
	c := load(t, seedDir)
	want := load(t, seedDir).Scoring.Death
	snapshot := c.Snapshot()
	for _, d := range []reflect.Value{reflect.ValueOf(&c.Scoring.Death).Elem(), reflect.ValueOf(&snapshot.Scoring.Death).Elem()} {
		for _, field := range []string{"FullHealthFraction", "RaidwideShare", "MinHealthSamples", "RaidwideWindowMs"} {
			f := d.FieldByName(field)
			switch f.Kind() {
			case reflect.Float64:
				f.SetFloat(-1)
			default:
				f.SetInt(-1)
			}
			if !reflect.DeepEqual(c.Snapshot().Scoring.Death, want) {
				t.Fatalf("%s aliases validated snapshot", field)
			}
		}
	}
}
