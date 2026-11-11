package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// First install from the app: the same updater transaction installs into a
// client that has no Relax Coach folders (or no Interface\AddOns at all).

func freshClient(t *testing.T) string {
	t.Helper()
	game := t.TempDir()
	addonPut(t, game, "WTF/Config.wtf", "SET locale \"enUS\"\n")
	return game
}

func TestFirstInstallBundleState(t *testing.T) {
	coach := func(v string) string { return "## Title: Relax Coach\n## Version: " + v + "\n" }
	cases := []struct {
		name, state      string
		install          bool
		files            map[string]string
		present, missing []string
	}{
		{"no Interface folder", "none", true, nil, []string{}, []string{"ForeverCoach", "Journey", "Journey-Content", "RelaxQuestPlanner"}},
		{"only unrelated addons", "none", true, map[string]string{"Interface/AddOns/Other/Other.toc": "x"}, []string{}, []string{"ForeverCoach", "Journey", "Journey-Content", "RelaxQuestPlanner"}},
		{"normal Journey only", "partial", true, map[string]string{"Interface/AddOns/Journey/Journey.toc": "## Title: Journey\n"}, []string{"Journey"}, []string{"ForeverCoach", "Journey-Content", "RelaxQuestPlanner"}},
		{"Coach folder without readable TOC", "partial", true, map[string]string{"Interface/AddOns/ForeverCoach/Core.lua": "x"}, []string{"ForeverCoach"}, []string{"Journey", "Journey-Content", "RelaxQuestPlanner"}},
		{"1.5.0 without content and planner", "partial", false, map[string]string{"Interface/AddOns/ForeverCoach/ForeverCoach.toc": coach("1.5.0"), "Interface/AddOns/Journey/Journey.toc": "## Title: Relax Journey\n"}, []string{"ForeverCoach", "Journey"}, []string{"Journey-Content", "RelaxQuestPlanner"}},
		{"1.4.5 guild install", "installed", false, map[string]string{"Interface/AddOns/ForeverCoach/ForeverCoach.toc": coach("1.4.5"), "Interface/AddOns/Journey/Journey.toc": "## Title: Relax Journey\n", "Interface/Content/Journey-Content/a.html": "x"}, []string{"ForeverCoach", "Journey", "Journey-Content"}, []string{}},
		{"1.5.0 bundle install", "installed", false, map[string]string{"Interface/AddOns/ForeverCoach/ForeverCoach.toc": coach("1.5.0"), "Interface/AddOns/Journey/Journey.toc": "## Title: Relax Journey\n", "Interface/AddOns/RelaxQuestPlanner/RelaxQuestPlanner.toc": "x", "Interface/Content/Journey-Content/a.html": "x"}, []string{"ForeverCoach", "Journey", "Journey-Content", "RelaxQuestPlanner"}, []string{}},
		{"legacy Coach-only 1.3.1", "installed", false, map[string]string{"Interface/AddOns/ForeverCoach/ForeverCoach.toc": coach("1.3.1")}, []string{"ForeverCoach"}, []string{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			game := freshClient(t)
			for name, body := range c.files {
				addonPut(t, game, name, body)
			}
			got := InstalledBundleState(game)
			if got.State != c.state || got.Install != c.install || !reflect.DeepEqual(got.PresentFolders, c.present) || !reflect.DeepEqual(got.MissingFolders, c.missing) {
				t.Fatalf("state %+v, want %s install=%v present=%v missing=%v", got, c.state, c.install, c.present, c.missing)
			}
		})
	}
	if got := InstalledBundleState(""); got.State != "none" || !got.Install {
		t.Fatalf("no game folder: %+v", got)
	}
}

func TestFirstInstallFreshClientInstallsAllFourFolders(t *testing.T) {
	const version = "1.5.0"
	game := freshClient(t)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatalf("first install into a client without Interface\\AddOns: %v", err)
	}
	for _, root := range addonGuildRootsFor(version) {
		if st, err := os.Lstat(filepath.Join(game, filepath.FromSlash(root))); err != nil || !st.IsDir() {
			t.Fatalf("%s not installed: %v", root, err)
		}
	}
	plannerInstalledMatches(t, game)
	if !strings.Contains(result.Message, version+" installed") || strings.Contains(result.Message, "updated") {
		t.Fatalf("first install must say installed, got %q", result.Message)
	}
	if result.BackupPath != "" {
		t.Fatalf("a fresh client has nothing to back up, got %q", result.BackupPath)
	}
	if s := InstalledBundleState(game); s.State != "installed" || s.Install {
		t.Fatalf("after install: %+v", s)
	}
	if b, err := os.ReadFile(filepath.Join(game, "WTF/Config.wtf")); err != nil || !strings.Contains(string(b), "enUS") {
		t.Fatal("WTF touched")
	}
}

