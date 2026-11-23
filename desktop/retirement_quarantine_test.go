package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"testing"
)

type retirementTestEntry struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Type        string `json:"type"`
	SHA256      string `json:"sha256,omitempty"`
}

// Every inherited scope assertion proves both absence from the source and
// presence in a verified manifest (or the inverse for protected neighbours).
func retirementKeptOrMoved(t *testing.T, game, path string, kept bool) {
	t.Helper()
	retirementExists(t, path, kept)
	relative, err := filepath.Rel(game, path)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, run := range retirementTestRuns(t, game) {
		for _, entry := range retirementTestManifest(t, game, run) {
			if entry.Source == filepath.ToSlash(relative) {
				count++
			}
		}
	}
	if (!kept && count != 1) || (kept && count != 0) {
		t.Fatalf("%s kept=%v, manifest entries=%d", relative, kept, count)
	}
}
func retirementTestDigest(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
func retirementTestRuns(t *testing.T, game string) []string {
	t.Helper()
	dirs, err := os.ReadDir(filepath.Join(game, "RelaxCoachRetired"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var runs []string
	for _, d := range dirs {
		if d.IsDir() {
			runs = append(runs, d.Name())
		}
	}
	return runs
}
func retirementTestManifest(t *testing.T, game, run string) []retirementTestEntry {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(game, "RelaxCoachRetired", run, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Schema  int                   `json:"schema"`
		Entries []retirementTestEntry `json:"entries"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Schema != 1 {
		t.Fatalf("manifest schema: %s", raw)
	}
	for _, e := range doc.Entries {
		retirementExists(t, filepath.Join(game, filepath.FromSlash(e.Destination)), true)
		if e.Type == "file" {
			data, err := os.ReadFile(filepath.Join(game, filepath.FromSlash(e.Destination)))
			if err != nil {
				t.Fatal(err)
			}
			if retirementTestDigest(string(data)) != e.SHA256 {
				t.Fatalf("hash mismatch: %+v", e)
			}
		} else if e.Type != "dir" || e.SHA256 != "" {
			t.Fatalf("invalid directory entry: %+v", e)
		}
	}
	return doc.Entries
}
func TestRetirementMoveNotDeleteManifestExact(t *testing.T) {
	game := retirementFixture(t)
	files := map[string]string{
		"Interface/AddOns/ForeverCoach/Core.lua":             "original addon bytes",
		"Interface/AddOns/ForeverCoach/nested/data.bin":      "\x00\xff\x01",
		"WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua": "original save bytes",
	}
	for p, data := range files {
		addonPut(t, game, p, data)
	}
	if err := os.Mkdir(filepath.Join(game, "Interface/AddOns/ForeverCoach/empty"), 0700); err != nil {
		t.Fatal(err)
	}
	addonPut(t, game, "Interface/AddOns/AHProfit/keep.lua", "keep")
	result := retireGame(context.Background(), game, retirementClosed)
	runs := retirementTestRuns(t, game)
	if len(runs) != 1 {
		t.Fatalf("want one reversible run, got %v: %+v", runs, result)
	}
	if !regexp.MustCompile(`^\d{8}T\d{6}Z(-\d+)?$`).MatchString(runs[0]) {
		t.Fatalf("not a UTC run name: %s", runs[0])
	}
	prefix := "RelaxCoachRetired/" + runs[0] + "/"
	want := []retirementTestEntry{
		{"Interface/AddOns/ForeverCoach", prefix + "AddOns/ForeverCoach", "dir", ""},
		{"Interface/AddOns/ForeverCoach/Core.lua", prefix + "AddOns/ForeverCoach/Core.lua", "file", retirementTestDigest(files["Interface/AddOns/ForeverCoach/Core.lua"])},
		{"Interface/AddOns/ForeverCoach/empty", prefix + "AddOns/ForeverCoach/empty", "dir", ""},
		{"Interface/AddOns/ForeverCoach/nested", prefix + "AddOns/ForeverCoach/nested", "dir", ""},
		{"Interface/AddOns/ForeverCoach/nested/data.bin", prefix + "AddOns/ForeverCoach/nested/data.bin", "file", retirementTestDigest(files["Interface/AddOns/ForeverCoach/nested/data.bin"])},
		{"WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua", prefix + "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua", "file", retirementTestDigest(files["WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"])},
	}
	if got := retirementTestManifest(t, game, runs[0]); !reflect.DeepEqual(got, want) {
		t.Fatalf("manifest entries\ngot  %+v\nwant %+v", got, want)
	}
	for p := range files {
		retirementExists(t, filepath.Join(game, p), false)
	}
	retirementExists(t, filepath.Join(game, "Interface/AddOns/AHProfit/keep.lua"), true)
	before, err := os.ReadFile(filepath.Join(game, prefix, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	retireGame(context.Background(), game, retirementClosed)
	if got := retirementTestRuns(t, game); !reflect.DeepEqual(got, runs) {
		t.Fatalf("empty rerun created a directory: %v", got)
	}
	after, err := os.ReadFile(filepath.Join(game, prefix, "manifest.json"))
	if err != nil || string(before) != string(after) {
		t.Fatalf("rerun rewrote manifest: %v", err)
	}
}
