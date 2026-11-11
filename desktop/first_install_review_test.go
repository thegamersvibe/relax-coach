package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for the independent review of the first-install commit.

// withMkdirHook runs fn after every Mkdir the installers make, like another
// program acting between two steps.
func withMkdirHook(t *testing.T, fn func(path string) error) {
	t.Helper()
	old := addonMkdir
	addonMkdir = func(path string, mode os.FileMode) error {
		if err := old(path, mode); err != nil {
			return err
		}
		return fn(path)
	}
	t.Cleanup(func() { addonMkdir = old })
}

// A parent created by the install and then replaced by a link must not
// redirect the addon outside the game folder (More addons and the bundle).
func TestFirstInstallCreatedParentReplacedByLink(t *testing.T) {
	for _, bundle := range []bool{false, true} {
		game := freshClient(t)
		outside := t.TempDir()
		if err := os.Mkdir(filepath.Join(outside, "AddOns"), 0755); err != nil {
			t.Fatal(err)
		}
		withMkdirHook(t, func(path string) error {
			if filepath.Base(path) != "AddOns" {
				return nil
			}
			iface := filepath.Join(game, "Interface")
			if err := os.Rename(iface, iface+"-moved"); err != nil {
				return err
			}
			return os.Symlink(outside, iface)
		})
		var err error
		if bundle {
			u := guildUpdaterForVersion(t, bundleZip(t, "1.5.0", nil), "1.5.0")
			_, err = u.apply(context.Background(), game, guildInfoForVersion("1.5.0"))
		} else {
			u := newExtraUpdater(t, extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3")))
			info, cerr := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
			if cerr != nil {
				t.Fatal(cerr)
			}
			_, err = u.applyExtra(context.Background(), game, info)
		}
		if err == nil {
			t.Fatalf("bundle=%v: install accepted a replaced parent", bundle)
		}
		if entries, _ := os.ReadDir(filepath.Join(outside, "AddOns")); len(entries) != 0 {
			t.Fatalf("bundle=%v: wrote outside the game folder: %v", bundle, entries)
		}
		addonMkdir = os.Mkdir
	}
}

// Cleanup after a failed More-addons install removes only the folders this
// install created, never a replacement or anything outside the game.
func TestFirstInstallExtraCleanupRemovesOnlyItsOwnFolders(t *testing.T) {
	for _, mode := range []string{"Interface replaced by link", "AddOns replaced by another folder"} {
		t.Run(mode, func(t *testing.T) {
			game := freshClient(t)
			outside := t.TempDir()
			foreign := filepath.Join(outside, "AddOns")
			if err := os.Mkdir(foreign, 0755); err != nil {
				t.Fatal(err)
			}
			u := newExtraUpdater(t, extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3")))
			info, err := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
			if err != nil {
				t.Fatal(err)
			}
			addons := filepath.Join(game, "Interface", "AddOns")
			var keep string
			u.rename = func(from, to string) error {
				if mode == "Interface replaced by link" {
					iface := filepath.Join(game, "Interface")
					os.Rename(iface, iface+"-moved")
					os.Symlink(outside, iface)
					keep = foreign
				} else {
					os.Rename(addons, addons+"-moved")
					os.Mkdir(addons, 0755)
					keep = addons
				}
				return errors.New("injected swap failure")
			}
			if _, err := u.applyExtra(context.Background(), game, info); err == nil {
				t.Fatal("injected failure not reported")
			}
			if st, err := os.Lstat(keep); err != nil || !st.IsDir() {
				t.Fatalf("cleanup removed %s, which this install did not create: %v", keep, err)
			}
		})
	}
}

// If creating Interface\AddOns fails after Interface was created, the bundle
// install removes Interface again.
func TestFirstInstallBundleParentCreationFailureCleansUp(t *testing.T) {
	game := freshClient(t)
	old := addonMkdir
	addonMkdir = func(path string, mode os.FileMode) error {
		if filepath.Base(path) == "AddOns" {
			return errors.New("injected mkdir failure")
		}
		return old(path, mode)
	}
	t.Cleanup(func() { addonMkdir = old })
	u := guildUpdaterForVersion(t, bundleZip(t, "1.5.0", nil), "1.5.0")
	if _, err := u.apply(context.Background(), game, guildInfoForVersion("1.5.0")); err == nil || !strings.Contains(err.Error(), "injected mkdir failure") {
		t.Fatalf("expected the mkdir failure, got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(game, "Interface")); !os.IsNotExist(err) {
		t.Fatalf("Interface left behind after a failed first install: %v", err)
	}
}

// Only real folders count as installed bundle folders.
func TestFirstInstallBundleStateRequiresFolders(t *testing.T) {
	for _, kind := range []string{"regular file", "dangling link"} {
		t.Run(kind, func(t *testing.T) {
			game := freshClient(t)
			addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Title: Relax Coach\n## Version: 1.5.0\n")
			for _, rel := range addonGuildRootsFor("1.5.0")[1:] {
				dest := filepath.Join(game, filepath.FromSlash(rel))
				os.MkdirAll(filepath.Dir(dest), 0755)
				if kind == "regular file" {
					os.WriteFile(dest, []byte("not a folder"), 0644)
				} else if err := os.Symlink(filepath.Join(t.TempDir(), "absent"), dest); err != nil {
					t.Skip(err)
				}
			}
			if s := InstalledBundleState(game); s.State != "partial" || len(s.MissingFolders) != 3 {
				t.Fatalf("%s counted as installed: %+v", kind, s)
			}
		})
	}
}
