package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Copied into an archive of the published app, NOT compiled with today's
// installer. The expected messages are reviewed literals, not derived from
// whichever error the installer happens to return.
const oldAppVersion = "1.8.0"
const oldAppUnknownFile = "archive path is outside the fixed guild allowlist: Interface/AddOns/RelaxRaid/AvoidableSpells.lua"

// Reviewed against the real addon/RelaxRaid tree. Do not derive the expected
// set from the archive: that could silently approve a second unknown root.
var oldAppRaidFiles = []string{
	"Interface/AddOns/RelaxRaid/AvoidableSpells.lua",
	"Interface/AddOns/RelaxRaid/Core.lua",
	"Interface/AddOns/RelaxRaid/DeathRecap.lua",
	"Interface/AddOns/RelaxRaid/DeathRecapUI.lua",
	"Interface/AddOns/RelaxRaid/Log.lua",
	"Interface/AddOns/RelaxRaid/Prep.lua",
	"Interface/AddOns/RelaxRaid/RelaxRaid.toc",
	"Interface/AddOns/RelaxRaid/Store.lua",
	"Interface/AddOns/RelaxRaid/Wire.lua",
}

const oldAppManualURL = "https://github.com/thegamersvibe/relax-coach/releases/latest"
const oldAppFallbackError = oldAppUnknownFile + ". This release comes with app 1.8.0; if updating the app here does not work, download RelaxCoach.exe from " + oldAppManualURL

// v1.7.1 addon_update.go / native_bridge.go: a renamed asset is not Available,
// so Check says app-only (NOT NeedsApp), and a forced Apply is not ready.
const oldAppOnlyNote = "No compatible new addon update is available. This is an app-only release for this updater; no compatible addon bundle (RelaxCoach-Bundle-1.8.0.zip) is included. Use Update App separately."
const oldAppFirstError = "this update is no longer ready: click Check, then try again"

// Include directories (even empty staging/backup directories), symlinks and
// every regular file, not just addon hashes. Directory mtimes change when a
// temporary transaction is created and removed; they are not file changes.
// Regular-file mtimes, sizes, modes and bytes must all remain unchanged.
type oldAppDiskEntry struct {
	Mode   string
	Size   int64  `json:",omitempty"`
	Mtime  int64  `json:",omitempty"`
	SHA256 string `json:",omitempty"`
	Link   string `json:",omitempty"`
}

