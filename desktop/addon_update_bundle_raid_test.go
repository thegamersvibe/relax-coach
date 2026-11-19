package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Relax Raid joins the combined bundle at addonRaidMinimumVersion (PLAN R9a: the app knows the folder before
// any bundle carries it; R9b's bundle comes after). These tests use the gate version itself.
const raidFixtureVersion = "0.1.0"

func raidFixtureEntries() []addonZipEntry {
	root := addonRaidRoot + "/"
	return []addonZipEntry{
		{root + "RelaxRaid.toc", "## Interface: 16001\n## Title: Relax Raid\n## Version: " + raidFixtureVersion + "\n## SavedVariables: RelaxRaidDB\n## SavedVariablesPerCharacter: RelaxRaidCharDB\n\nCore.lua\nLog.lua\n", 0644},
		{root + "Core.lua", "-- raid core\r\n", 0644},
		{root + "Log.lua", "-- raid log\x00\xff\r\n", 0644},
	}
}

// An earlier, manually installed RelaxRaid (with a file the bundle does not ship) and the player's saved
// variables, which live under WTF and must never be touched.
func raidManualCopy(t *testing.T, game string) map[string]string {
	t.Helper()
	addonPut(t, game, addonRaidRoot+"/RelaxRaid.toc", "## Title: Relax Raid\n## Version: 0.0.9-test\nCore.lua\n")
	addonPut(t, game, addonRaidRoot+"/Core.lua", "-- tester's copy\x00\xff")
	addonPut(t, game, addonRaidRoot+"/OnlyInOldCopy.lua", "-- must be kept in the backup")
	return addonTreeHashes(t, filepath.Join(game, filepath.FromSlash(addonRaidRoot)))
}

func raidSavedVariables(t *testing.T, game string) map[string]string {
	t.Helper()
	addonPut(t, game, "WTF/Account/FAKEACCOUNT/SavedVariables/RelaxRaid.lua", "RelaxRaidDB = {[\"nights\"] = {}}\n")
	addonPut(t, game, "WTF/Account/FAKEACCOUNT/Realm/Char/SavedVariables/RelaxRaid.lua", "RelaxRaidCharDB = {[\"logger\"] = true}\n")
	return addonTreeHashes(t, filepath.Join(game, "WTF"))
}

func sameHashes(t *testing.T, what string, got, want map[string]string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: %d files, want %d", what, len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s: %s differs", what, k)
		}
	}
}

