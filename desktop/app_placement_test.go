package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type placementFixture struct {
	root, exe, home, desktop string
	shortcuts                []string
}

func newPlacementFixture(t *testing.T) *placementFixture {
	t.Helper()
	f := &placementFixture{root: t.TempDir()}
	f.exe = filepath.Join(f.root, "Downloads", "RelaxCoach.exe")
	f.home = filepath.Join(f.root, "Users", "player")
	f.desktop = filepath.Join(f.home, "Desktop")
	for _, d := range []string{filepath.Dir(f.exe), f.desktop} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(f.exe, append([]byte("MZ"), bytes.Repeat([]byte{7}, 4096)...), 0755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *placementFixture) env(shortcutErr error) appPlacementEnv {
	return appPlacementEnv{exe: f.exe, home: f.home, desktop: f.desktop, shortcut: func(link, target, dir string) error {
		f.shortcuts = append(f.shortcuts, link+"->"+target+"@"+dir)
		if shortcutErr != nil {
			return shortcutErr
		}
		return os.WriteFile(link, []byte("lnk"), 0644)
	}}
}

func TestAppPlacementOfferCopyAndShortcut(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	p := e.placement()
	target := filepath.Join(f.home, "RelaxCoach", "RelaxCoach.exe")
	if p.State != "offer" || p.Target != target || p.Shortcut != filepath.Join(f.desktop, "Relax Coach.lnk") {
		t.Fatalf("placement %+v", p)
	}
	got, err := e.install()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(f.exe)
	copied, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(copied, want) {
		t.Fatalf("copy differs: %v", err)
	}
	if got.State != "copied" || !strings.Contains(got.Message, "shortcut was added") || len(f.shortcuts) != 1 ||
		f.shortcuts[0] != filepath.Join(f.desktop, "Relax Coach.lnk")+"->"+target+"@"+filepath.Dir(target) {
		t.Fatalf("result %+v shortcuts %v", got, f.shortcuts)
	}
	// The copy is now the installed one: running it offers nothing.
	e.exe = target
	if p := e.placement(); p.State != "installed" {
		t.Fatalf("running the installed copy: %+v", p)
	}
	// A second downloaded copy points at the installed one and never replaces it.
	e.exe = f.exe
	if p := e.placement(); p.State != "other-copy" || !strings.Contains(p.Message, "already installed") {
		t.Fatalf("second copy: %+v", p)
	}
	if _, err := e.install(); err == nil {
		t.Fatal("replaced an installed copy")
	}
}

func TestAppPlacementShortcutProblemsKeepTheCopy(t *testing.T) {
	// Shortcut creation fails: the verified copy stays, the message says why.
	f := newPlacementFixture(t)
	got, err := f.env(errors.New("COM unavailable")).install()
	if err != nil || got.State != "copied" || !strings.Contains(got.Message, "could not be created (COM unavailable)") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := os.Stat(got.Target); err != nil {
		t.Fatal("copy removed after a shortcut failure")
	}
	// An existing shortcut is left alone.
	f = newPlacementFixture(t)
	os.WriteFile(filepath.Join(f.desktop, "Relax Coach.lnk"), []byte("player's own"), 0644)
	got, err = f.env(nil).install()
	if err != nil || len(f.shortcuts) != 0 || !strings.Contains(got.Message, "already exists and was left unchanged") {
		t.Fatalf("%+v %v %v", got, err, f.shortcuts)
	}
	if b, _ := os.ReadFile(filepath.Join(f.desktop, "Relax Coach.lnk")); string(b) != "player's own" {
		t.Fatal("existing shortcut overwritten")
	}
	// No shortcut support at all.
	f = newPlacementFixture(t)
	e := f.env(nil)
	e.shortcut = nil
	if got, err := e.install(); err != nil || !strings.Contains(got.Message, "open RelaxCoach.exe from") {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestAppPlacementRefusesLinkedFolderAndUnknownPaths(t *testing.T) {
	f := newPlacementFixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(f.home, "RelaxCoach")); err != nil {
		t.Skip(err)
	}
	e := f.env(nil)
	if p := e.placement(); p.State != "other-copy" && p.State != "offer" {
		t.Fatalf("placement %+v", p)
	}
	if _, err := e.install(); err == nil {
		t.Fatal("copied through a linked RelaxCoach folder")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote outside the user folder: %v", entries)
	}
	if p := (appPlacementEnv{err: errors.New("no home")}).placement(); p.State != "unavailable" {
		t.Fatalf("%+v", p)
	}
	if p := (appPlacementEnv{exe: "relative.exe", home: f.home}).placement(); p.State != "unavailable" {
		t.Fatalf("%+v", p)
	}
}

func TestAppPlacementBridge(t *testing.T) {
	f := newPlacementFixture(t)
	b := newNativeBridge(&App{}, false, nativeServices{place: func() appPlacementEnv { return f.env(nil) }})
	defer b.Close()
	p, err := b.Call("appPlacement", nil)
	if err != nil || p.(AppPlacement).State != "offer" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := b.Call("installAppCopy", []json.RawMessage{json.RawMessage(`"x"`)}); err == nil {
		t.Fatal("installAppCopy accepted an argument")
	}
	got, err := b.Call("installAppCopy", nil)
	if err != nil || got.(AppPlacement).State != "copied" {
		t.Fatalf("%+v %v", got, err)
	}
	none := newNativeBridge(&App{}, false, nativeServices{})
	defer none.Close()
	if p, err := none.Call("appPlacement", nil); err != nil || p.(AppPlacement).State != "unavailable" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := none.Call("installAppCopy", nil); err == nil {
		t.Fatal("copied without a placement service")
	}
}
