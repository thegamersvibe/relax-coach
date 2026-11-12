package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Preserve the independent review's two adversarial regression tests unchanged.
// os.Root guarantees containment, not a no-symlinks open. Lstat/OpenRoot are
// separate operations. Exercise the exact candidate read path without hooks.
func TestIndependentAccountDirectorySwapMustNotCrossIdentity(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux race probe; Windows needs native validation")
	}
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	p := accountSave(t, game, "A#1", "Alpha")
	accountSave(t, game, "B#2", "BetaForeign")
	a, b := accountApp(t, game)
	if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(filepath.Dir(p))
	parked := filepath.Join(filepath.Dir(dir), "parked")
	stop, done := make(chan struct{}), make(chan error, 1)
	go func() {
		for {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			if err := os.Rename(dir, parked); err != nil {
				done <- err
				return
			}
			if err := os.Symlink("B#2", dir); err != nil {
				done <- err
				return
			}
			runtime.Gosched()
			if err := os.Remove(dir); err != nil {
				done <- err
				return
			}
			if err := os.Rename(parked, dir); err != nil {
				done <- err
				return
			}
			runtime.Gosched()
		}
	}()
	defer func() {
		close(stop)
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	deadline := time.Now().Add(4 * time.Second)
	for attempts := 1; time.Now().Before(deadline); attempts++ {
		// Same readSelectedSave used by reloadLocked, with initially enumerated
		// identities. No new identity request is sent during this loop.
		data, _, err := a.readSelectedSave()
		if err == nil && strings.Contains(string(data), "BetaForeign") {
			t.Fatalf("selected A#1 read B#2 save through raced relative symlink (attempt %d); candidate account=%q", attempts, a.account)
		}
	}
}

func TestIndependentAccountFullRefreshSwapMustNotCrossIdentity(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux race probe; Windows needs native validation")
	}
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	p := accountSave(t, game, "A#1", "Alpha")
	accountSave(t, game, "B#2", "BetaForeign")
	a, b := accountApp(t, game)
	if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(filepath.Dir(p))
	parked := filepath.Join(filepath.Dir(dir), "parked")
	stop, done := make(chan struct{}), make(chan error, 1)
	go func() {
		for {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			if err := os.Rename(dir, parked); err != nil {
				done <- err
				return
			}
			if err := os.Symlink("B#2", dir); err != nil {
				done <- err
				return
			}
			runtime.Gosched()
			if err := os.Remove(dir); err != nil {
				done <- err
				return
			}
			if err := os.Rename(parked, dir); err != nil {
				done <- err
				return
			}
			runtime.Gosched()
		}
	}()
	defer func() {
		close(stop)
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	deadline := time.Now().Add(8 * time.Second)
	for attempts := 1; time.Now().Before(deadline); attempts++ {
		value, err := b.Call("reportRefresh", nil)
		if err != nil {
			t.Fatal(err)
		}
		v := a.snapshot()
		if v.Account != "A#1" {
			t.Fatalf("selected identity changed: %q", v.Account)
		}
		if strings.Contains(value.(nativeReport).HTML, "BetaForeign") {
			t.Fatalf("full reportRefresh displayed B#2's BetaForeign under selected %q and source %q (attempt %d)", v.Account, v.SVPath, attempts)
		}
	}
}

// A final-file link can stay inside SavedVariables and still substitute an
// unchecked object. The data is copied from B; no escape from os.Root is needed.
func TestAccountFinalSaveSwapRejectsUncheckedFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux race probe; Windows needs native validation")
	}
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	p := accountSave(t, game, "A#1", "Alpha")
	other := accountSave(t, game, "B#2", "BetaForeign")
	foreign, err := os.ReadFile(other)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(p), "foreign.lua"), foreign, 0600); err != nil {
		t.Fatal(err)
	}
	a, b := accountApp(t, game)
	if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
		t.Fatal(err)
	}
	accountIdentitySwapProbe(t, p, "foreign.lua", func(attempt int) {
		data, _, err := a.readSelectedSave()
		if err == nil && !strings.Contains(string(data), "Alpha") {
			t.Fatalf("selected save read unchecked replacement (attempt %d): %s", attempt, data)
		}
	})
	data, _, err := a.readSelectedSave()
	if err != nil || !strings.Contains(string(data), "Alpha") {
		t.Fatalf("original save did not recover: %q, %v", data, err)
	}
}

