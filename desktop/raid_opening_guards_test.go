package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestRaidPagePickerRechecksAdmission(t *testing.T) {
	game, _, b := raidOpeningFixture(t)
	log := filepath.Join(t.TempDir(), "selected.txt")
	raidOpeningLog(t, log, 99)
	b.services.chooseFile = func(context.Context, string) (string, error) {
		raidRankSave(t, game, "A#1", strings.Replace(officerSave(), `"officer"`, `"member"`, 1))
		return log, nil
	}
	if got, err := b.Call("raidChooseLog", nil); got != nil || err == nil || !strings.Contains(err.Error(), "locked") {
		t.Fatal("picker reused old admission")
	}
}

func TestRaidPageEmptyAndErrors(t *testing.T) {
	_, _, b := raidOpeningFixture(t)
	if !strings.Contains(raidCallHTML(t, b, "raidPage"), "Open a combat log") {
		t.Fatal("empty page tried to open a log")
	}
	if !strings.Contains(raidCallHTML(t, b, "raidChooseOfficer"), "open a night before") {
		t.Fatal("officer chooser without a night")
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing.txt"), t.TempDir()} {
		html := raidCallHTML(t, b, "raidOpen", path)
		if !strings.Contains(html, `role="alert"`) || strings.Contains(html, `id="raid-ranked"`) {
			t.Fatal("unreadable log did not show error without night")
		}
	}
	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raidCallHTML(t, b, "raidOpen", path), "no nights found") {
		t.Fatal("empty input fabricated a night")
	}
	for _, method := range []string{"raidOpen", "raidNight", "raidOfficerFile"} {
		if _, err := b.Call(method, nil); err == nil {
			t.Fatal("missing path/index accepted")
		}
	}
}

// Counts only, through the same admitted bridge route and embedded config used
// by the app. Never print a source filename, actor, identifier or error detail.
func TestRaidPageWiredRealLogs(t *testing.T) {
	folder := os.Getenv("RELAX_RAID_REAL_LOGS")
	if folder == "" {
		t.Skip("private logs not enabled")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal("private folder unreadable")
	}
	_, _, b := raidOpeningFixture(t)
	count := 0
	number := func(text, pattern string) int {
		t.Helper()
		m := regexp.MustCompile(pattern).FindStringSubmatch(text)
		if len(m) != 2 {
			t.Fatal("private count missing")
		}
		n, e := strconv.Atoi(m[1])
		if e != nil {
			t.Fatal("private count malformed")
		}
		return n
	}
	rows := func(text, id string) int {
		t.Helper()
		start := strings.Index(text, `id="`+id+`"`)
		if start < 0 {
			t.Fatal("private score list missing")
		}
		end := strings.Index(text[start:], "</section>")
		if end < 0 {
			t.Fatal("private list end missing")
		}
		return strings.Count(text[start:start+end], `data-raid-player=`)
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		got, err := b.Call("raidOpen", nativeArgs(filepath.Join(folder, entry.Name())))
		if err != nil {
			t.Fatal("private opening failed")
		}
		html, ok := got.(string)
		if !ok || strings.Contains(html, `role="alert"`) {
			t.Fatal("private rendering failed")
		}
		bosses := number(html, ` · ([0-9]+) boss pulls`)
		if bosses != 13 || !strings.Contains(html, "Dungeon run: no ranks") {
			t.Fatal("private classification/count changed")
		}
		count++
		t.Logf("log %d: wired_pages=%d lines=%d bosses=%d trash=%d scored_unranked=%d provisional=%d not_enough=%d deaths=%d wipes=%d", count, strings.Count(html, "<option "), number(html, `Lines read: ([0-9]+)`), bosses, number(html, ` · ([0-9]+) trash pulls`), rows(html, "raid-ranked"), rows(html, "raid-provisional"), rows(html, "raid-unscored"), number(html, `<h3>Deaths \(([0-9]+)\)`), number(html, `<h3>Wipes \(([0-9]+)\)`))
	}
	if count != 2 {
		t.Fatal("expected two private logs")
	}
}
