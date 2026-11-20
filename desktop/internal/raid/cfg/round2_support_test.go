package cfg_test

import "testing"

func TestRound2CreditRequiresStacks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		credit bool
		stacks any
		omit   bool
		field  string
	}{
		{"null", true, nil, false, "effects[0].stacks"},
		{"missing", true, nil, true, "effects[0].stacks"},
		{"non_stacking", true, false, false, ""},
		{"stacking", true, true, false, ""},
		{"uncredited_unknown", false, nil, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "support.json", func(d map[string]any) {
				e := d["effects"].([]any)[0].(map[string]any)
				e["scope"], e["verified"], e["credit"], e["stacks"] = "raid", true, tc.credit, tc.stacks
				if tc.omit {
					delete(e, "stacks")
				}
			})
			if tc.field != "" {
				refusedField(t, dir, "support.json", tc.field)
			} else {
				load(t, dir)
			}
		})
	}
}