func raidInstalledMatches(t *testing.T, game string) {
	t.Helper()
	for _, e := range raidFixtureEntries() {
		b, err := os.ReadFile(filepath.Join(game, filepath.FromSlash(e.name)))
		if err != nil || string(b) != e.body {
			t.Fatalf("raid file %s not installed exactly: %v", e.name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(game, filepath.FromSlash(addonRaidRoot), "OnlyInOldCopy.lua")); !os.IsNotExist(err) {
		t.Fatalf("obsolete file left in the installed RelaxRaid: %v", err)
	}
}

func TestRaidGateAndAssetName(t *testing.T) {
	for v, want := range map[string]string{
		addonRaidMinimumVersion: "RelaxCoach-RaidBundle-" + addonRaidMinimumVersion + ".zip",
		"2.0.0":                 "RelaxCoach-RaidBundle-2.0.0.zip",
		"1.7.2":                 "RelaxCoach-Bundle-1.7.2.zip",
		"1.7.10":                "RelaxCoach-Bundle-1.7.10.zip",
	} {
		if got := addonAssetName(v); got != want {
			t.Fatalf("addonAssetName(%s) = %s, want %s", v, got, want)
		}
	}
	if roots := addonGuildRootsFor("1.7.2"); len(roots) != 4 || roots[3] != addonPlannerRoot {
		t.Fatalf("1.7.2 roots changed: %v", roots)
	}
	if roots := addonGuildRootsFor(addonRaidMinimumVersion); len(roots) != 5 || roots[4] != addonRaidRoot {
		t.Fatalf("gate roots: %v", roots)
	}
}

func TestBundleRaidFreshClientInstall(t *testing.T) {
	version := addonRaidMinimumVersion
	game := guildGame(t)
	saved := raidSavedVariables(t, game)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	raidInstalledMatches(t, game)
	plannerInstalledMatches(t, game)
	if !strings.Contains(result.Message, "Relax Quest Planner, Relax Raid and guide content") {
		t.Fatalf("result message does not name Relax Raid: %q", result.Message)
	}
	if _, err := os.Stat(filepath.Join(result.BackupPath, filepath.FromSlash(addonRaidRoot))); !os.IsNotExist(err) {
		t.Fatalf("invented a RelaxRaid backup for a fresh client: %v", err)
	}
	sameHashes(t, "WTF", addonTreeHashes(t, filepath.Join(game, "WTF")), saved)
	if _, err := os.Stat(filepath.Join(game, "Interface/AddOns/Unrelated/keep.lua")); err != nil {
		t.Fatalf("unrelated addon touched: %v", err)
	}
}

func TestBundleRaidExistingFolderIsBackedUp(t *testing.T) {
	version := addonRaidMinimumVersion
	game := guildGame(t)
	before := raidManualCopy(t, game)
	saved := raidSavedVariables(t, game)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	raidInstalledMatches(t, game)
	sameHashes(t, "RelaxRaid backup", addonTreeHashes(t, filepath.Join(result.BackupPath, filepath.FromSlash(addonRaidRoot))), before)
	sameHashes(t, "WTF", addonTreeHashes(t, filepath.Join(game, "WTF")), saved)
}

// A failure while placing RelaxRaid (the last tree) puts every tree back: an earlier copy is restored, a
// fresh client has no RelaxRaid folder, the saved variables are untouched.
func TestBundleRaidRollbackRestoresPreviousState(t *testing.T) {
	version := addonRaidMinimumVersion
	for _, manual := range []bool{false, true} {
		name := "fresh"
		if manual {
			name = "earlier-copy"
		}
		t.Run(name, func(t *testing.T) {
			game := guildGame(t)
			if manual {
				raidManualCopy(t, game)
			}
			raidSavedVariables(t, game)
			before := addonTreeHashes(t, game)
			u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
			target := filepath.Join(game, filepath.FromSlash(addonRaidRoot))
			u.rename = func(from, to string) error {
				if to == target && strings.Contains(from, string(os.PathSeparator)+"staged"+string(os.PathSeparator)) {
					return errors.New("injected RelaxRaid swap failure")
				}
				return os.Rename(from, to)
			}
			if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err == nil || !strings.Contains(err.Error(), "previous addon trees restored") {
				t.Fatalf("expected a restored rollback, got %v", err)
			}
			after := addonTreeHashes(t, game)
			for k := range after {
				if strings.HasPrefix(k, "RelaxCoachBackups/") {
					delete(after, k)
				}
			}
			sameHashes(t, "game tree after rollback", after, before)
			if _, err := os.Lstat(target); manual == os.IsNotExist(err) {
				t.Fatalf("RelaxRaid presence after rollback wrong (earlier copy=%v): %v", manual, err)
			}
		})
	}
}

// An old bundle (below the gate) on the new app installs as before and never touches a RelaxRaid folder.
func TestBundleRaidOldBundleOnNewApp(t *testing.T) {
	const version = "1.7.2"
	game := guildGame(t)
	before := raidManualCopy(t, game)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	plannerInstalledMatches(t, game)
	sameHashes(t, "RelaxRaid", addonTreeHashes(t, filepath.Join(game, filepath.FromSlash(addonRaidRoot))), before)
	if strings.Contains(result.Message, "Relax Raid") {
		t.Fatalf("an old bundle's message names Relax Raid: %q", result.Message)
	}
}

func raidWithout(e []addonZipEntry) []addonZipEntry {
	out := []addonZipEntry{}
	for _, x := range e {
		if !strings.HasPrefix(x.name, addonRaidRoot+"/") {
			out = append(out, x)
		}
	}
	return out
}

func raidEditTOC(e []addonZipEntry, old, new string) []addonZipEntry {
	for i := range e {
		if strings.HasSuffix(e[i].name, "RelaxRaid.toc") {
			e[i].body = strings.Replace(e[i].body, old, new, 1)
		}
	}
	return e
}

func TestBundleRaidFailsClosed(t *testing.T) {
	gate := addonRaidMinimumVersion
	for _, tc := range []struct {
		name, version, want string
		mutate              func([]addonZipEntry, map[string]any) []addonZipEntry
	}{
		{"gate bundle without RelaxRaid", gate, "incomplete guild archive: missing Interface/AddOns/RelaxRaid/RelaxRaid.toc", func(e []addonZipEntry, m map[string]any) []addonZipEntry { return raidWithout(e) }},
		{"manifest without raidVersion", gate, "manifest", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			delete(m, "raidVersion")
			return e
		}},
		{"old bundle carrying RelaxRaid", "1.7.2", "outside the fixed guild allowlist", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, raidFixtureEntries()...)
		}},
		{"old bundle naming a raidVersion", "1.7.2", "manifest", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			m["raidVersion"] = raidFixtureVersion
			return e
		}},
		{"wrong title", gate, "Relax Raid TOC", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return raidEditTOC(e, "Title: Relax Raid", "Title: Something Else")
		}},
		{"wrong account saved variables", gate, "Relax Raid TOC", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return raidEditTOC(e, "SavedVariables: RelaxRaidDB", "SavedVariables: OtherDB")
		}},
		{"wrong character saved variables", gate, "Relax Raid TOC", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return raidEditTOC(e, "SavedVariablesPerCharacter: RelaxRaidCharDB", "SavedVariablesPerCharacter: OtherDB")
		}},
		{"version mismatch", gate, "Relax Raid TOC", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			m["raidVersion"] = "9.9.9"
			return e
		}},
		{"requires another addon", gate, "must not require", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return raidEditTOC(e, "## SavedVariables:", "## Dependencies: ForeverCoach\n## SavedVariables:")
		}},
		{"load file missing", gate, "load file", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			out := []addonZipEntry{}
			for _, x := range e {
				if x.name != addonRaidRoot+"/Log.lua" {
					out = append(out, x)
				}
			}
			return out
		}},
		{"second TOC", gate, "only RelaxRaid.toc", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{addonRaidRoot + "/RelaxRaid_Vanilla.toc", "## Title: Relax Raid\n", 0644})
		}},
		{"executable in the folder", gate, "outside the fixed guild allowlist", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{addonRaidRoot + "/tool.exe", "MZ", 0644})
		}},
		{"media in the folder", gate, "outside the fixed guild allowlist", func(e []addonZipEntry, m map[string]any) []addonZipEntry {
			return append(e, addonZipEntry{addonRaidRoot + "/icon.tga", "tga", 0644})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			game := guildGame(t)
			raidSavedVariables(t, game)
			before := addonTreeHashes(t, game)
			u := guildUpdaterForVersion(t, bundleZip(t, tc.version, tc.mutate), tc.version)
			_, err := u.apply(context.Background(), game, guildInfoForVersion(tc.version))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error containing %q", err, tc.want)
			}
			sameHashes(t, "game on refusal", addonTreeHashes(t, game), before)
		})
	}
}

