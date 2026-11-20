package cfg_test

import (
	"fmt"
	"strings"
	"testing"
)

func TestRound2PartRanges(t *testing.T) {
	for _, role := range []string{"dps", "healer", "tank"} {
		for _, part := range []string{"output", "preparation"} {
			for _, value := range []int{-1, 0, 100, 101} {
				t.Run(fmt.Sprintf("%s/%s/%d", role, part, value), func(t *testing.T) {
					dir := copyValidationData(t)
					change(t, dir, "scoring.json", func(d map[string]any) {
						w := map[string]any{"output": 0, "activity": 0, "mechanics": 0, "utility": 0, "preparation": 0}
						w[part] = value
						if value == -1 {
							// Sum remains 100 and every OTHER part is in range, so
							// neither the sum check nor another range masks this one.
							w["activity"], w["utility"] = 51, 50
						} else if value <= 100 {
							w["activity"] = 100 - value
						}
						d["weights"].(map[string]any)[role] = w
					})
					if value < 0 || value > 100 {
						refusedField(t, dir, "scoring.json", "weights."+role+"."+part)
					} else {
						load(t, dir)
					}
				})
			}
		}
	}
}

func TestRound2RoundingStep(t *testing.T) {
	for _, value := range []float64{-1, 0, .01, 100, 100.01} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d["rounding_step"] = value })
			if value <= 0 || value > 100 {
				refusedField(t, dir, "scoring.json", "rounding_step")
			} else if c := load(t, dir); c.Scoring.RoundingStep == nil || *c.Scoring.RoundingStep != value {
				t.Fatal("explicit rounding step was lost")
			}
		})
	}
}

func TestRound2SupportScope(t *testing.T) {
	for _, scope := range []string{"unknown", "personal", "raid"} {
		for _, credit := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", scope, credit), func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, "support.json", func(d map[string]any) {
					e := d["effects"].([]any)[0].(map[string]any)
					e["scope"], e["credit"], e["verified"], e["stacks"] = scope, credit, true, false
				})
				if credit && scope != "raid" {
					refusedField(t, dir, "support.json", "effects[0].credit")
				} else {
					load(t, dir)
				}
			})
		}
	}
}

func TestRound2DuplicateProfileSupport(t *testing.T) {
	dir := copyValidationData(t)
	change(t, dir, "specs.json", func(d map[string]any) {
		p := d["profiles"].([]any)[0].(map[string]any)
		s := p["support"].([]any)
		p["support"] = []any{s[0], s[0]} // Both references resolve; only uniqueness can refuse this.
	})
	refusedField(t, dir, "specs.json", "profiles[0].support")
}

func TestRound2TargetSelectors(t *testing.T) {
	for _, tc := range []struct {
		name, field string
		edit        func([]any) []any
	}{
		{"zero", "[0].id", func(u []any) []any { u[0].(map[string]any)["id"] = 0; return u }},
		{"negative", "[0].id", func(u []any) []any { u[0].(map[string]any)["id"] = -1; return u }},
		{"missing", "[0].id", func(u []any) []any { delete(u[0].(map[string]any), "id"); return u }},
		{"duplicate", "[1].id", func(u []any) []any { u[1].(map[string]any)["id"] = 7; return u }},
		{"source", "[0].source", func(u []any) []any { u[0].(map[string]any)["source"] = ""; return u }},
		{"valid", "", func(u []any) []any { return u }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "encounters-enabled.json", func(d map[string]any) {
				r := syntheticRule()
				u := []any{map[string]any{"id": 7, "source": "synthetic"}, map[string]any{"id": 8, "name": "different label", "source": "synthetic"}}
				r["target_rule"] = map[string]any{"mode": "boss_and_units", "units": tc.edit(u)}
				d["encounters"] = []any{r}
			})
			if tc.field != "" {
				refusedField(t, dir, "encounters-enabled.json", "encounters[0].target_rule.units"+tc.field)
			} else {
				load(t, dir)
			}
		})
	}
}

func TestRound2EncounterSpellIdentities(t *testing.T) {
	for _, tc := range []struct{ file, field string }{
		{"encounters-candidate.json", "encounters[0].abilities[0]"},
		{"encounters-enabled.json", "encounters[0].mechanics[0].spell"},
	} {
		for _, same := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/same_%v", tc.file, same), func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, "specs.json", func(d map[string]any) {
					d["profiles"].([]any)[0].(map[string]any)["spells"] = []any{sourcedSpell("First", 77)}
				})
				change(t, dir, tc.file, func(d map[string]any) {
					name := "Second"
					if same {
						name = " first "
					}
					if tc.file == "encounters-candidate.json" {
						d["encounters"].([]any)[0].(map[string]any)["abilities"] = []any{sourcedSpell(name, 77)}
					} else {
						r := syntheticRule()
						r["mechanics"].([]any)[0].(map[string]any)["spell"] = sourcedSpell(name, 77)
						d["encounters"] = []any{r}
					}
				})
				if same {
					load(t, dir)
				} else {
					err := refused(t, dir, tc.file)
					for _, path := range []string{tc.field + ".id", "specs.json: profiles[0].spells[0].name"} {
						if !strings.Contains(err.Error(), path) {
							t.Fatalf("identity error lacks %s: %v", path, err)
						}
					}
				}
			})
		}
	}
}
