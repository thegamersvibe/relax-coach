package cfg_test

import (
	"fmt"
	"testing"
)

func TestValidateScoring(t *testing.T) {
	cases := []struct {
		name string
		edit func(map[string]any)
	}{
		{"weight_sum", func(d map[string]any) { d["weights"].(map[string]any)["dps"].(map[string]any)["output"] = 99 }},
		{"negative_weight", func(d map[string]any) { d["weights"].(map[string]any)["healer"].(map[string]any)["output"] = -1 }},
		{"zero_kill", func(d map[string]any) { d["night"].(map[string]any)["kill_weight"] = 0 }},
		{"wipe_not_lower", func(d map[string]any) { d["night"].(map[string]any)["wipe_weight"] = 1 }},
		{"trash_above_cap", func(d map[string]any) { d["night"].(map[string]any)["trash_share"] = .1 }},
		{"boss_share", func(d map[string]any) { d["night"].(map[string]any)["min_boss_share"] = 1 }},
		{"coverage", func(d map[string]any) { d["confidence"].(map[string]any)["min_coverage"] = 1.1 }},
		{"negative_gap", func(d map[string]any) { d["fights"].(map[string]any)["pull_gap_ms"] = -1 }},
		{"negative_tail", func(d map[string]any) { d["fights"].(map[string]any)["tail_ms"] = -1 }},
		{"short_weight", func(d map[string]any) { d["confidence"].(map[string]any)["short_fight_weight"] = 0 }},
		{"single_baseline", func(d map[string]any) { d["confidence"].(map[string]any)["min_measurable_players"] = 1 }},
		{"minimum_order", func(d map[string]any) { d["confidence"].(map[string]any)["min_measurable_players"] = 4 }},
		{"role_share", func(d map[string]any) { d["fights"].(map[string]any)["tank_min_boss_swing_share"] = 2 }},
		{"healer_ratio", func(d map[string]any) { d["fights"].(map[string]any)["healer_min_healing_damage_ratio"] = -1 }},
		{"boss_selection", func(d map[string]any) { d["fights"].(map[string]any)["boss_unit_selection"] = "first_name" }},
		{"ratio_unsorted", func(d map[string]any) { d["ratio_points"].([]any)[1].(map[string]any)["ratio"] = .1 }},
		{"ratio_score", func(d map[string]any) { d["ratio_points"].([]any)[1].(map[string]any)["score"] = 101 }},
		{"ratio_empty", func(d map[string]any) { d["ratio_points"] = []any{} }},
		{"healer_share", func(d map[string]any) { d["healer"].(map[string]any)["team_share"] = .8 }},
		{"death_window", func(d map[string]any) { d["death"].(map[string]any)["window_ms"] = 0 }},
		{"death_share", func(d map[string]any) { d["death"].(map[string]any)["wipe_dead_share"] = 2 }},
		{"null_optional_negative", func(d map[string]any) { d["new_at_60_seconds"] = -1 }},
		{"bad_optional_health", func(d map[string]any) { d["healer"].(map[string]any)["low_health_fraction"] = 2 }},
		{"bad_measure_weight", func(d map[string]any) { d["measure_weights"] = map[string]any{"output": map[string]any{"damage": -1}} }},
		{"bad_count_penalty", func(d map[string]any) { d["count_penalties"] = map[string]any{"owned_death": -1} }},
		{"bad_assignment", func(d map[string]any) { d["assignments"].([]any)[0].(map[string]any)["output_to_utility"] = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "scoring.json", tc.edit)
			refused(t, dir, "scoring.json")
		})
	}
	// Zero trash and explicit false are legitimate settings, not missing values.
	dir := copyData(t)
	change(t, dir, "scoring.json", func(d map[string]any) {
		d["night"].(map[string]any)["trash_share"] = 0
		d["fights"].(map[string]any)["healer_strictly_greater"] = false
		d["gcd_ms"] = 1250
		d["rounding_step"] = .5
	})
	c := load(t, dir)
	if c.Scoring.Night.TrashShare != 0 || c.Scoring.Fights.HealerStrictlyGreater || *c.Scoring.GCDMs != 1250 {
		t.Fatal("explicit values lost")
	}
}

func TestValidateProfiles(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"missing":            func(d map[string]any) { d["profiles"] = d["profiles"].([]any)[1:] },
		"duplicate":          func(d map[string]any) { p := d["profiles"].([]any); p[1] = p[0] },
		"extra":              func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["id"] = "WARRIOR.invented" },
		"wrong_class":        func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["class_name"] = "ROGUE" },
		"empty_roles":        func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["roles"] = []any{} },
		"unknown_role":       func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["roles"] = []any{"wizard"} },
		"empty_source":       func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["source"] = "" },
		"empty_comparison":   func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["comparison"] = "" },
		"unknown_pet_policy": func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["pets"] = "guess" },
		"unknown_support":    func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["support"] = []any{"imaginary_buff"} },
	} {
		t.Run(name, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "specs.json", edit)
			refused(t, dir, "specs.json")
		})
	}
	for name, edit := range map[string]func(map[string]any){
		"missing_id":   func(d map[string]any) { d["classes"].(map[string]any)["WARRIOR"] = []any{} },
		"duplicate_id": func(d map[string]any) { p := d["classes"].(map[string]any)["WARRIOR"].([]any); p[1] = p[0] },
	} {
		t.Run(name, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "spec-ids.json", edit)
			refused(t, dir, "spec-ids.json")
		})
	}
}