// SYNTHETIC (no real RelaxRaid bundle exists before R9b): the app before R9a, simulated by the roots it knew,
// finds every path it does not know under Interface/AddOns/RelaxRaid/ and nowhere else, so it refuses the gate
// bundle for that folder alone. The real gate (published old tag, real bundle, check_old_app_refuses_bundle.sh
// re-pinned to the gate version) is R9b's.
func TestBundleRaidSyntheticPreviousAppRefusesOnlyTheNewFolder(t *testing.T) {
	gate := addonRaidMinimumVersion
	old := addonGuildRootsFor("1.7.2")
	coach := addonPublicFilesForVersion(gate)
	unknown := []string{}
	for _, e := range funModeEntries(gate) {
		if !addonGuildPath(e.name, false, coach, old) {
			unknown = append(unknown, e.name)
		}
	}
	if len(unknown) != len(raidFixtureEntries()) {
		t.Fatalf("paths the old app does not know: %v", unknown)
	}
	for _, name := range unknown {
		if !strings.HasPrefix(name, addonRaidRoot+"/") {
			t.Fatalf("the old app would also refuse %s, outside RelaxRaid", name)
		}
	}
}

// A gate release that also carries the old asset name: the new app takes the raid bundle only. A gate release
// with only the old name is not offered.
func TestRaidReleaseSelection(t *testing.T) {
	gate := addonRaidMinimumVersion
	old := "RelaxCoach-Bundle-" + gate + ".zip"
	both := func(r map[string]any) {
		r["assets"] = append(r["assets"].([]map[string]any), map[string]any{"name": old, "browser_download_url": addonReleasePrefix + "download/v" + gate + "/" + old})
	}
	onlyOld := func(r map[string]any) {
		r["assets"].([]map[string]any)[0] = map[string]any{"name": old, "browser_download_url": addonReleasePrefix + "download/v" + gate + "/" + old}
	}
	for _, tc := range []struct {
		name      string
		mutate    func(map[string]any)
		available bool
	}{{"both names", both, true}, {"old name only", onlyOld, false}} {
		t.Run(tc.name, func(t *testing.T) {
			u := addonTestUpdater(t, func(w http.ResponseWriter, r *http.Request) {
				body := guildTestRelease(gate)
				tc.mutate(body)
				json.NewEncoder(w).Encode(body)
			})
			got, err := u.check(context.Background(), "1.7.2")
			if err != nil {
				t.Fatal(err)
			}
			if got.Available != tc.available {
				t.Fatalf("availability: %+v", got)
			}
			if tc.available && (got.AssetName != "RelaxCoach-RaidBundle-"+gate+".zip" || !strings.HasSuffix(got.AssetURL, "/RelaxCoach-RaidBundle-"+gate+".zip")) {
				t.Fatalf("took %s (%s), want the raid bundle", got.AssetName, got.AssetURL)
			}
			if !tc.available && !strings.Contains(got.Notes, "RelaxCoach-RaidBundle-"+gate+".zip") {
				t.Fatalf("reason does not name the raid bundle: %+v", got)
			}
		})
	}
}

