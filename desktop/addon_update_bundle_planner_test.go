package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Relax Quest Planner joins the combined bundle at 1.5.0.
const plannerFixtureVersion = "0.1.2-test"

func plannerFixtureEntries() []addonZipEntry {
	root := addonPlannerRoot + "/"
	return []addonZipEntry{
		{root + "RelaxQuestPlanner.toc", "## Interface: 16001\n## Title: Relax Quest Planner\n## Version: " + plannerFixtureVersion + "\n## SavedVariablesPerCharacter: RelaxQuestPlannerDB\n## OptionalDeps: QuestieDB, Journey\n\nInit.lua\nCore\\Engine.lua\n", 0644},
		{root + "Init.lua", "-- planner init\r\n", 0644},
		{root + "Core/Engine.lua", "-- planner engine\x00\xff\r\n", 0644},
		{root + "Media/relax-icon.tga", "tga bytes\x00", 0644},
		{root + "README.md", "planner readme", 0644},
	}
}

func bundleZip(t *testing.T, version string, mutate func([]addonZipEntry, map[string]any) []addonZipEntry) []byte {
	t.Helper()
	entries := funModeEntries(version)
	manifest := guildManifest(entries)
	manifest["version"] = version
	if addonBundleRelease(version) {
		manifest["plannerVersion"] = plannerFixtureVersion
	}
	if mutate != nil {
		entries = mutate(entries, manifest)
		files := guildManifest(entries)["files"]
		manifest["files"] = files
	}
	return guildZip(t, entries, manifest)
}

// A tester's manually installed planner test patch (with a file the bundle no
// longer ships) and a fresh client without the folder.
func plannerManualPatch(t *testing.T, game string) map[string]string {
	t.Helper()
	addonPut(t, game, addonPlannerRoot+"/RelaxQuestPlanner.toc", "## Title: Relax Quest Planner\n## Version: 0.1.1-test\nInit.lua\n")
	addonPut(t, game, addonPlannerRoot+"/Init.lua", "-- tester's manual patch\x00\xff")
	addonPut(t, game, addonPlannerRoot+"/OnlyInOldPatch.lua", "-- must be kept in the backup")
	return addonTreeHashes(t, filepath.Join(game, filepath.FromSlash(addonPlannerRoot)))
}

