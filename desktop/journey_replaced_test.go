//go:build !windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Journey replaced by another updater (CurseForge, Wago, Journey Guide
// Manager): the Journey folder is there but its TOC Title is not ours.

func journeyFolder(game string) string {
	return filepath.Join(game, "Interface", "AddOns", "Journey")
}

// foreignJourney replaces the installed Journey with another updater's copy.
func foreignJourney(t *testing.T, game, toc string) {
	t.Helper()
	dir := journeyFolder(game)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", toc)
	addonPut(t, game, "Interface/AddOns/Journey/Upstream.lua", "-- from CurseForge\n")
}

func installedBundle(t *testing.T, version string) string {
	t.Helper()
	game := freshClient(t)
	if _, err := bundleInstall(t, game, version, wowClosed, nil); err != nil {
		t.Fatal(err)
	}
	return game
}

func TestJourneyReplacedState(t *testing.T) {
	game := installedBundle(t, "1.5.2")
	if s := InstalledBundleState(game); s.State != "installed" {
		t.Fatalf("our Journey: %+v", s)
	}
	for name, toc := range map[string]string{
		"guide manager Journey": "## Title: Journey\r\n## Version: 4.00.09\r\n",
		"colored other title":   "## Title: |cff00ff00Journey|r XP\n## Version: 1.0\n",
		"no title":              "## Version: 1.0\n",
	} {
		t.Run(name, func(t *testing.T) {
			game := installedBundle(t, "1.5.2")
			foreignJourney(t, game, toc)
			s := InstalledBundleState(game)
			if s.State != "replaced" || s.Install {
				t.Fatalf("foreign Journey: %+v", s)
			}
			if !addonRepairNeeded(game) {
				t.Fatal("a replaced Journey must be repairable at the same version")
			}
		})
	}
	t.Run("Journey.toc missing in the folder", func(t *testing.T) {
		game := installedBundle(t, "1.5.2")
		os.Remove(filepath.Join(journeyFolder(game), "Journey.toc"))
		if s := InstalledBundleState(game); s.State != "replaced" {
			t.Fatalf("%+v", s)
		}
	})
	t.Run("Journey missing is partial, not replaced", func(t *testing.T) {
		game := installedBundle(t, "1.5.2")
		os.RemoveAll(journeyFolder(game))
		if s := InstalledBundleState(game); s.State != "partial" {
			t.Fatalf("%+v", s)
		}
	})
	t.Run("no Coach: a stock Journey is a first install, not replaced", func(t *testing.T) {
		game := freshClient(t)
		foreignJourney(t, game, "## Title: Journey\n## Version: 4.00.09\n")
		if s := InstalledBundleState(game); s.State != "partial" || !s.Install {
			t.Fatalf("%+v", s)
		}
	})
}

// Repair at the same version restores Relax Journey and keeps the other
// updater's folder in the backup; nothing is deleted.
func TestJourneyReplacedRepairKeepsForeignFolder(t *testing.T) {
	const version = "1.5.2"
	game := installedBundle(t, version)
	foreignJourney(t, game, "## Title: Journey\r\n## Version: 4.00.09\r\n")
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	u.running = wowOpen
	a := &App{}
	a.setGame(game)
	b := newNativeBridge(a, false, nativeServices{check: func(context.Context, string) (AddonReleaseInfo, error) {
		info := guildInfoForVersion(version)
		none := addonNoUpdate(info, "current")
		info.Repair = true
		none.sameVersion = &info
		return none, nil
	}, apply: u.apply})
	defer b.Close()
	v, err := b.Call("checkAddon", nil)
	o := v.(nativeOffer)
	if err != nil || !o.Available || !o.Repair || o.Bundle.State != "replaced" {
		t.Fatalf("expected a Repair offer for the replaced Journey, got %+v %v", o, err)
	}
	v, err = b.Call("applyAddon", nativeArgs(version))
	if err != nil {
		t.Fatal(err)
	}
	res := v.(AddonUpdateResult)
	if s := InstalledBundleState(game); s.State != "installed" {
		t.Fatalf("after Repair: %+v", s)
	}
	kept, err := os.ReadFile(filepath.Join(res.BackupPath, "Interface", "AddOns", "Journey", "Upstream.lua"))
	if err != nil || string(kept) != "-- from CurseForge\n" {
		t.Fatalf("the other updater's Journey is not in the backup %s: %v", res.BackupPath, err)
	}
	for _, want := range []string{"Relax Journey restored", "turn off auto-update for Journey"} {
		if !strings.Contains(res.Message, want) {
			t.Fatalf("message %q lacks %q", res.Message, want)
		}
	}
}
