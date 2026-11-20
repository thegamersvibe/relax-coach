package cfg_test

import (
	"fmt"
	"testing"
)

func TestReviewRanges(t *testing.T) {
	for _, value := range []int{-1, 0, 1, 4} {
		t.Run(fmt.Sprintf("raid_cutoff_%d", value), func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "instances.json", func(d map[string]any) { d["raid_group_size_above"] = value })
			refusedField(t, dir, "instances.json", "raid_group_size_above")
		})
	}
	for _, value := range []int{5, 6, 40} {
		dir := copyData(t)
		change(t, dir, "instances.json", func(d map[string]any) { d["raid_group_size_above"] = value })
		load(t, dir)
	}
	for _, tc := range []struct {
		group, key string
		value      any
	}{
		{"fights", "tail_ms", 10001},
		{"night", "kill_weight", 1.01}, {"night", "kill_weight", 1e300},
		{"measure_weights", "output", map[string]any{"damage": 100.01}},
		{"measure_weights", "output", map[string]any{"damage": 1e300}},
	} {
		t.Run(tc.group+"/"+tc.key+fmt.Sprint(tc.value), func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d[tc.group].(map[string]any)[tc.key] = tc.value })
			refusedField(t, dir, "scoring.json", tc.group+"."+tc.key)
		})
	}
	dir := copyData(t)
	change(t, dir, "scoring.json", func(d map[string]any) {
		d["fights"].(map[string]any)["tail_ms"] = 10000
		d["night"].(map[string]any)["wipe_weight"] = 0
		d["measure_weights"] = map[string]any{"output": map[string]any{"damage": 100, "other": 0}}
	})
	load(t, dir)
}

func TestReviewScoringFields(t *testing.T) {
	for _, tc := range []struct {
		group, key string
		value      any
	}{
		{"night", "kill_weight", 0}, {"night", "wipe_weight", -1},
		{"confidence", "low_confidence_fight_ms", 0}, {"confidence", "short_fight_weight", 1.1},
		{"death", "one_shot_window_ms", 10001}, {"death", "unhealed_min_ms", 10001}, {"death", "lost_boss_gap_ms", 0},
		{"fights", "pull_gap_ms", 0}, {"fights", "tail_ms", -1}, {"fights", "tank_min_top_ratio", 0},
		{"fights", "tank_min_boss_swing_share", 0}, {"fights", "reentry_gap_ms", 0}, {"fights", "healer_min_healing_damage_ratio", 0},
		{"fights", "boss_unit_selection", "guess"}, {"night", "trash_share", .1}, {"night", "max_trash_share", 2}, {"night", "min_boss_share", 1},
		{"confidence", "min_coverage", 2}, {"confidence", "min_measurable_players", 1}, {"confidence", "min_baseline_players", 1},
		{"healer", "team_share", 2}, {"healer", "individual_share", 2}, {"healer", "unhealed_team_score", -1}, {"healer", "unhealed_output_cap", 101}, {"healer", "low_health_fraction", 0},
		{"death", "window_ms", 0}, {"death", "wipe_dead_share", 0}, {"death", "summary_window_ms", 0},
	} {
		t.Run(tc.group+"/"+tc.key, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d[tc.group].(map[string]any)[tc.key] = tc.value })
			refusedField(t, dir, "scoring.json", tc.group+"."+tc.key)
		})
	}
	for _, key := range []string{"new_at_60_seconds", "gcd_ms", "rounding_step", "count_start_score"} {
		t.Run(key, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d[key] = -1 })
			refusedField(t, dir, "scoring.json", key)
		})
	}
}

func TestReviewSurvivorCases(t *testing.T) {
	for _, tc := range []struct {
		name, file, field string
		edit              func(map[string]any)
	}{
		{"assignment", "scoring.json", "assignments", func(d map[string]any) { a := d["assignments"].([]any); d["assignments"] = append(a, a[0]) }},
		{"measure_part", "scoring.json", "measure_weights", func(d map[string]any) { d["measure_weights"] = map[string]any{"invented": map[string]any{"a": 1}} }},
		{"zero_weights", "scoring.json", "measure_weights", func(d map[string]any) { d["measure_weights"] = map[string]any{"output": map[string]any{"a": 0}} }},
		{"ratio_scores", "scoring.json", "ratio_points", func(d map[string]any) { d["ratio_points"].([]any)[2].(map[string]any)["score"] = 60 }},
		{"ratio_equal", "scoring.json", "ratio_points", func(d map[string]any) { d["ratio_points"].([]any)[1].(map[string]any)["ratio"] = .5 }},
		{"overlap", "support.json", "overlap", func(d map[string]any) { d["overlap"] = "double_credit" }},
		{"profile_append", "specs.json", "profiles", func(d map[string]any) { p := d["profiles"].([]any); d["profiles"] = append(p, p[0]) }},
		{"empty_spells", "specs.json", "spells", func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["spells"] = []any{} }},
		{"encounter_append", "encounters-enabled.json", "encounters", func(d map[string]any) { e := d["encounters"].([]any); d["encounters"] = append(e, e[0]) }},
		{"boss_only_units", "encounters-enabled.json", "target_rule", func(d map[string]any) {
			e := d["encounters"].([]any)[0].(map[string]any)
			e["target_rule"] = map[string]any{"mode": "boss_only", "units": e["boss_units"]}
		}},
		{"candidate_append", "encounters-candidate.json", "encounters", func(d map[string]any) { e := d["encounters"].([]any); d["encounters"] = append(e, e[0]) }},
		{"null_map_value", "scoring.json", "count_penalties", func(d map[string]any) { d["count_penalties"] = map[string]any{"owned_death": nil} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, tc.file, tc.edit)
			refusedField(t, dir, tc.file, tc.field)
		})
	}
}

func TestLowercaseClassRefused(t *testing.T) {
	dir := copyData(t)
	change(t, dir, "spec-ids.json", func(d map[string]any) {
		m := d["classes"].(map[string]any)
		m["warrior"] = m["WARRIOR"]
		delete(m, "WARRIOR")
	})
	change(t, dir, "specs.json", func(d map[string]any) {
		for _, v := range d["profiles"].([]any) {
			p := v.(map[string]any)
			if p["class_name"] == "WARRIOR" {
				p["class_name"] = "warrior"
				p["id"] = "warrior." + p["spec"].(string)
			}
		}
	})
	refusedField(t, dir, "spec-ids.json", "classes")
}
