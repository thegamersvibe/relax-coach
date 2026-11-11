package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// مانع should-fix 4a: an empty Journey/planner folder or a linked Content
// parent is not an installed bundle folder.
func TestFIBundleStateEmptyFoldersAndLinkedContent(t *testing.T) {
	game := freshClient(t)
	addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Title: Relax Coach\n## Version: 1.5.0\n")
	os.MkdirAll(filepath.Join(game, "Interface/AddOns/Journey"), 0755)
	os.MkdirAll(filepath.Join(game, "Interface/AddOns/RelaxQuestPlanner"), 0755)
	outside := t.TempDir()
	addonPut(t, outside, "Journey-Content/a.html", "x")
	if err := os.Symlink(outside, filepath.Join(game, "Interface", "Content")); err != nil {
		t.Skip(err)
	}
	s := InstalledBundleState(game)
	if s.State != "partial" || !reflect.DeepEqual(s.MissingFolders, []string{"Journey", "Journey-Content", "RelaxQuestPlanner"}) {
		t.Fatalf("empty folders / linked Content counted as installed: %+v", s)
	}
}

// مانع should-fix 4b: More addons re-check the parents between moving the old
// version aside and publishing the new one, and after publishing; a changed
// path is never reported as success, and the lock and backup are kept.
func TestFIExtraSwapRechecksParents(t *testing.T) {
	for _, when := range []string{"after backup move", "after publish"} {
		t.Run(when, func(t *testing.T) {
			game := extraGame(t)
			u0 := newExtraUpdater(t, extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3")))
			info, _ := u0.checkExtra(context.Background(), "forevermaps", "0.0.0")
			if _, err := u0.applyExtra(context.Background(), game, info); err != nil {
				t.Fatal(err)
			}
			outside := t.TempDir()
			addons := filepath.Join(game, "Interface", "AddOns")
			swap := func() error {
				if err := os.Rename(addons, addons+"-moved"); err != nil {
					return err
				}
				return os.Symlink(outside, addons)
			}
			u := newExtraUpdater(t, extraRelease(t, "0.2.0", extraGoodZip(t, "0.2.0")))
			info, err := u.checkExtra(context.Background(), "forevermaps", "0.1.3")
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			u.rename = func(from, to string) error {
				calls++
				if err := os.Rename(from, to); err != nil {
					return err
				}
				if (when == "after backup move" && calls == 1) || (when == "after publish" && calls == 2) {
					return swap()
				}
				return nil
			}
			res, err := u.applyExtra(context.Background(), game, info)
			if err == nil {
				t.Fatalf("reported success through a changed path: %+v", res)
			}
			if when == "after backup move" {
				if entries, _ := os.ReadDir(outside); len(entries) != 0 {
					t.Fatalf("published outside the game folder: %v", entries)
				}
			}
			if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".extra-addon-update.lock")); err != nil {
				t.Fatalf("lock not kept after a changed path: %v", err)
			}
			prev, _ := filepath.Glob(filepath.Join(game, "RelaxCoachBackups", "ForeverMaps-*", "previous", "Core.lua"))
			if len(prev) != 1 {
				t.Fatalf("backup of the previous version not kept: %v", prev)
			}
		})
	}
}

// مانع should-fix 4c: a folder swapped in right after the install created it
// (here one that already holds a player's file) is not adopted, and nothing
// the player put there is removed.
func TestFICreatedFolderSwappedAfterMkdir(t *testing.T) {
	for _, bundle := range []bool{false, true} {
		game := freshClient(t)
		addons := filepath.Join(game, "Interface", "AddOns")
		withMkdirHook(t, func(path string) error {
			if path != addons {
				return nil
			}
			if err := os.Rename(path, path+"-ours"); err != nil {
				return err
			}
			if err := os.Mkdir(path, 0755); err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(path, "player.txt"), []byte("mine"), 0644)
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
		addonMkdir = os.Mkdir
		if err == nil {
			t.Fatalf("bundle=%v: installed into a folder swapped in after Mkdir", bundle)
		}
		if b, e := os.ReadFile(filepath.Join(addons, "player.txt")); e != nil || string(b) != "mine" {
			t.Fatalf("bundle=%v: the player's folder was changed: %v", bundle, e)
		}
		if entries, _ := os.ReadDir(addons); len(entries) != 1 {
			t.Fatalf("bundle=%v: wrote into the swapped-in folder: %v", bundle, entries)
		}
	}
}

var _ = errors.New
