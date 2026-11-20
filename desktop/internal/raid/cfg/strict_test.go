package cfg_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

func refused(t *testing.T, dir, file string) error {
	t.Helper()
	path := filepath.Join(dir, file)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	c, err := cfg.Load(dir)
	if err == nil || c != nil {
		t.Fatalf("accepted invalid %s; partial=%v err=%v", file, c != nil, err)
	}
	if !strings.Contains(err.Error(), file) {
		t.Fatalf("error lacks file context: %v", err)
	}
	validationErr := err
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("invalid/newer config was rewritten")
	}
	return validationErr
}

func TestRejectUnknownKeys(t *testing.T) {
	for _, file := range dataFiles {
		t.Run(file, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, file, func(d map[string]any) { d["typo"] = true })
			refused(t, dir, file)
		})
	}
	for _, tc := range []struct {
		file string
		edit func(map[string]any)
	}{
		{"scoring.json", func(d map[string]any) { d["fights"].(map[string]any)["pullGap"] = 1 }},
		{"specs.json", func(d map[string]any) {
			d["profiles"].([]any)[0].(map[string]any)["active_time"].(map[string]any)["tallent"] = true
		}},
		{"spec-ids.json", func(d map[string]any) {
			d["classes"].(map[string]any)["WARRIOR"].([]any)[0].(map[string]any)["typo"] = true
		}},
		{"support.json", func(d map[string]any) {
			d["effects"].([]any)[0].(map[string]any)["spell"].(map[string]any)["spellID"] = 42
		}},
	} {
		t.Run("nested/"+tc.file, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, tc.file, tc.edit)
			refused(t, dir, tc.file)
		})
	}
}

func TestRejectVersions(t *testing.T) {
	for _, file := range dataFiles {
		unsupported := 2
		if file == "scoring.json" {
			unsupported = 1
		}
		for _, v := range []any{0, unsupported, 3, -1, nil, "1"} {
			t.Run(file, func(t *testing.T) {
				dir := copyValidationData(t)
				change(t, dir, file, func(d map[string]any) { d["schema"] = v })
				refused(t, dir, file)
			})
		}
	}
	for _, field := range []string{"schema", "version", "source"} {
		t.Run("missing/"+field, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { delete(d, field) })
			refused(t, dir, "scoring.json")
		})
	}
	for _, v := range []string{"", "1.2", "latest", "1.-1.0", "01.0.0"} {
		t.Run("bad-content-version/"+v, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, "scoring.json", func(d map[string]any) { d["version"] = v })
			refused(t, dir, "scoring.json")
		})
	}
	// A new content version with the same format is precisely the no-release path.
	dir := copyValidationData(t)
	change(t, dir, "scoring.json", func(d map[string]any) { d["version"] = "99.2.0" })
	if load(t, dir).Scoring.Version != "99.2.0" {
		t.Fatal("content revision rejected/cached")
	}
}

func TestRejectJSONShape(t *testing.T) {
	for _, edit := range []func(map[string]any){
		func(d map[string]any) { delete(d, "fights") },
		func(d map[string]any) { delete(d["fights"].(map[string]any), "tail_ms") },
		func(d map[string]any) { d["fights"].(map[string]any)["tail_ms"] = nil },
		func(d map[string]any) { d["fights"].(map[string]any)["tail_ms"] = 1.5 },
		func(d map[string]any) { d["ratio_points"] = nil },
		func(d map[string]any) { d["Schema"] = d["schema"]; delete(d, "schema") },
		func(d map[string]any) { d["source"] = "   " },
	} {
		dir := copyValidationData(t)
		change(t, dir, "scoring.json", edit)
		refused(t, dir, "scoring.json")
	}
	for _, edit := range []func([]byte) []byte{
		func(b []byte) []byte { return append(b, []byte(` {}`)...) },
		func(b []byte) []byte { return []byte(`null`) },
		func(b []byte) []byte { return []byte(`[]`) },
		func(b []byte) []byte { return []byte(``) },
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"schema": 2`), []byte(`"schema": 2, "schema": 2`), 1)
		},
		func(b []byte) []byte {
			return bytes.Replace(b, []byte(`"tail_ms": 2500`), []byte(`"tail_ms": 2500, "tail_ms": 1`), 1)
		},
	} {
		dir := copyValidationData(t)
		path := filepath.Join(dir, "scoring.json")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		bad := edit(b)
		if bytes.Equal(b, bad) {
			t.Fatal("JSON shape fixture edit did not change the input")
		}
		if err := os.WriteFile(path, bad, 0600); err != nil {
			t.Fatal(err)
		}
		refused(t, dir, "scoring.json")
	}
}

func TestRejectCaseFoldedAliases(t *testing.T) {
	for _, tc := range []struct {
		file string
		edit func(map[string]any)
	}{
		{"scoring.json", func(d map[string]any) { d["Schema"] = d["schema"] }},
		{"scoring.json", func(d map[string]any) { d["fights"].(map[string]any)["TAIL_MS"] = 1 }},
		{"specs.json", func(d map[string]any) { d["profiles"].([]any)[0].(map[string]any)["Uncalibrated"] = true }},
	} {
		t.Run(tc.file, func(t *testing.T) {
			dir := copyValidationData(t)
			change(t, dir, tc.file, tc.edit)
			refused(t, dir, tc.file)
		})
	}
}

func TestMissingFileNoPartialConfig(t *testing.T) {
	for _, file := range dataFiles {
		t.Run(file, func(t *testing.T) {
			dir := copyValidationData(t)
			if err := os.Remove(filepath.Join(dir, file)); err != nil {
				t.Fatal(err)
			}
			c, err := cfg.Load(dir)
			if err == nil || c != nil || !strings.Contains(err.Error(), file) {
				t.Fatalf("config=%v error=%v", c, err)
			}
		})
	}
}
