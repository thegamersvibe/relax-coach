package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Repros from the R9a review (the reviewer and the second opinion): the app killed mid-swap after Coach lands,
// then a Repair whose download fails; a refusal that left an empty RelaxCoachBackups; a raidVersion present
// but empty or null below the gate.

type markerEntry struct {
	Mode os.FileMode
	Data string
}

// A snapshot with bytes, modes, directories and links (addonTreeHashes ignores directories and modes).
func markerSnapshot(t *testing.T, root string) map[string]markerEntry {
	t.Helper()
	out := map[string]markerEntry{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		e := markerEntry{Mode: info.Mode()}
		if info.Mode()&os.ModeSymlink != 0 {
			e.Data, err = os.Readlink(p)
		} else if info.Mode().IsRegular() {
			var b []byte
			b, err = os.ReadFile(p)
			e.Data = string(b)
		}
		out[filepath.ToSlash(rel)] = e
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func markerSame(t *testing.T, root string, want map[string]markerEntry) {
	t.Helper()
	got := markerSnapshot(t, root)
	diff := []string{}
	for k, v := range want {
		if n, ok := got[k]; !ok {
			diff = append(diff, "removed "+k)
		} else if n != v {
			diff = append(diff, "changed "+k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			diff = append(diff, "added "+k)
		}
	}
	sort.Strings(diff)
	if len(diff) != 0 {
		t.Fatalf("tree changed: %v", diff)
	}
}

// A 1.7.1 install with every managed folder present (and optionally an earlier RelaxRaid copy).
func markerGame(t *testing.T, raid bool) string {
	t.Helper()
	game := guildGame(t)
	plannerManualPatch(t, game)
	addonPut(t, game, addonGuildRoots[1]+"/Journey.toc", "## Title: Relax Journey\n## Version: 0.0.1\nprivate.lua\n")
	addonPut(t, game, addonGuildRoots[0]+"/ForeverCoach.toc", "## Title: Relax Coach\n## Version: 1.7.1\nCore.lua\n")
	if raid {
		raidManualCopy(t, game)
	}
	raidSavedVariables(t, game)
	return game
}

// The child process: install, and exit hard right after the stop-th rename.
func TestRepairMarkerCrashChild(t *testing.T) {
	game := os.Getenv("REPAIR_MARKER_CRASH_GAME")
	if game == "" {
		t.Skip("subprocess only")
	}
	version := os.Getenv("REPAIR_MARKER_CRASH_VERSION")
	stop, err := strconv.Atoi(os.Getenv("REPAIR_MARKER_CRASH_STOP"))
	if err != nil {
		t.Fatal(err)
	}
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	n := 0
	u.rename = func(from, to string) error {
		if err := os.Rename(from, to); err != nil {
			return err
		}
		if n++; n == stop {
			os.Exit(77)
		}
		return nil
	}
	_, err = u.apply(context.Background(), game, guildInfoForVersion(version))
	t.Fatalf("crash point not reached: count=%d err=%v", n, err)
}

func markerCrash(t *testing.T, game, version string, stop int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRepairMarkerCrashChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), "REPAIR_MARKER_CRASH_GAME="+game, "REPAIR_MARKER_CRASH_VERSION="+version, "REPAIR_MARKER_CRASH_STOP="+strconv.Itoa(stop))
	out, err := cmd.CombinedOutput()
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 77 {
		t.Fatalf("crash child: %v\n%s", err, out)
	}
	lock := filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")
	old := time.Now().Add(-20 * time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
}

func markerFailingServer(t *testing.T) *addonUpdater {
	return addonTestUpdater(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "temporary network outage", http.StatusServiceUnavailable)
	})
}

