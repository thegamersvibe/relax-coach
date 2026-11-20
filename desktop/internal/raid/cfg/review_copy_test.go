package cfg_test

import (
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

func ownershipConfig(t *testing.T) *cfg.Config {
	dir := copyData(t)
	change(t, dir, "scoring.json", func(d map[string]any) {
		d["new_at_60_seconds"] = 60
		d["gcd_ms"] = 1500
		d["rounding_step"] = .5
		d["healer"].(map[string]any)["low_health_fraction"] = .2
		d["death"].(map[string]any)["lost_boss_gap_ms"] = 20
		d["death"].(map[string]any)["summary_window_ms"] = 30
		d["measure_weights"] = map[string]any{"output": map[string]any{"damage": 1}}
		d["count_penalties"] = map[string]any{"owned_death": 5}
	})
	change(t, dir, "specs.json", func(d map[string]any) {
		p := d["profiles"].([]any)[0].(map[string]any)
		p["spells"] = []any{sourcedSpell("Synthetic", 77)}
		p["cast_signatures"] = []any{sourcedSpell("Synthetic", 77)}
	})
	change(t, dir, "support.json", func(d map[string]any) {
		d["effects"].([]any)[0].(map[string]any)["spell"] = sourcedSpell("Synthetic", 77)
	})
	change(t, dir, "encounters-candidate.json", func(d map[string]any) {
		d["encounters"].([]any)[0].(map[string]any)["abilities"] = []any{sourcedSpell("Synthetic", 77)}
	})
	change(t, dir, "encounters-enabled.json", func(d map[string]any) {
		r := syntheticRule()
		r["target_rule"] = map[string]any{"mode": "boss_and_units", "units": []any{map[string]any{"id": 78, "source": "synthetic"}}}
		r["mechanics"].([]any)[0].(map[string]any)["spell"] = sourcedSpell("Synthetic", 77)
		d["encounters"] = []any{r}
	})
	return load(t, dir)
}

// Change every reachable exported leaf, including maps, nested slices and pointers.
func corrupt(v reflect.Value) {
	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			corrupt(v.Elem())
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				corrupt(v.Field(i))
			}
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			corrupt(v.Index(i))
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			x := reflect.New(v.Type().Elem()).Elem()
			x.Set(v.MapIndex(k))
			corrupt(x)
			v.SetMapIndex(k, x)
		}
	case reflect.String:
		v.SetString("changed")
	case reflect.Bool:
		v.SetBool(!v.Bool())
	case reflect.Int, reflect.Int64:
		v.SetInt(-1)
	case reflect.Float64:
		v.SetFloat(-1)
	}
}

func TestLookupCopies(t *testing.T) {
	c := ownershipConfig(t)
	expected := ownershipConfig(t)
	p, _ := expected.Profile("WARRIOR.arms")
	e, _ := expected.Encounter(77)
	targets := expected.OutputTargets(77)
	p2, _ := c.Profile("WARRIOR.arms")
	e2, _ := c.Encounter(77)
	targets2 := c.OutputTargets(77)
	corrupt(reflect.ValueOf(&p2).Elem())
	corrupt(reflect.ValueOf(&e2).Elem())
	corrupt(reflect.ValueOf(&targets2).Elem())
	p3, _ := c.Profile("WARRIOR.arms")
	e3, _ := c.Encounter(77)
	targets3 := c.OutputTargets(77)
	if !reflect.DeepEqual(p, p3) || !reflect.DeepEqual(e, e3) || !reflect.DeepEqual(targets, targets3) {
		t.Fatal("caller mutation changed a later lookup")
	}
}

func TestLoadViewCopies(t *testing.T) {
	c := ownershipConfig(t)
	expected := ownershipConfig(t)
	p, _ := expected.Profile("WARRIOR.arms")
	e, _ := expected.Encounter(77)
	targets := expected.OutputTargets(77)
	kind, listed := c.InstanceKind(329, 5)
	corrupt(reflect.ValueOf(c).Elem())
	p2, ok := c.Profile("WARRIOR.arms")
	e2, found := c.Encounter(77)
	targets2 := c.OutputTargets(77)
	kind2, listed2 := c.InstanceKind(329, 5)
	if !ok || !found || !reflect.DeepEqual(p, p2) || !reflect.DeepEqual(e, e2) || !reflect.DeepEqual(targets, targets2) || kind != kind2 || listed != listed2 {
		t.Fatal("Load exposed internal lookup state")
	}
}

func TestSnapshotCopies(t *testing.T) {
	c := ownershipConfig(t)
	getter, ok := any(c).(interface{ Snapshot() cfg.Config })
	if !ok {
		t.Fatal("no detached snapshot accessor")
	}
	before := any(ownershipConfig(t)).(interface{ Snapshot() cfg.Config }).Snapshot()
	out := getter.Snapshot()
	corrupt(reflect.ValueOf(&out).Elem())
	if !reflect.DeepEqual(before, getter.Snapshot()) {
		t.Fatal("returned snapshot shares internal maps/slices/pointers")
	}
	corrupt(reflect.ValueOf(c).Elem())
	if !reflect.DeepEqual(before, getter.Snapshot()) {
		t.Fatal("snapshot shares internal maps/slices/pointers")
	}
}
