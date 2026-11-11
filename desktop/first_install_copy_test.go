package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A crash while the app copies itself must not leave a half-written
// RelaxCoach.exe at the installed path, where it would read as "already
// installed" and block every later copy.
func TestFICopyCrashLeavesUnusableExistingCopy(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	var residue string
	var half []byte
	appCopyWritten = func(path string) {
		b, _ := os.ReadFile(path)
		residue, half = path, b[:len(b)/2]
	}
	t.Cleanup(func() { appCopyWritten = nil })
	p, err := e.install()
	if err != nil {
		t.Fatal(err)
	}
	// Recreate what a kill at that moment would leave on disk.
	if err := os.RemoveAll(filepath.Dir(p.Target)); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Dir(residue), 0755)
	if err := os.WriteFile(residue, half, 0700); err != nil {
		t.Fatal(err)
	}
	appCopyWritten = nil
	if got := e.placement(); got.State != "offer" {
		t.Fatalf("after a crash mid-copy the placement is %q (%s); the half-written file at %s blocks every later copy", got.State, got.Message, residue)
	}
	if _, err := e.install(); err != nil {
		t.Fatalf("a later copy failed after a crash: %v", err)
	}
	want, _ := os.ReadFile(f.exe)
	if got, err := os.ReadFile(p.Target); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("installed copy differs after recovery: %v", err)
	}
	if b, err := os.ReadFile(residue); err != nil || !bytes.Equal(b, half) {
		t.Fatalf("the earlier run's leftover was touched; only this run's temp file may be cleaned up: %v", err)
	}
}

// Something other than a regular file at the installed path is not "already
// installed": it is reported as such and nothing is copied.
func TestFICopyTargetIsNotAFile(t *testing.T) {
	for _, kind := range []string{"folder", "link"} {
		t.Run(kind, func(t *testing.T) {
			f := newPlacementFixture(t)
			target := filepath.Join(f.home, "RelaxCoach", "RelaxCoach.exe")
			os.MkdirAll(filepath.Dir(target), 0755)
			if kind == "folder" {
				os.Mkdir(target, 0755)
			} else if err := os.Symlink(f.exe, target); err != nil {
				t.Skip(err)
			}
			e := f.env(nil)
			p := e.placement()
			if p.State != "blocked" || !strings.Contains(p.Message, "something else is at that path") {
				t.Fatalf("placement %+v", p)
			}
			if _, err := e.install(); err == nil {
				t.Fatal("copied over something that is not a regular file")
			}
		})
	}
}
