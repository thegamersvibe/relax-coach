package cfg_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizedDuplicates(t *testing.T) {
	// json.Marshal sorts uppercase keys first; exercise authored reverse order too.
	dir := copyData(t)
	path := filepath.Join(dir, "scoring.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	bad := bytes.Replace(b, []byte(`"count_penalties": {}`), []byte(`"count_penalties": {"owned_death": 1, "OWNED_DEATH": 2}`), 1)
	if bytes.Equal(b, bad) {
		t.Fatal("missing duplicate-key fixture anchor")
	}
	if err := os.WriteFile(path, bad, 0600); err != nil {
		t.Fatal(err)
	}
	refused(t, dir, "scoring.json")

	for _, transform := range []struct {
		name string
		f    func(string) string
	}{
		{"case", strings.ToUpper}, {"space", func(v string) string { return " \t" + v + "\n" }},
	} {
		for _, tc := range []struct {
			name, file string
			edit       func(map[string]any, func(string) string)
		}{
			{"labels", "realm-labels.json", func(d map[string]any, f func(string) string) {
				a := d["labels"].([]any)
				d["labels"] = append(a, f(a[0].(string)))
			}},
			{"categories", "consumables.json", func(d map[string]any, f func(string) string) {
				a := d["categories"].([]any)
				d["categories"] = append(a, f(a[0].(string)))
			}},
			{"assignments", "scoring.json", func(d map[string]any, f func(string) string) {
				a := d["assignments"].([]any)
				a[1].(map[string]any)["job"] = f(a[0].(map[string]any)["job"].(string))
			}},
			{"effects", "support.json", func(d map[string]any, f func(string) string) {
				a := d["effects"].([]any)
				a[1].(map[string]any)["id"] = f(a[0].(map[string]any)["id"].(string))
			}},
			{"candidates", "encounters-candidate.json", func(d map[string]any, f func(string) string) {
				a := d["encounters"].([]any)
				first := a[0].(map[string]any)
				dup := map[string]any{}
				for k, v := range first {
					dup[k] = v
				}
				dup["name"] = f(first["name"].(string))
				dup["raid"] = f(first["raid"].(string))
				d["encounters"] = append(a, dup)
			}},
			{"spec_ids", "spec-ids.json", func(d map[string]any, f func(string) string) {
				a := d["classes"].(map[string]any)["WARRIOR"].([]any)
				dup := map[string]any{"id": f(a[0].(map[string]any)["id"].(string)), "label": "duplicate", "default": false}
				d["classes"].(map[string]any)["WARRIOR"] = append(a, dup)
			}},
			{"profiles", "specs.json", func(d map[string]any, f func(string) string) {
				a := d["profiles"].([]any)
				dup := map[string]any{}
				for k, v := range a[0].(map[string]any) {
					dup[k] = v
				}
				dup["id"] = f(dup["id"].(string))
				d["profiles"] = append(a, dup)
			}},
			{"spells", "specs.json", func(d map[string]any, f func(string) string) {
				p := d["profiles"].([]any)[0].(map[string]any)
				a := p["spells"].([]any)
				p["spells"] = append(a, map[string]any{"name": f(a[0].(map[string]any)["name"].(string))})
			}},
			{"resource_exceptions", "specs.json", func(d map[string]any, f func(string) string) {
				p := d["profiles"].([]any)[0].(map[string]any)
				a := p["resource_exceptions"].([]any)
				p["resource_exceptions"] = append(a, f(a[0].(string)))
			}},
			{"mechanics", "encounters-enabled.json", func(d map[string]any, f func(string) string) {
				r := syntheticRule()
				a := r["mechanics"].([]any)
				dup := map[string]any{}
				for k, v := range a[0].(map[string]any) {
					dup[k] = v
				}
				dup["id"] = f(dup["id"].(string))
				r["mechanics"] = append(a, dup)
				d["encounters"] = []any{r}
			}},
			{"map_keys", "scoring.json", func(d map[string]any, f func(string) string) {
				d["count_penalties"] = map[string]any{"owned_death": 1, f("owned_death"): 2}
			}},
			{"nested_map_keys", "scoring.json", func(d map[string]any, f func(string) string) {
				d["measure_weights"] = map[string]any{"output": map[string]any{"damage": 1, f("damage"): 2}}
			}},
		} {
			t.Run(transform.name+"/"+tc.name, func(t *testing.T) {
				dir := copyData(t)
				change(t, dir, tc.file, func(d map[string]any) { tc.edit(d, transform.f) })
				refused(t, dir, tc.file)
				if tc.name == "spec_ids" {
					refusedField(t, dir, tc.file, "classes")
				}
			})
		}
	}
}

func sourcedSpell(name string, id int) map[string]any {
	return map[string]any{"name": name, "id": id, "source": map[string]any{"kind": "real_log", "note": "synthetic test"}}
}

func TestSpellIdentity(t *testing.T) {
	for _, field := range []string{"spells", "cast_signatures"} {
		for _, spells := range [][]any{
			{sourcedSpell("First", 77), sourcedSpell("Second", 77)},
			{sourcedSpell("First", 77), sourcedSpell(" first ", 78)},
		} {
			t.Run(field+fmt.Sprint(spells), func(t *testing.T) {
				dir := copyData(t)
				change(t, dir, "specs.json", func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)[field] = spells })
				refusedField(t, dir, "specs.json", field)
			})
		}
	}
	for _, different := range []bool{false, true} {
		dir := copyData(t)
		change(t, dir, "specs.json", func(d map[string]any) {
			d["profiles"].([]any)[0].(map[string]any)["spells"] = []any{sourcedSpell("First", 77)}
		})
		change(t, dir, "support.json", func(d map[string]any) {
			name := "First"
			if different {
				name = "Second"
			}
			d["effects"].([]any)[0].(map[string]any)["spell"] = sourcedSpell(name, 77)
		})
		if different {
			refusedField(t, dir, "support.json", "spell.id")
		} else {
			load(t, dir)
		}
	}
}