func TestSpellIDRequiresProvenance(t *testing.T) {
	for _, field := range []string{"spells", "cast_signatures"} {
		for _, source := range []any{nil, map[string]any{"kind": "memory", "note": "guess"}, map[string]any{"kind": "real_log", "note": ""}} {
			t.Run(fmt.Sprint(field, source), func(t *testing.T) {
				dir := copyData(t)
				change(t, dir, "specs.json", func(d map[string]any) {
					d["profiles"].([]any)[0].(map[string]any)[field] = []any{map[string]any{"name": "Synthetic spell", "id": 123456, "source": source}}
				})
				refused(t, dir, "specs.json")
			})
		}
	}
	for _, kind := range []string{"real_log", "forever_source"} {
		t.Run(kind, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "specs.json", func(d map[string]any) {
				d["profiles"].([]any)[0].(map[string]any)["spells"] = []any{map[string]any{"name": "Synthetic spell", "id": 123456, "source": map[string]any{"kind": kind, "note": "synthetic provenance test, not a production mapping"}}}
			})
			load(t, dir)
		})
	}
}

func TestValidateCatalogues(t *testing.T) {
	cases := []struct {
		file string
		edit func(map[string]any)
	}{
		{"support.json", func(d map[string]any) { d["effects"].([]any)[0].(map[string]any)["scope"] = "unknown" }},
		{"support.json", func(d map[string]any) { e := d["effects"].([]any); e[1] = e[0] }},
		{"support.json", func(d map[string]any) {
			e := d["effects"].([]any)[0].(map[string]any)
			e["scope"] = "personal"
			e["credit"] = true
		}},
		{"support.json", func(d map[string]any) {
			d["effects"].([]any)[0].(map[string]any)["spell"].(map[string]any)["id"] = 123456
		}},
		{"encounters-candidate.json", func(d map[string]any) { d["encounters"].([]any)[0].(map[string]any)["source"] = "" }},
		{"encounters-enabled.json", func(d map[string]any) { d["default_target_rule"] = "all_damage" }},
		{"consumables.json", func(d map[string]any) { d["categories"] = []any{"potion", "potion"} }},
		{"realm-labels.json", func(d map[string]any) { d["labels"] = []any{"PvP-EU", "PvP-EU"} }},
		{"instances.json", func(d map[string]any) { d["instances"].([]any)[0].(map[string]any)["kind"] = "guess" }},
		{"instances.json", func(d map[string]any) { d["instances"].([]any)[0].(map[string]any)["id"] = 0 }},
		{"instances.json", func(d map[string]any) { i := d["instances"].([]any); d["instances"] = append(i, i[0]) }},
	}
	for i, tc := range cases {
		t.Run(fmt.Sprint(tc.file, i), func(t *testing.T) { dir := copyData(t); change(t, dir, tc.file, tc.edit); refused(t, dir, tc.file) })
	}
}

func syntheticRule() map[string]any {
	return map[string]any{"id": 77, "name": "Synthetic encounter", "source": "synthetic log fixture", "boss_units": []any{map[string]any{"id": 123456, "source": "synthetic log fixture"}}, "mechanics": []any{map[string]any{"id": "synthetic_hit", "spell": map[string]any{"name": "Synthetic hit"}, "avoidable": true, "roles": []any{"dps"}, "affected_players": "non-tanks in the synthetic fixture", "valid_response": "leave the synthetic hazard", "evidence_source": map[string]any{"kind": "real_log", "note": "synthetic fixture standing in for validated log evidence"}}}}
}
func TestEnabledRulesRequireEvidence(t *testing.T) {
	for name, edit := range map[string]func(map[string]any){
		"no_source":             func(r map[string]any) { r["source"] = "" },
		"empty_selector":        func(r map[string]any) { r["boss_units"] = []any{map[string]any{"source": "synthetic evidence"}} },
		"no_selector_source":    func(r map[string]any) { r["boss_units"].([]any)[0].(map[string]any)["source"] = "" },
		"unidentified_players":  func(r map[string]any) { r["mechanics"].([]any)[0].(map[string]any)["affected_players"] = "" },
		"unidentified_response": func(r map[string]any) { r["mechanics"].([]any)[0].(map[string]any)["valid_response"] = "" },
		"datamine_only": func(r map[string]any) {
			r["mechanics"].([]any)[0].(map[string]any)["evidence_source"].(map[string]any)["kind"] = "forever_source"
		},
		"unknown_target_mode": func(r map[string]any) { r["target_rule"] = map[string]any{"mode": "all", "units": []any{}} },
		"no_added_targets":    func(r map[string]any) { r["target_rule"] = map[string]any{"mode": "boss_and_units", "units": []any{}} },
	} {
		t.Run(name, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "encounters-enabled.json", func(d map[string]any) { r := syntheticRule(); edit(r); d["encounters"] = []any{r} })
			refused(t, dir, "encounters-enabled.json")
		})
	}
	dir := copyData(t)
	change(t, dir, "encounters-enabled.json", func(d map[string]any) { d["encounters"] = []any{syntheticRule()} })
	load(t, dir)
}
