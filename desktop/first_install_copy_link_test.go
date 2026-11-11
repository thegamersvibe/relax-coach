//go:build linux

package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func fiLinks(t *testing.T, p string) uint64 {
	t.Helper()
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return uint64(st.Sys().(*syscall.Stat_t).Nlink)
}

// مانع F1: publishing the copy must never give the executable a second name.
// If the app is killed right after publishing (or removing a temp name
// fails), a hard-linked RelaxCoach.exe is refused by appReadFile, so Update
// App on that copy would stop working for good.
func TestFICopyPublishLeavesOneLink(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	var atKill uint64
	var readErr error
	appCopyPublished = func(target string) {
		atKill = fiLinks(t, target)            // what a kill at this instant leaves
		_, readErr = appReadFile(target, appMaxExecutable) // the check ApplyAppRelease runs first
	}
	t.Cleanup(func() { appCopyPublished = nil })
	p, err := e.install()
	if err != nil {
		t.Fatal(err)
	}
	if atKill != 1 || readErr != nil {
		t.Fatalf("right after publishing the copy has %d links (read: %v); a kill here breaks Update App on it", atKill, readErr)
	}
	if n := fiLinks(t, p.Target); n != 1 {
		t.Fatalf("installed copy has %d links", n)
	}
	entries, _ := os.ReadDir(filepath.Dir(p.Target))
	if len(entries) != 1 {
		t.Fatalf("temp file left next to the copy: %v", entries)
	}
}

// When publishing fails the message may say nothing changed, and then it must
// be true: the existing file is untouched and no temp file is left.
func TestFICopyPublishFailureChangesNothing(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	target := filepath.Join(f.home, "RelaxCoach", "RelaxCoach.exe")
	appCopyWritten = func(string) { os.WriteFile(target, []byte("someone else"), 0600) }
	t.Cleanup(func() { appCopyWritten = nil })
	_, err := e.install()
	if err == nil || !strings.Contains(err.Error(), "nothing was changed") {
		t.Fatalf("expected a refused publish, got %v", err)
	}
	if b, _ := os.ReadFile(target); string(b) != "someone else" {
		t.Fatal("the existing file was replaced")
	}
	if entries, _ := os.ReadDir(filepath.Dir(target)); len(entries) != 1 {
		t.Fatalf("temp file left: %v", entries)
	}
}