func TestAccountIntermediateDirectorySwapsMustNotCrossIdentity(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux race probe; Windows needs native validation")
	}
	for _, layer := range []string{"WTF", "Account", "identity", "SavedVariables"} {
		t.Run(layer, func(t *testing.T) {
			accountConfigEnv(t)
			game := filepath.Join(t.TempDir(), "_forever_")
			p := accountSave(t, game, "A#1", "Alpha")
			target := filepath.Dir(p)
			switch layer {
			case "WTF":
				target = filepath.Join(game, "WTF")
			case "Account":
				target = filepath.Join(game, "WTF", "Account")
			case "identity":
				target = filepath.Dir(target)
			}
			foreignDir := target + ".foreign"
			rel, err := filepath.Rel(target, p)
			if err != nil {
				t.Fatal(err)
			}
			foreignPath := filepath.Join(foreignDir, rel)
			if err := os.MkdirAll(filepath.Dir(foreignPath), 0755); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(foreignPath, []byte(strings.ReplaceAll(string(data), "Alpha", "BetaForeign")), 0600); err != nil {
				t.Fatal(err)
			}
			a, b := accountApp(t, game)
			if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
				t.Fatal(err)
			}
			accountIdentitySwapProbe(t, target, filepath.Base(foreignDir), func(attempt int) {
				data, _, err := a.readSelectedSave()
				if err == nil && !strings.Contains(string(data), "Alpha") {
					t.Fatalf("%s swap read another directory (attempt %d): %s", layer, attempt, data)
				}
			})
			data, _, err = a.readSelectedSave()
			if err != nil || !strings.Contains(string(data), "Alpha") {
				t.Fatalf("original directory did not recover: %q, %v", data, err)
			}
		})
	}
}

// In particular, Root.Lstat captures Windows file IDs eagerly; os.Lstat may
// defer collecting them until SameFile, when a pathname could mean another file.
func TestAccountDirectoryHandleKeepsCheckedIdentityAfterRename(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "A#1")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	checked, err := root.Lstat("A#1")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := openPlainDirectory(root, "A#1")
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if err := os.Rename(path, path+".parked"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	opened, err := dir.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	replacement, err := root.Lstat("A#1")
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(checked, opened) || os.SameFile(checked, replacement) {
		t.Fatal("checked identity or opened handle followed the replaced pathname")
	}
}

func TestAccountNormalSaveReplacementKeepsSelectionAndRecovers(t *testing.T) {
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	p := accountSave(t, game, "A#1", "Alpha")
	accountSave(t, game, "B#2", "BetaForeign")
	a, b := accountApp(t, game)
	if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
		t.Fatal(err)
	}
	saves, err := os.OpenRoot(filepath.Dir(p))
	if err != nil {
		t.Fatal(err)
	}
	defer saves.Close()
	before, err := saves.Lstat("ForeverCoach.lua")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	replacement := p + ".new"
	if err := os.WriteFile(replacement, []byte(strings.ReplaceAll(string(data), "Alpha", "AlphaReloaded")), 0600); err != nil {
		t.Fatal(err)
	}
	mtime := before.ModTime().Add(time.Second)
	if err := os.Chtimes(replacement, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, p); err != nil {
		t.Fatal(err)
	}
	after, err := saves.Lstat("ForeverCoach.lua")
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("fixture did not replace the save's identity")
	}
	if _, err := b.Call("reportRefresh", nil); err != nil {
		t.Fatal(err)
	}
	assertAccountCharacter(t, a, "AlphaReloaded")
	if v := a.snapshot(); v.Account != "A#1" || v.SVPath != p || v.Warning != "" || v.Error != "" {
		t.Fatalf("normal replacement changed selection or failed: %+v", v)
	}
	// A transient gap must retain only this account's last-good report, then
	// recover without requiring selection again or pinning the old inode.
	if err := os.Rename(p, replacement); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call("reportRefresh", nil); err != nil {
		t.Fatal(err)
	}
	assertAccountCharacter(t, a, "AlphaReloaded")
	if v := a.snapshot(); v.Account != "A#1" || v.Warning == "" {
		t.Fatalf("transient gap lost selected account or stale warning: %+v", v)
	}
	if err := os.Rename(replacement, p); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call("reportRefresh", nil); err != nil {
		t.Fatal(err)
	}
	assertAccountCharacter(t, a, "AlphaReloaded")
	if v := a.snapshot(); v.Account != "A#1" || v.Warning != "" || v.Error != "" {
		t.Fatalf("replacement did not recover: %+v", v)
	}
}

// Use real filesystem mutation, not hooks between validation and opening.
func accountIdentitySwapProbe(t *testing.T, path, link string, probe func(int)) {
	t.Helper()
	parked := path + ".parked"
	type result struct {
		cycles int
		err    error
	}
	stop, done := make(chan struct{}), make(chan result, 1)
	go func() {
		cycles := 0
		swap := func() error {
			if err := os.Rename(path, parked); err != nil {
				return err
			}
			if err := os.Symlink(link, path); err != nil {
				return err
			}
			runtime.Gosched()
			if err := os.Remove(path); err != nil {
				return err
			}
			return os.Rename(parked, path)
		}
		for {
			select {
			case <-stop:
				done <- result{cycles: cycles}
				return
			default:
			}
			if err := swap(); err != nil {
				done <- result{cycles: cycles, err: err}
				return
			}
			cycles++
			runtime.Gosched()
		}
	}()
	attempts := 0
	defer func() {
		close(stop)
		r := <-done
		if r.err != nil {
			t.Error(r.err)
		}
		if r.cycles == 0 {
			t.Error("filesystem mutator did not run")
		}
		t.Logf("read attempts=%d completed swaps=%d", attempts, r.cycles)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		attempts++
		probe(attempts)
	}
}
