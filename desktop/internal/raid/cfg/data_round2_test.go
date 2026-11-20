package cfg_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDataRound2MechanicRoles(t *testing.T) {
	for _, tc := range []struct {
		name  string
		roles []string
		valid bool
	}{
		{"every_role", []string{}, true},
		{"dps", []string{"dps"}, true},
		{"healer", []string{"healer"}, true},
		{"tank", []string{"tank"}, true},
		{"explicit_all", []string{"dps", "healer", "tank"}, true},
		{"unknown", []string{"wizard"}, false},
		{"duplicate", []string{"dps", "dps"}, false},
		{"normalized_duplicate", []string{"dps", " DPS "}, false},
		{"null", nil, false},
		{"missing", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "encounters-enabled.json", func(d map[string]any) {
				r := syntheticRule()
				m := r["mechanics"].([]any)[0].(map[string]any)
				if tc.name == "missing" {
					delete(m, "roles")
				} else {
					m["roles"] = tc.roles
				}
				d["encounters"] = []any{r}
			})
			if !tc.valid {
				refusedField(t, dir, "encounters-enabled.json", "encounters[0].mechanics[0].roles")
				return
			}
			got := load(t, dir).Encounters.Encounters[0].Mechanics[0].Roles
			if strings.Join(got, ",") != strings.Join(tc.roles, ",") {
				t.Fatalf("mechanic roles changed: got %v, want %v", got, tc.roles)
			}
		})
	}
}

func TestDataRound2NULKeyLimit(t *testing.T) {
	for _, tc := range []struct{ name, key, shown string }{
		{"short", "\x00" + strings.Repeat("x", 62), "\x00" + strings.Repeat("x", 62)},
		{"exact", "\x00" + strings.Repeat("x", 63), "\x00" + strings.Repeat("x", 63)},
		{"over", "\x00" + strings.Repeat("x", 64), "\x00" + strings.Repeat("x", 63) + "..."},
		{"long", "\x00" + strings.Repeat("x", 4096), "\x00" + strings.Repeat("x", 63) + "..."},
		{"late_nul", strings.Repeat("x", 64) + "\x00tail", strings.Repeat("x", 64) + "..."},
		{"byte_not_rune_limit", "\x00" + strings.Repeat("é", 32), "\x00" + strings.Repeat("é", 31) + "\xc3..."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyValidationData(t)
			replaceJSONText(t, dir, "scoring.json", `"source":`, `"`+tc.key+`":`)
			err := refused(t, dir, "scoring.json")
			want := "scoring.json: $[" + strconv.Quote(tc.shown) + "]:"
			if !strings.HasPrefix(err.Error(), want) || strings.Contains(err.Error(), "\x00") {
				t.Fatalf("NUL key path must cap authored bytes at 64 and quote them: got %q, want prefix %q", err, want)
			}
		})
	}
}

func TestDataRound2ScoringSchema(t *testing.T) {
	for _, schema := range []int{1, 2, 3} {
		t.Run(nameValue(schema), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d["schema"] = schema })
			if schema == 2 {
				if got := load(t, dir).Scoring.Schema; got != 2 {
					t.Fatalf("scoring schema = %d, want 2", got)
				}
			} else {
				refusedField(t, dir, "scoring.json", "schema")
			}
		})
	}
	t.Run("old_shape", func(t *testing.T) {
		dir := copyValidationData(t)
		change(t, dir, "scoring.json", func(d map[string]any) {
			d["schema"] = 1
			delete(d, "gcd_energy_ms")
			delete(d["confidence"].(map[string]any), "provisional_min_coverage")
			for _, key := range []string{"full_health_fraction", "min_health_samples", "raidwide_share", "raidwide_window_ms"} {
				delete(d["death"].(map[string]any), key)
			}
		})
		refusedField(t, dir, "scoring.json", "schema")
	})
}

func TestDataRound2MeasureTotals(t *testing.T) {
	for _, deaths := range []float64{49, 50, 51} {
		t.Run(nameValue(deaths), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) {
				setScoringValue(d, "measure_weights.mechanics.deaths", deaths)
			})
			if deaths == 50 {
				load(t, dir)
				return
			}
			err := refused(t, dir, "scoring.json")
			if !strings.Contains(err.Error(), "measure_weights.mechanics") || !strings.Contains(err.Error(), "sum to 100") {
				t.Fatalf("missing measure-total field/diagnosis: %v", err)
			}
		})
	}
	for _, name := range []string{"no_parts", "absent_part", "empty_part"} {
		t.Run(name, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) {
				switch name {
				case "no_parts":
					d["measure_weights"] = map[string]any{}
				case "absent_part":
					delete(d["measure_weights"].(map[string]any), "mechanics")
				case "empty_part":
					d["measure_weights"].(map[string]any)["mechanics"] = map[string]any{}
				}
			})
			if name == "empty_part" {
				refusedField(t, dir, "scoring.json", "measure_weights.mechanics")
			} else {
				load(t, dir)
			}
		})
	}
}

func TestDataRound2ProvisionalDocumentation(t *testing.T) {
	for _, path := range []string{
		filepath.Join(seedDir, "README.md"),
		filepath.Join(seedDir, "..", "..", "docs", "raid", "R5-REVIEW-EVIDENCE.md"),
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(b)
		if !strings.Contains(text, "0.15") || strings.Contains(text, "0.25") || strings.Contains(text, "25 hundredths") {
			t.Errorf("%s: provisional floor must consistently document 0.15", filepath.Base(path))
		}
	}
}