// A RelaxRaid folder below the gate (a manual copy, or left after a restore) is listed as present, does not make
// the install partial, and the first gate bundle backs it up byte-for-byte and replaces it.
func TestBundleRaidCopyBelowTheGateThenFirstGateBundle(t *testing.T) {
	game := guildGame(t)
	u := guildUpdaterForVersion(t, bundleZip(t, "1.7.2", nil), "1.7.2")
	if _, err := u.apply(context.Background(), game, guildInfoForVersion("1.7.2")); err != nil {
		t.Fatal(err)
	}
	before := raidManualCopy(t, game)
	s := InstalledBundleState(game)
	if s.State != "installed" || len(s.MissingFolders) != 0 || !strings.Contains(strings.Join(s.PresentFolders, ","), "RelaxRaid") {
		t.Fatalf("state below the gate: %+v", s)
	}
	gate := addonRaidMinimumVersion
	u = guildUpdaterForVersion(t, bundleZip(t, gate, nil), gate)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(gate))
	if err != nil {
		t.Fatal(err)
	}
	raidInstalledMatches(t, game)
	sameHashes(t, "RelaxRaid backup", addonTreeHashes(t, filepath.Join(result.BackupPath, filepath.FromSlash(addonRaidRoot))), before)
	if s := InstalledBundleState(game); s.State != "installed" {
		t.Fatalf("state after the gate bundle: %+v", s)
	}
}

func TestBundleStateWithRelaxRaid(t *testing.T) {
	// a 1.7.x install without RelaxRaid is complete; with the gate version installed, RelaxRaid is expected
	for _, tc := range []struct {
		version, state string
		raid           bool
		missing        string
	}{
		{"1.7.2", "installed", false, ""},
		{"1.7.2", "installed", true, ""},
		{addonRaidMinimumVersion, "partial", false, "RelaxRaid"},
		{addonRaidMinimumVersion, "installed", true, ""},
	} {
		t.Run(tc.version+map[bool]string{true: "+raid", false: ""}[tc.raid], func(t *testing.T) {
			game := guildGame(t)
			u := guildUpdaterForVersion(t, bundleZip(t, tc.version, nil), tc.version)
			if _, err := u.apply(context.Background(), game, guildInfoForVersion(tc.version)); err != nil {
				t.Fatal(err)
			}
			target := filepath.Join(game, filepath.FromSlash(addonRaidRoot))
			if tc.raid && !addonRaidRelease(tc.version) {
				raidManualCopy(t, game)
			}
			if !tc.raid {
				os.RemoveAll(target)
			}
			s := InstalledBundleState(game)
			if s.State != tc.state || strings.Join(s.MissingFolders, ",") != tc.missing {
				t.Fatalf("state %+v, want %s missing %q", s, tc.state, tc.missing)
			}
			listed := false
			for _, p := range s.PresentFolders {
				listed = listed || p == "RelaxRaid"
			}
			if listed != tc.raid {
				t.Fatalf("RelaxRaid listed=%v, want %v: %+v", listed, tc.raid, s)
			}
		})
	}
}

// The app killed right after RelaxRaid was moved aside: a later run's lock recovery puts it back.
func TestBundleRaidStaleLockRecoveryPutsRelaxRaidBack(t *testing.T) {
	version := addonRaidMinimumVersion
	game := guildGame(t)
	before := raidManualCopy(t, game)
	lock, transaction := crashedBundleTransaction(t, game, 20*time.Minute, false)
	prev := filepath.Join(transaction, "previous", filepath.FromSlash(addonRaidRoot))
	if err := os.MkdirAll(filepath.Dir(prev), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(game, filepath.FromSlash(addonRaidRoot)), prev); err != nil {
		t.Fatal(err)
	}
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	if !u.addonRecoverStaleLock(game) {
		t.Fatal("the stale lock was not seen")
	}
	sameHashes(t, "RelaxRaid after recovery", addonTreeHashes(t, filepath.Join(game, filepath.FromSlash(addonRaidRoot))), before)
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatalf("the stale lock was kept: %v", err)
	}
}

// A fresh client (no Coach yet) is not told that RelaxRaid is missing: before the install it lists the folders
// every bundle has had, so the first-install card does not change when the app learns the new folder.
func TestBundleStateFreshClientDoesNotExpectRelaxRaid(t *testing.T) {
	game := freshClient(t)
	addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Relax Journey\n")
	s := InstalledBundleState(game)
	if !s.Install || s.State != "partial" {
		t.Fatalf("state %+v", s)
	}
	if got := strings.Join(s.MissingFolders, ","); got != "ForeverCoach,Journey-Content,RelaxQuestPlanner" {
		t.Fatalf("missing %q", got)
	}
}