func oldAppDiskSnapshot(t *testing.T, root string) map[string]oldAppDiskEntry {
	t.Helper()
	out := map[string]oldAppDiskEntry{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entry := oldAppDiskEntry{Mode: info.Mode().String()}
		switch {
		case info.Mode().IsRegular():
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entry.Size, entry.Mtime = info.Size(), info.ModTime().UnixNano()
			entry.SHA256 = fmt.Sprintf("%x", sha256.Sum256(data))
		case info.Mode()&os.ModeSymlink != 0:
			entry.Link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		case !info.IsDir():
			return fmt.Errorf("unexpected special file: %s", rel)
		}
		out[filepath.ToSlash(rel)] = entry
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func oldAppRefusalProblem(result AddonUpdateResult, err error, want string, before, after map[string]oldAppDiskEntry, versionBefore, versionAfter string) error {
	if err == nil {
		return errors.New("old app accepted a bundle that must be refused")
	}
	if err.Error() != want {
		return fmt.Errorf("refusal message differs: got %q, want %q", err.Error(), want)
	}
	if result != (AddonUpdateResult{}) {
		return fmt.Errorf("refusal returned an installation result: %+v", result)
	}
	if versionBefore == "0.0.0" || versionBefore != versionAfter {
		return fmt.Errorf("installed version changed: %s -> %s", versionBefore, versionAfter)
	}
	if !reflect.DeepEqual(before, after) {
		// A first use of the old updater creates the backup root before
		// extracting. Permit that ONE empty directory, never a file, link,
		// child, removed backup or change to an already installed file.
		_, existed := before["RelaxCoachBackups"]
		entry, added := after["RelaxCoachBackups"]
		if existed || !added || !strings.HasPrefix(entry.Mode, "d") || len(after) != len(before)+1 {
			return errors.New("disk changed: file, directory, backup, staging or lock snapshot differs")
		}
		for name, was := range before {
			if got, present := after[name]; !present || got != was {
				return fmt.Errorf("disk changed: %s", name)
			}
		}
	}
	return nil
}

func TestOldAppRefusalAssertions(t *testing.T) {
	for _, kind := range []string{"clean", "accepted", "message", "message-prefix", "message-suffix", "message-other-path", "empty-backup", "backup-file", "backup-link", "version", "result", "file", "deleted", "mode", "stage", "backup", "lock"} {
		t.Run(kind, func(t *testing.T) {
			game := guildGame(t)
			before := oldAppDiskSnapshot(t, game)
			version := InstalledCoachVersion(game)
			result, err := AddonUpdateResult{}, errors.New(oldAppFallbackError)
			wantReason := ""
			switch kind {
			case "accepted":
				err, wantReason = nil, "accepted"
			case "message":
				err, wantReason = errors.New("some other refusal"), "message"
			case "message-prefix":
				err, wantReason = errors.New("prefix: "+oldAppFallbackError), "message"
			case "message-suffix":
				err, wantReason = errors.New(oldAppFallbackError+"; second unknown path"), "message"
			case "message-other-path":
				err, wantReason = errors.New(strings.ReplaceAll(oldAppFallbackError, "RelaxRaid/AvoidableSpells.lua", "Other/AvoidableSpells.lua")), "message"
			case "empty-backup":
				if err := os.Mkdir(filepath.Join(game, "RelaxCoachBackups"), 0700); err != nil {
					t.Fatal(err)
				}
			case "backup-file":
				addonPut(t, game, "RelaxCoachBackups", "not a directory")
				wantReason = "disk"
			case "backup-link":
				if err := os.Symlink(t.TempDir(), filepath.Join(game, "RelaxCoachBackups")); err != nil {
					t.Fatal(err)
				}
				wantReason = "disk"
			case "version":
				addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Version: 1.8.0\n")
				wantReason = "version"
			case "result":
				result.Version, wantReason = "1.8.0", "result"
			case "file":
				addonPut(t, game, "config.json", "changed")
				wantReason = "disk"
			case "deleted":
				if err := os.Remove(filepath.Join(game, "config.json")); err != nil {
					t.Fatal(err)
				}
				wantReason = "disk"
			case "mode":
				if err := os.Chmod(filepath.Join(game, "config.json"), 0600); err != nil {
					t.Fatal(err)
				}
				wantReason = "disk"
			case "stage":
				if err := os.MkdirAll(filepath.Join(game, "RelaxCoachBackups", "unfinished", "staged"), 0700); err != nil {
					t.Fatal(err)
				}
				wantReason = "disk"
			case "backup":
				addonPut(t, game, "RelaxCoachBackups/unfinished/previous/partial.lua", "partial")
				wantReason = "disk"
			case "lock":
				addonPut(t, game, "RelaxCoachBackups/.addon-update.lock", "unfinished")
				wantReason = "disk"
			}
			problem := oldAppRefusalProblem(result, err, oldAppFallbackError, before, oldAppDiskSnapshot(t, game), version, InstalledCoachVersion(game))
			if kind == "clean" || kind == "empty-backup" {
				if problem != nil {
					t.Fatal(problem)
				}
			} else if problem == nil || !strings.Contains(problem.Error(), wantReason) {
				t.Fatalf("probe did not reject %s with %q: %v", kind, wantReason, problem)
			}
		})
	}
}

func TestOldAppRefusalReplacementExecutable(t *testing.T) {
	data, err := os.ReadFile(os.Getenv("RC_EXE"))
	if err != nil {
		t.Fatal(err)
	}
	// This is the published app's actual downloaded-exe validator, not
	// today's parser and not just a check for the two-byte MZ signature.
	if err := validateAppPE(data); err != nil {
		t.Fatalf("old app rejected replacement executable: %v", err)
	}
	t.Logf("old app accepted replacement executable sha256=%x", sha256.Sum256(data))
}

// Remove exactly the reviewed raid files, their manifest rows and raidVersion
// (unknown to the old strict JSON decoder). Everything else, including every
// retained hash, must install through the published installer unchanged.
func TestOldAppRefusalStrippedBundleInstalls(t *testing.T) {
	payload, err := os.ReadFile(os.Getenv("RC_BUNDLE"))
	if err != nil {
		t.Fatal(err)
	}
	approved := map[string]bool{}
	for _, name := range oldAppRaidFiles {
		approved[name] = true
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	var removedFiles, removedRows []string
	kept := 0
	for _, f := range reader.File {
		if approved[f.Name] {
			removedFiles = append(removedFiles, f.Name)
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == "manifest.json" {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(data, &m); err != nil {
				t.Fatal(err)
			}
			if len(m["raidVersion"]) == 0 {
				t.Fatal("real raid manifest must declare raidVersion")
			}
			delete(m, "raidVersion")
			var files []map[string]any
			if err := json.Unmarshal(m["files"], &files); err != nil {
				t.Fatal(err)
			}
			var keep []map[string]any
			for _, row := range files {
				name, _ := row["path"].(string)
				if approved[name] {
					removedRows = append(removedRows, name)
					continue
				}
				keep = append(keep, row)
			}
			if m["files"], err = json.Marshal(keep); err != nil {
				t.Fatal(err)
			}
			if data, err = json.MarshalIndent(m, "", "  "); err != nil {
				t.Fatal(err)
			}
		}
		h := f.FileHeader
		h.CompressedSize64, h.UncompressedSize64, h.CRC32 = 0, 0, 0
		dst, err := w.CreateHeader(&h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(data); err != nil {
			t.Fatal(err)
		}
		kept++
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	sort.Strings(removedFiles)
	sort.Strings(removedRows)
	if !reflect.DeepEqual(removedFiles, oldAppRaidFiles) || !reflect.DeepEqual(removedRows, oldAppRaidFiles) {
		t.Fatalf("real bundle must carry exactly the reviewed raid files and rows: files=%q rows=%q", removedFiles, removedRows)
	}
	game := guildGame(t)
	installed := strings.TrimPrefix(os.Getenv("RC_OLD"), "v")
	addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Title: Relax Coach\n## Version: "+installed+"\n## SavedVariables: ForeverCoachDB\nCore.lua\n")
	stripped := out.Bytes()
	result, err := guildUpdaterForVersion(t, stripped, "1.8.0").apply(context.Background(), game, guildInfoForVersion("1.8.0"))
	if err != nil || result.Version != "1.8.0" {
		t.Fatalf("old app refused the 1.8.0 bundle even without RelaxRaid: %+v %v", result, err)
	}
	if got := InstalledCoachVersion(game); got != "1.8.0" {
		t.Fatalf("installed version after stripped install = %q", got)
	}
	t.Logf("old app installed the stripped 1.8.0 bundle (%d archive entries, %d files updated); only RelaxRaid was removed", kept, result.UpdatedFiles)
}

func TestOldAppRefusesRealBundle(t *testing.T) {
	payload, err := os.ReadFile(os.Getenv("RC_BUNDLE"))
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("RC_VERSION") != "1.8.0" {
		t.Fatal("refusal probe is pinned to 1.8.0")
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	var unknown []string
	for _, f := range reader.File {
		if !addonGuildPath(strings.TrimSuffix(f.Name, "/"), f.Mode().IsDir(), addonPublicFilesForVersion("1.8.0"), addonGuildRootsFor("1.8.0")) {
			unknown = append(unknown, f.Name)
		}
	}
	sort.Strings(unknown)
	if !reflect.DeepEqual(unknown, oldAppRaidFiles) {
		t.Fatalf("unexpected old-app unknown path set: %q", unknown)
	}
	t.Logf("complete unknown path set: %q", unknown)
	// Freeze the actual old shell's error presentation as well as the bridge
	// error. If the old UI changes, do not silently claim this is its message.
	shell, err := os.ReadFile("native_shell.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(shell), "const verb = install => install ? 'Install' : 'Update';") ||
		!strings.Contains(string(shell), "const failText = (install,err) => verb(install)+' failed: '+err.message+' Check again before retrying.';") ||
		!strings.Contains(string(shell), "const install = offer.Install === true, label = bundleVerb();") ||
		!strings.Contains(string(shell), "const result = await rc.applyAddon(offer.Version);") ||
		!strings.Contains(string(shell), ": (offer.Notes || 'No compatible new addon update is available.')+' No files changed.'") ||
		!strings.Contains(string(shell), "} catch(err) { bundleStatus(label === 'Repair' ? 'Repair failed: '+err.message+' Check again before retrying.' : failText(install, err)); }") {
		t.Fatal("old shell error presentation changed; review the exact user-facing message")
	}
	for _, priorBackup := range []bool{true, false} {
		t.Run(fmt.Sprintf("prior-backup=%t", priorBackup), func(t *testing.T) {
			for _, route := range []string{"app-first", "installer-fallback"} {
				t.Run(route, func(t *testing.T) {
					game := guildGame(t)
					installed := strings.TrimPrefix(os.Getenv("RC_OLD"), "v")
					addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Title: Relax Coach\n## Version: "+installed+"\n## SavedVariables: ForeverCoachDB\nCore.lua\n")
					// Exercise both app-installed and hand-copied old bundles. A first
					// updater attempt may create ONLY the empty backup root on refusal.
					if priorBackup {
						addonPut(t, game, "RelaxCoachBackups/previous-install/keep.txt", "retained previous backup")
					}
					a := &App{}
					a.setGame(game)
					// The real release publishes ONLY the new name. Fallback below is a
					// deliberately forced old-name offer to exercise the old installer;
					// it is not reachable through normal Check on that release.
					fallback := route == "installer-fallback"
					u := guildUpdaterForVersion(t, payload, oldAppVersion)
					release := guildInfoForVersion("1.8.0")
					release.HasApp = true
					calls, checks, downloads := 0, 0, 0
					check := func(context.Context, string) (AddonReleaseInfo, error) { return release, nil }
					if !fallback {
						checker := addonTestUpdater(t, func(w http.ResponseWriter, r *http.Request) {
							if r.URL.Path != "/repos/thegamersvibe/relax-coach/releases/latest" {
								downloads++
								t.Errorf("renamed release must not download: %s", r.URL.Path)
								http.NotFound(w, r)
								return
							}
							checks++
							prefix := addonReleasePrefix + "download/v1.8.0/"
							var assets []map[string]any
							for _, name := range []string{"RelaxCoach-RaidBundle-1.8.0.zip", "SHA256SUMS.txt", "RelaxCoach.exe"} {
								assets = append(assets, map[string]any{"name": name, "browser_download_url": prefix + name})
							}
							json.NewEncoder(w).Encode(map[string]any{"tag_name": "v1.8.0", "assets": assets})
						})
						check = checker.check
					}
					var installerErr error
					b := newNativeBridge(a, false, nativeServices{
						selfUpdate: func() bool { return true },
						appLast: func() (string, string, error) {
							if route == "installer-fallback" {
								return "old-transaction", "failed", nil
							}
							return "", "", nil
						},
						check: check,
						apply: func(ctx context.Context, game string, r AddonReleaseInfo) (AddonUpdateResult, error) {
							calls++
							result, err := u.apply(ctx, game, r)
							installerErr = err
							return result, err
						},
					})
					defer b.Close()
					before := oldAppDiskSnapshot(t, game)
					versionBefore := InstalledCoachVersion(game)
					value, err := b.Call("checkAddon", nil)
					if err != nil {
						t.Fatal(err)
					}
					offer := value.(nativeOffer)
					if offer.Available != fallback || offer.NeedsApp || offer.Install || offer.Repair {
						t.Fatalf("unexpected offer: %+v", offer)
					}
					if !fallback && (offer.Notes != oldAppOnlyNote || offer.Version != "" || checks != 1 || downloads != 0) {
						t.Fatalf("renamed asset did not produce the exact app-only offer without downloads: %+v checks=%d downloads=%d", offer, checks, downloads)
					}
					value, err = b.Call("applyAddon", nativeArgs(oldAppVersion))
					after := oldAppDiskSnapshot(t, game)
					versionAfter := InstalledCoachVersion(game)
					if !fallback && (!reflect.DeepEqual(before, after) || downloads != 0) {
						t.Fatal("app-only route changed disk or downloaded a payload")
					}
					want, wantCalls := oldAppFirstError, 0
					if fallback {
						want, wantCalls = oldAppFallbackError, 1
					}
					// Log both complete snapshots even on failure, for release evidence.
					for _, snapshot := range []struct {
						name  string
						value any
					}{
						{"before", before}, {"after", after},
					} {
						data, err := json.Marshal(snapshot.value)
						if err != nil {
							t.Fatal(err)
						}
						t.Logf("snapshot-%s: %s", snapshot.name, data)
					}
					t.Logf("installed version before=%s after=%s; installer calls=%d", versionBefore, versionAfter, calls)
					if !priorBackup && fallback {
						entries, readErr := os.ReadDir(filepath.Join(game, "RelaxCoachBackups"))
						if readErr != nil || len(entries) != 0 {
							t.Fatalf("first refusal must leave only an empty backup directory: %v %v", entries, readErr)
						}
						t.Log("allowed new directory: RelaxCoachBackups (asserted empty; no addon file changed)")
					}
					if problem := oldAppRefusalProblem(value.(AddonUpdateResult), err, want, before, after, versionBefore, versionAfter); problem != nil {
						t.Fatal(problem)
					}
					if calls != wantCalls {
						t.Fatalf("installer calls=%d want %d", calls, wantCalls)
					}
					if fallback && (installerErr == nil || installerErr.Error() != oldAppUnknownFile) {
						t.Fatalf("unexpected installer refusal: %v", installerErr)
					}
					if fallback {
						t.Logf("forced installer-fallback UI message: %s", "Update failed: "+err.Error()+" Check again before retrying.")
					} else {
						t.Logf("Check status prefix: %s No files changed.", offer.Notes)
						t.Log("normal route: app-only release, no addon download; use Update App first")
					}
				})
			}
		})
	}
}
