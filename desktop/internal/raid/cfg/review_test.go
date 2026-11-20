package cfg_test

import (
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

func refusedField(t *testing.T, dir, file, field string) {
	t.Helper()
	err := refused(t, dir, file)
	if !strings.Contains(err.Error(), field) {
		t.Fatalf("error lacks field %s: %v", field, err)
	}
}

func TestSelectorNPCIdentity(t *testing.T) {
	for _, invalid := range []int64{0, -1} {
		s := cfg.CreatureSelector{ID: invalid}
		if s.Matches(invalid) {
			t.Errorf("invalid id %d matched", invalid)
		}
	}
	for _, label := range []string{"", "日本語"} {
		dir := copyData(t)
		change(t, dir, "encounters-enabled.json", func(d map[string]any) {
			u := d["encounters"].([]any)[0].(map[string]any)["boss_units"].([]any)[0].(map[string]any)
			if label == "" {
				delete(u, "name")
			} else {
				u["name"] = label
			}
		})
		load(t, dir)
	}

	for _, id := range []any{nil, 0, -1} {
		t.Run("invalid", func(t *testing.T) {
			dir := copyData(t)
			change(t, dir, "encounters-enabled.json", func(d map[string]any) {
				u := d["encounters"].([]any)[0].(map[string]any)["boss_units"].([]any)[0].(map[string]any)
				if id == nil {
					delete(u, "id")
				} else {
					u["id"] = id
				}
			})
			refused(t, dir, "encounters-enabled.json")
		})
	}
	for _, label := range []string{"", "Grand Crusader Dathrohan", "大十字軍ダソハン"} {
		u := cfg.CreatureSelector{ID: 10812, Name: label, Source: "synthetic test"}
		matcher, ok := any(u).(interface{ Matches(int64) bool })
		if !ok {
			t.Error("selector has no NPC-id matching contract")
			continue
		}
		if !matcher.Matches(10812) || matcher.Matches(10813) || matcher.Matches(0) {
			t.Errorf("label affected NPC identity: %+v", u)
		}
	}
}

func TestDuplicateBossNPC(t *testing.T) {
	dir := copyData(t)
	change(t, dir, "encounters-enabled.json", func(d map[string]any) {
		units := d["encounters"].([]any)[0].(map[string]any)["boss_units"].([]any)
		units[1].(map[string]any)["id"] = units[0].(map[string]any)["id"]
	})
	refused(t, dir, "encounters-enabled.json")
}
