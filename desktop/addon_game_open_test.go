//go:build !windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

const (
	adviceReload  = "Done. Type /reload in game."
	adviceRestart = "Done. Exit WoW completely and start it again (/reload is not enough for new files)."
	adviceStart   = "Done. Start WoW to load it."
	adviceClose   = "Close WoW and try again"
)

func wowOpen(context.Context) (bool, error)   { return true, nil }
func wowClosed(context.Context) (bool, error) { return false, nil }

// bundleInstall installs version on game with WoW reported by running.
func bundleInstall(t *testing.T, game, version string, running func(context.Context) (bool, error), mutate func([]addonZipEntry, map[string]any) []addonZipEntry) (AddonUpdateResult, error) {
	t.Helper()
	u := guildUpdaterForVersion(t, bundleZip(t, version, mutate), version)
	u.running = running
	return u.apply(context.Background(), game, guildInfoForVersion(version))
}

func addExtraLua(entries []addonZipEntry, _ map[string]any) []addonZipEntry {
	return append(entries, addonZipEntry{"Interface/AddOns/Journey/NewModule.lua", "-- new file\n", 0644})
}

// The bundle installs while WoW is running, and says afterwards whether
// /reload is enough: only when every root existed and no new file path came.
func TestGameOpenBundleInstallsAndAdvises(t *testing.T) {
	t.Run("first install while WoW runs needs a restart", func(t *testing.T) {
		game := freshClient(t)
		res, err := bundleInstall(t, game, "1.5.1", wowOpen, nil)
		if err != nil {
			t.Fatalf("refused while WoW runs: %v", err)
		}
		if !strings.HasSuffix(res.Message, adviceRestart) {
			t.Fatalf("message %q", res.Message)
		}
	})
	t.Run("same file list needs only /reload", func(t *testing.T) {
		game := freshClient(t)
		if _, err := bundleInstall(t, game, "1.5.1", wowClosed, nil); err != nil {
			t.Fatal(err)
		}
		res, err := bundleInstall(t, game, "1.5.2", wowOpen, nil)
		if err != nil {
			t.Fatalf("refused while WoW runs: %v", err)
		}
		if !strings.HasSuffix(res.Message, adviceReload) || InstalledCoachVersion(game) != "1.5.2" {
			t.Fatalf("message %q, installed %s", res.Message, InstalledCoachVersion(game))
		}
	})
	t.Run("a new file path needs a restart", func(t *testing.T) {
		game := freshClient(t)
		if _, err := bundleInstall(t, game, "1.5.1", wowClosed, nil); err != nil {
			t.Fatal(err)
		}
		res, err := bundleInstall(t, game, "1.5.2", wowOpen, addExtraLua)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(res.Message, adviceRestart) {
			t.Fatalf("message %q", res.Message)
		}
	})
	t.Run("a new root folder needs a restart", func(t *testing.T) {
		game := freshClient(t)
		if _, err := bundleInstall(t, game, "1.5.1", wowClosed, nil); err != nil {
			t.Fatal(err)
		}
		os.RemoveAll(filepath.Join(game, filepath.FromSlash(addonPlannerRoot)))
		res, err := bundleInstall(t, game, "1.5.2", wowOpen, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(res.Message, adviceRestart) {
			t.Fatalf("message %q", res.Message)
		}
	})
	t.Run("a path that only differs in case is not new", func(t *testing.T) {
		game := freshClient(t)
		if _, err := bundleInstall(t, game, "1.5.1", wowClosed, nil); err != nil {
			t.Fatal(err)
		}
		toc := filepath.Join(game, "Interface", "AddOns", "Journey", "Journey.toc")
		if err := os.Rename(toc, filepath.Join(filepath.Dir(toc), "journey.TOC")); err != nil {
			t.Fatal(err)
		}
		if got, _ := addonHasNewPaths(filepath.Dir(toc), map[string]addonFileDigest{"Journey.toc": {}}); got {
			t.Fatal("case-only difference counted as a new file (Windows paths ignore case)")
		}
	})
	t.Run("WoW closed: start it", func(t *testing.T) {
		game := freshClient(t)
		res, err := bundleInstall(t, game, "1.5.1", wowClosed, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(res.Message, adviceStart) {
			t.Fatalf("message %q", res.Message)
		}
	})
}

// A move refused because a file is in use rolls the whole bundle back and is
// the only case that asks to close WoW.
func TestGameOpenBundleFileInUseRollsBack(t *testing.T) {
	game := freshClient(t)
	if _, err := bundleInstall(t, game, "1.5.1", wowClosed, nil); err != nil {
		t.Fatal(err)
	}
	before := addonTreeHashes(t, game)
	u := guildUpdaterForVersion(t, bundleZip(t, "1.5.2", nil), "1.5.2")
	u.running = wowOpen
	moves := 0
	u.rename = func(a, b string) error {
		moves++
		if moves == 3 { // Journey moved aside after Coach was swapped
			return &os.LinkError{Op: "rename", Old: a, New: b, Err: syscall.EBUSY}
		}
		return os.Rename(a, b)
	}
	_, err := u.apply(context.Background(), game, guildInfoForVersion("1.5.2"))
	if err == nil || !strings.HasPrefix(err.Error(), adviceClose) {
		t.Fatalf("in-use failure: %v", err)
	}
	if after := addonTreeHashes(t, game); !reflect.DeepEqual(before, after) {
		t.Fatal("not rolled back")
	}
	if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")); !os.IsNotExist(err) {
		t.Fatal("lock left behind after a clean rollback")
	}
}

// Any other failure keeps its own message (no "close WoW").
func TestGameOpenOtherFailureDoesNotAskToCloseWoW(t *testing.T) {
	game := freshClient(t)
	if _, err := bundleInstall(t, game, "1.5.1", wowClosed, nil); err != nil {
		t.Fatal(err)
	}
	u := guildUpdaterForVersion(t, bundleZip(t, "1.5.2", nil), "1.5.2")
	u.running = wowOpen
	u.rename = func(a, b string) error { return &os.LinkError{Op: "rename", Old: a, New: b, Err: syscall.ENOSPC} }
	_, err := u.apply(context.Background(), game, guildInfoForVersion("1.5.2"))
	if err == nil || strings.Contains(err.Error(), adviceClose) {
		t.Fatalf("got %v", err)
	}
}

func extraInstall(t *testing.T, game, version string, payload []byte, running func(context.Context) (bool, error)) (AddonUpdateResult, error) {
	t.Helper()
	u := newExtraUpdater(t, extraRelease(t, version, payload))
	installed := "0.0.0"
	if s, err := extraInstalled(game, extraAddons[1]); err == nil && s.exists {
		installed = s.version.text
	}
	info, err := u.checkExtra(context.Background(), "forevermaps", installed)
	if err != nil {
		t.Fatal(err)
	}
	u.running = running
	return u.applyExtra(context.Background(), game, info)
}

func TestGameOpenExtraAddonInstallsAndAdvises(t *testing.T) {
	game := extraGame(t)
	res, err := extraInstall(t, game, "0.1.3", extraGoodZip(t, "0.1.3"), wowOpen)
	if err != nil {
		t.Fatalf("refused while WoW runs: %v", err)
	}
	if !strings.HasSuffix(res.Message, adviceRestart) {
		t.Fatalf("first install message %q", res.Message)
	}
	res, err = extraInstall(t, game, "0.1.4", extraGoodZip(t, "0.1.4"), wowOpen)
	if err != nil || !strings.HasSuffix(res.Message, adviceReload) {
		t.Fatalf("same files: %v %q", err, res.Message)
	}
	more := extraZip(t, map[string]string{
		"ForeverMaps/ForeverMaps.toc": strings.Replace(extraTestTOC, "%s", "0.1.5", 1),
		"ForeverMaps/Core.lua":        "-- core",
		"ForeverMaps/Data/World.lua":  "-- world",
		"ForeverMaps/Data/Zones.lua":  "-- new",
	})
	res, err = extraInstall(t, game, "0.1.5", more, wowOpen)
	if err != nil || !strings.HasSuffix(res.Message, adviceRestart) {
		t.Fatalf("new file: %v %q", err, res.Message)
	}
}

func TestGameOpenExtraAddonFileInUseRestores(t *testing.T) {
	game := extraGame(t)
	if _, err := extraInstall(t, game, "0.1.3", extraGoodZip(t, "0.1.3"), wowClosed); err != nil {
		t.Fatal(err)
	}
	before := addonTreeHashes(t, game)
	u := newExtraUpdater(t, extraRelease(t, "0.1.4", extraGoodZip(t, "0.1.4")))
	info, _ := u.checkExtra(context.Background(), "forevermaps", "0.1.3")
	u.running = wowOpen
	for _, failAt := range []int{1, 2} {
		moves := 0
		u.rename = func(a, b string) error {
			moves++
			if moves == failAt {
				return &os.LinkError{Op: "rename", Old: a, New: b, Err: syscall.EBUSY}
			}
			return os.Rename(a, b)
		}
		_, err := u.applyExtra(context.Background(), game, info)
		if err == nil || !strings.HasPrefix(err.Error(), adviceClose) {
			t.Fatalf("fail at move %d: %v", failAt, err)
		}
		if after := addonTreeHashes(t, game); !reflect.DeepEqual(before, after) {
			t.Fatalf("fail at move %d: not restored", failAt)
		}
	}
}
