package cfg_test

import "testing"

func TestCatalogueFieldErrors(t *testing.T) {
	for _, tc := range []struct {
		file, array, key string
		value            any
	}{
		{"specs.json", "profiles", "source", ""}, {"specs.json", "profiles", "comparison", ""},
		{"specs.json", "profiles", "calibration_note", ""}, {"specs.json", "profiles", "roles", []any{}},
		{"specs.json", "profiles", "pets", "guess"}, {"specs.json", "profiles", "spells", []any{}},
		{"specs.json", "profiles", "resource_exceptions", []any{"a", "a"}}, {"specs.json", "profiles", "support", []any{"missing"}},
		{"support.json", "effects", "source", ""}, {"support.json", "effects", "note", ""}, {"support.json", "effects", "scope", "guess"},
		{"encounters-enabled.json", "encounters", "id", 0}, {"encounters-enabled.json", "encounters", "name", ""}, {"encounters-enabled.json", "encounters", "source", ""},
		{"encounters-candidate.json", "encounters", "name", ""}, {"encounters-candidate.json", "encounters", "raid", ""}, {"encounters-candidate.json", "encounters", "source", ""}, {"encounters-candidate.json", "encounters", "note", ""},
		{"instances.json", "instances", "id", 0}, {"instances.json", "instances", "name", ""}, {"instances.json", "instances", "kind", "guess"}, {"instances.json", "instances", "source", ""},
	} {
		t.Run(tc.file+"/"+tc.key, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, tc.file, func(d map[string]any) { d[tc.array].([]any)[0].(map[string]any)[tc.key] = tc.value })
			refusedField(t, dir, tc.file, tc.array+"[0]."+tc.key)
		})
	}
	for _, key := range []string{"id", "affected_players", "valid_response", "roles", "evidence_source"} {
		t.Run("mechanic/"+key, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "encounters-enabled.json", func(d map[string]any) {
				r := syntheticRule()
				m := r["mechanics"].([]any)[0].(map[string]any)
				switch key {
				case "roles":
					m[key] = []any{"wizard"} // Empty mechanics roles now mean every role.
				case "evidence_source":
					m[key] = map[string]any{"kind": "forever_source", "note": "test"}
				default:
					m[key] = ""
				}
				d["encounters"] = []any{r}
			})
			refusedField(t, dir, "encounters-enabled.json", "encounters[0].mechanics[0]."+key)
		})
	}
	for _, key := range []string{"id", "source"} {
		t.Run("selector/"+key, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "encounters-enabled.json", func(d map[string]any) {
				u := d["encounters"].([]any)[0].(map[string]any)["boss_units"].([]any)[0].(map[string]any)
				if key == "id" {
					u[key] = 0
				} else {
					u[key] = ""
				}
			})
			refusedField(t, dir, "encounters-enabled.json", "encounters[0].boss_units[0]."+key)
		})
	}
	for _, key := range []string{"name", "id", "source"} {
		t.Run("spell/"+key, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "support.json", func(d map[string]any) {
				s := sourcedSpell("synthetic", 77)
				switch key {
				case "name":
					s[key] = ""
				case "id":
					s[key] = -1
				case "source":
					s[key] = nil
				}
				d["effects"].([]any)[0].(map[string]any)["spell"] = s
			})
			refusedField(t, dir, "support.json", "effects[0].spell."+key)
		})
	}
}

func TestNestedDecodeFieldErrors(t *testing.T) {
	for _, tc := range []struct {
		field string
		edit  func(map[string]any)
	}{
		{"$.profiles[0].active_time.typo", func(d map[string]any) {
			d["profiles"].([]any)[0].(map[string]any)["active_time"].(map[string]any)["typo"] = true
		}},
		{"$.profiles[0].roles", func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["roles"] = true }},
	} {
		dir := copyValidationData(t)
		change(t, dir, "specs.json", tc.edit)
		refusedField(t, dir, "specs.json", tc.field)
	}
}