// A failed first install leaves the client exactly as it was: no Interface
// folder appears, nothing is half installed, the lock is gone.
func TestFirstInstallFreshClientRollbackRemovesCreatedFolders(t *testing.T) {
	const version = "1.5.0"
	game := freshClient(t)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	target := filepath.Join(game, filepath.FromSlash(addonPlannerRoot))
	u.rename = func(from, to string) error {
		if to == target {
			return errors.New("injected planner swap failure")
		}
		return os.Rename(from, to)
	}
	if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err == nil || !strings.Contains(err.Error(), "previous addon trees restored") {
		t.Fatalf("expected a restored rollback, got %v", err)
	}
	if _, err := os.Lstat(filepath.Join(game, "Interface")); !os.IsNotExist(err) {
		t.Fatalf("rollback left an Interface folder behind: %v", err)
	}
	if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock left behind: %v", err)
	}
}

// A normal (non-Relax) Journey is moved into the backup by the first install.
func TestFirstInstallOverNormalJourneyKeepsItInBackup(t *testing.T) {
	const version = "1.5.0"
	game := freshClient(t)
	addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Journey\n## Version: 9.9.9\n")
	if s := InstalledBundleState(game); s.State != "partial" || !s.Install {
		t.Fatalf("before: %+v", s)
	}
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "installed") || !strings.Contains(result.Message, "kept in the backup") {
		t.Fatalf("message %q", result.Message)
	}
	b, err := os.ReadFile(filepath.Join(result.BackupPath, "Interface/AddOns/Journey/Journey.toc"))
	if err != nil || !strings.Contains(string(b), "9.9.9") {
		t.Fatalf("normal Journey not in the backup: %v", err)
	}
}

func TestFirstInstallExistingClientStillSaysUpdated(t *testing.T) {
	const version = "1.5.0"
	game := guildGame(t)
	addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Relax Journey\n")
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Message, "updated together") || strings.Contains(result.Message, " installed;") {
		t.Fatalf("update message %q", result.Message)
	}
}

// More addons follow the same rule on a client without Interface\AddOns.
func TestFirstInstallExtraAddonIntoFreshClient(t *testing.T) {
	for _, fail := range []bool{false, true} {
		game := freshClient(t)
		u := newExtraUpdater(t, extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3")))
		if fail {
			u.rename = func(string, string) error { return errors.New("injected swap failure") }
		}
		info, err := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
		if err != nil || !info.Install {
			t.Fatalf("check %+v %v", info, err)
		}
		res, err := u.applyExtra(context.Background(), game, info)
		if fail {
			if err == nil {
				t.Fatal("injected failure not reported")
			}
			if _, err := os.Lstat(filepath.Join(game, "Interface")); !os.IsNotExist(err) {
				t.Fatalf("failed install left an Interface folder: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("install into a client without Interface\\AddOns: %v", err)
		}
		if !strings.Contains(res.Message, "installed") {
			t.Fatalf("message %q", res.Message)
		}
		a, _ := extraAddonByID("forevermaps")
		if v := InstalledExtraVersion(game, a); v != "0.1.3" {
			t.Fatalf("installed version %s", v)
		}
	}
}

func TestFirstInstallRefusesLinkedAddOnsParent(t *testing.T) {
	game := freshClient(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(game, "Interface")); err != nil {
		t.Skip(err)
	}
	if _, err := addonMissingDirs(game, filepath.Join(game, "Interface", "AddOns")); err == nil {
		t.Fatal("a linked Interface folder was accepted")
	}
	const version = "1.5.0"
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err == nil {
		t.Fatal("installed through a linked Interface folder")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote outside the game folder: %v", entries)
	}
}