func plannerInstalledMatches(t *testing.T, game string) {
	t.Helper()
	for _, e := range plannerFixtureEntries() {
		b, err := os.ReadFile(filepath.Join(game, filepath.FromSlash(e.name)))
		if err != nil || string(b) != e.body {
			t.Fatalf("planner file %s not installed exactly: %v", e.name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(game, filepath.FromSlash(addonPlannerRoot), "OnlyInOldPatch.lua")); !os.IsNotExist(err) {
		t.Fatalf("obsolete manual-patch file left in the installed planner: %v", err)
	}
}

func TestBundlePlannerFreshClientInstall(t *testing.T) {
	const version = "1.5.0"
	game := guildGame(t)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	plannerInstalledMatches(t, game)
	if !strings.Contains(result.Message, "Relax Quest Planner") {
		t.Fatalf("result message does not mention the planner: %q", result.Message)
	}
	if _, err := os.Stat(filepath.Join(result.BackupPath, filepath.FromSlash(addonPlannerRoot))); !os.IsNotExist(err) {
		t.Fatalf("invented a planner backup for a fresh client: %v", err)
	}
	if _, err := os.Stat(filepath.Join(game, "Interface/AddOns/Unrelated/keep.lua")); err != nil {
		t.Fatalf("unrelated addon touched: %v", err)
	}
}

func TestBundlePlannerManualPatchIsBackedUp(t *testing.T) {
	const version = "1.5.0"
	game := guildGame(t)
	before := plannerManualPatch(t, game)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	plannerInstalledMatches(t, game)
	backup := addonTreeHashes(t, filepath.Join(result.BackupPath, filepath.FromSlash(addonPlannerRoot)))
	if len(backup) != len(before) {
		t.Fatalf("backup inventory %v, want %v", backup, before)
	}
	for name, hash := range before {
		if backup[name] != hash {
			t.Fatalf("manual patch file %s not kept byte-for-byte in %s", name, result.BackupPath)
		}
	}
}

// A failure while placing the planner (the last tree) must put every tree
// back: the manual patch is restored, a fresh client has no planner folder.
func TestBundlePlannerRollbackRestoresPreviousState(t *testing.T) {
	const version = "1.5.0"
	for _, manual := range []bool{false, true} {
		name := "fresh"
		if manual {
			name = "manual-patch"
		}
		t.Run(name, func(t *testing.T) {
			game := guildGame(t)
			if manual {
				plannerManualPatch(t, game)
			}
			before := addonTreeHashes(t, game)
			u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
			target := filepath.Join(game, filepath.FromSlash(addonPlannerRoot))
			u.rename = func(from, to string) error {
				if to == target && strings.Contains(from, string(os.PathSeparator)+"staged"+string(os.PathSeparator)) {
					return errors.New("injected planner swap failure")
				}
				return os.Rename(from, to)
			}
			if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err == nil || !strings.Contains(err.Error(), "previous addon trees restored") {
				t.Fatalf("expected a restored rollback, got %v", err)
			}
			after := addonTreeHashes(t, game)
			delete(after, "RelaxCoachBackups") // directory walk only yields files
			for k := range after {
				if strings.HasPrefix(k, "RelaxCoachBackups/") {
					delete(after, k)
				}
			}
			if len(after) != len(before) {
				t.Fatalf("game tree after rollback differs: %d files, want %d", len(after), len(before))
			}
			for k, v := range before {
				if after[k] != v {
					t.Fatalf("not restored: %s", k)
				}
			}
			if _, err := os.Lstat(target); manual == os.IsNotExist(err) {
				t.Fatalf("planner folder presence after rollback wrong (manual=%v): %v", manual, err)
			}
			if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")); !os.IsNotExist(err) {
				t.Fatalf("lock left behind: %v", err)
			}
		})
	}
}

// Bundles before 1.5.0 never touch a manually installed planner.
func TestBundlePlannerLegacyGuildLeavesManualPatchAlone(t *testing.T) {
	const version = "1.4.9"
	game := guildGame(t)
	before := plannerManualPatch(t, game)
	u := guildUpdaterForVersion(t, funModeZip(t, funModeEntries(version), version), version)
	if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err != nil {
		t.Fatal(err)
	}
	after := addonTreeHashes(t, filepath.Join(game, filepath.FromSlash(addonPlannerRoot)))
	if len(after) != len(before) {
		t.Fatal("legacy guild update changed the planner folder")
	}
	for k, v := range before {
		if after[k] != v {
			t.Fatalf("legacy guild update changed %s", k)
		}
	}
}

func TestBundlePlannerFailsClosed(t *testing.T) {
	const version = "1.5.0"
	for _, tc := range []struct {
		name, want string
		mutate     func([]addonZipEntry, map[string]any) []addonZipEntry
	}{
		{"missing planner", "RelaxQuestPlanner.toc", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			out := []addonZipEntry{}
			for _, x := range e {
				if !strings.HasPrefix(x.name, addonPlannerRoot+"/") {
					out = append(out, x)
				}
			}
			return out
		}},
		{"manifest without planner version", "manifest", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			delete(m, "plannerVersion")
			return e
		}},
		{"wrong planner title", "Relax Quest Planner TOC", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			for i := range e {
				if strings.HasSuffix(e[i].name, "RelaxQuestPlanner.toc") {
					e[i].body = strings.Replace(e[i].body, "Title: Relax Quest Planner", "Title: Something Else", 1)
				}
			}
			return e
		}},
		{"planner version mismatch", "Relax Quest Planner TOC", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			m["plannerVersion"] = "9.9.9"
			return e
		}},
		{"planner requires another addon", "must not require", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			for i := range e {
				if strings.HasSuffix(e[i].name, "RelaxQuestPlanner.toc") {
					e[i].body = strings.Replace(e[i].body, "## OptionalDeps:", "## Dependencies: Journey\n## OptionalDeps:", 1)
				}
			}
			return e
		}},
		{"planner load file missing", "load file", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			out := []addonZipEntry{}
			for _, x := range e {
				if !strings.HasSuffix(x.name, "Core/Engine.lua") {
					out = append(out, x)
				}
			}
			return out
		}},
		{"second planner TOC", "only RelaxQuestPlanner.toc", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{addonPlannerRoot + "/RelaxQuestPlanner_Vanilla.toc", "## Title: Relax Quest Planner\n## Version: 0.0.1\nInit.lua\n", 0644})
		}},
		{"nested planner TOC", "only RelaxQuestPlanner.toc", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{addonPlannerRoot + "/Libs/Other/Other.toc", "## Title: Other\n", 0644})
		}},
		{"second Journey TOC", "only Journey.toc", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{"Interface/AddOns/Journey/Journey_Vanilla.toc", "## Title: Relax Journey\n", 0644})
		}},
		{"case-variant planner TOC", "only RelaxQuestPlanner.toc", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{addonPlannerRoot + "/RelaxQuestPlanner_TBC.TOC", "## Title: x\n", 0644})
		}},
		{"executable in planner folder", "outside the fixed guild allowlist", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{addonPlannerRoot + "/tool.exe", "MZ", 0644})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			game := guildGame(t)
			before := addonTreeHashes(t, game)
			u := guildUpdaterForVersion(t, bundleZip(t, version, tc.mutate), version)
			_, err := u.apply(context.Background(), game, guildInfoForVersion(version))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
			after := addonTreeHashes(t, game)
			for k, v := range before {
				if after[k] != v {
					t.Fatalf("game changed on refusal: %s", k)
				}
			}
		})
	}
}
