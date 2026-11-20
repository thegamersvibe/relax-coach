package cfg_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

func TestRejectInvalidUTF8(t *testing.T) {
	for _, tc := range []struct{ file, old, field string }{
		{"support.json", "Provisional party", "effects["},
		{"support.json", "Mana Spring Totem", ".spell.name"},
		{"realm-labels.json", `"PvP-EU"`, "labels[0]"},
		{"support.json", `"note":`, ".no"},
	} {
		t.Run(tc.old, func(t *testing.T) {
			dir := copyData(t)
			path := filepath.Join(dir, tc.file)
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			replacement := append([]byte(tc.old[:3]), byte(255))
			replacement = append(replacement, []byte(tc.old[3:])...)
			bad := bytes.Replace(b, []byte(tc.old), replacement, 1)
			if bytes.Equal(b, bad) {
				t.Fatal("missing test anchor")
			}
			if err := os.WriteFile(path, bad, 0600); err != nil {
				t.Fatal(err)
			}
			refusedField(t, dir, tc.file, tc.field)
			_, err = cfg.Load(dir)
			if !strings.Contains(err.Error(), "UTF-8") {
				t.Fatalf("not an encoding error: %v", err)
			}
		})
	}
	dir := copyData(t)
	change(t, dir, "realm-labels.json", func(d map[string]any) { d["labels"] = []any{"العربية", "日本語", "valid �"} })
	load(t, dir)
}

func TestTrailingJSON(t *testing.T) {
	for _, tail := range []string{" {}", " []", " null", " garbage"} {
		t.Run(tail, func(t *testing.T) {
			dir := copyData(t)
			path := filepath.Join(dir, "instances.json")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(b, []byte(tail)...), 0600); err != nil {
				t.Fatal(err)
			}
			refused(t, dir, "instances.json")
		})
	}
}