func TestRepairMarkerSurvivesAFailedRepairUntilAVerifiedInstall(t *testing.T) {
	version := addonRaidMinimumVersion
	for _, manual := range []bool{false, true} {
		t.Run(fmt.Sprintf("earlier-raid-copy=%v", manual), func(t *testing.T) {
			game := markerGame(t, manual)
			wtf := markerSnapshot(t, filepath.Join(game, "WTF"))
			markerCrash(t, game, version, 2) // Coach is new; Journey, the planner and RelaxRaid are still old
			if InstalledCoachVersion(game) != version || !addonRepairNeeded(game) {
				t.Fatalf("after the crash: Coach %s, repair offered %v", InstalledCoachVersion(game), addonRepairNeeded(game))
			}
			offer := guildInfoForVersion(version)
			offer.Repair = true
			for i := 0; i < 2; i++ { // the first failed Repair clears the lock; a second one must still be allowed
				if _, err := markerFailingServer(t).apply(context.Background(), game, offer); err == nil || !strings.Contains(err.Error(), "503") {
					t.Fatalf("Repair %d: want the failed download, got %v", i+1, err)
				}
				if !addonRepairNeeded(game) {
					t.Fatalf("Repair %d failed and the half-install is forgotten (state %+v)", i+1, InstalledBundleState(game))
				}
			}
			if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")); !os.IsNotExist(err) {
				t.Fatalf("lock left: %v", err)
			}
			// a Repair that downloads and verifies everything clears the marker
			u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
			if _, err := u.apply(context.Background(), game, offer); err != nil {
				t.Fatal(err)
			}
			raidInstalledMatches(t, game)
			plannerInstalledMatches(t, game)
			if addonRepairNeeded(game) || InstalledBundleState(game).State != "installed" {
				t.Fatalf("after a verified Repair: repair %v, state %+v", addonRepairNeeded(game), InstalledBundleState(game))
			}
			if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", addonRepairMarkerName)); !os.IsNotExist(err) {
				t.Fatalf("marker left after a verified install: %v", err)
			}
			if _, err := u.apply(context.Background(), game, offer); err == nil || !strings.Contains(err.Error(), "nothing to repair") {
				t.Fatalf("a second Repair after completion: %v", err)
			}
			markerSame(t, filepath.Join(game, "WTF"), wtf)
		})
	}
}

// A crash before Coach lands: the next normal update completes everything and clears the marker.
func TestRepairMarkerClearedByTheNextUpdate(t *testing.T) {
	version := addonRaidMinimumVersion
	game := markerGame(t, false)
	markerCrash(t, game, version, 1)
	u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
	if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err != nil {
		t.Fatal(err)
	}
	if addonRepairNeeded(game) {
		t.Fatalf("marker kept after a verified update: %+v", InstalledBundleState(game))
	}
}

// If the marker cannot be written, recovery keeps the lock (the only other sign of the interruption).
func TestRepairMarkerUnwritableKeepsTheLock(t *testing.T) {
	game := markerGame(t, false)
	lock, _ := crashedBundleTransaction(t, game, 20*time.Minute, false)
	if err := os.Mkdir(filepath.Join(game, "RelaxCoachBackups", addonRepairMarkerName), 0700); err != nil {
		t.Fatal(err) // a directory in the marker's place: the write fails
	}
	if !newAddonUpdater().addonRecoverStaleLock(game) {
		t.Fatal("stale lock not seen")
	}
	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("lock removed without a marker: %v", err)
	}
	if !addonRepairNeeded(game) {
		t.Fatal("repair not offered")
	}
}

// A refused bundle leaves the game folder exactly as it was, including no new RelaxCoachBackups folder; an
// existing backup folder is kept.
func TestRefusalLeavesNoBackupFolderBehind(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("backups-existed=%v", existing), func(t *testing.T) {
			game := guildGame(t)
			raidSavedVariables(t, game)
			if existing {
				if err := os.Mkdir(filepath.Join(game, "RelaxCoachBackups"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			before := markerSnapshot(t, game)
			gate := addonRaidMinimumVersion
			u := guildUpdaterForVersion(t, bundleZip(t, gate, func(e []addonZipEntry, _ map[string]any) []addonZipEntry { return raidWithout(e) }), gate)
			if _, err := u.apply(context.Background(), game, guildInfoForVersion(gate)); err == nil {
				t.Fatal("accepted a gate bundle without RelaxRaid")
			}
			markerSame(t, game, before)
		})
	}
}

// Below the gate the manifest must not name raidVersion at all: an empty or null value is refused too. At the
// gate, empty, null or a non-string is refused.
func TestRaidVersionPresenceInTheManifest(t *testing.T) {
	gate := addonRaidMinimumVersion
	for _, tc := range []struct {
		version string
		value   any
	}{{"1.7.1", ""}, {"1.7.1", nil}, {gate, ""}, {gate, nil}, {gate, 1}} {
		t.Run(fmt.Sprintf("%s/%#v", tc.version, tc.value), func(t *testing.T) {
			game := guildGame(t)
			raidManualCopy(t, game)
			before := markerSnapshot(t, game)
			payload := bundleZip(t, tc.version, func(e []addonZipEntry, m map[string]any) []addonZipEntry { m["raidVersion"] = tc.value; return e })
			_, err := guildUpdaterForVersion(t, payload, tc.version).apply(context.Background(), game, guildInfoForVersion(tc.version))
			if err == nil || !strings.Contains(err.Error(), "manifest") {
				t.Fatalf("raidVersion=%#v in %s: %v", tc.value, tc.version, err)
			}
			markerSame(t, game, before)
		})
	}
}
