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
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Copied into an archive of the published app, NOT compiled with today's
// installer. The expected messages are reviewed literals, not derived from
// whichever error the installer happens to return.
const oldAppUnknownFile = "archive path is outside the fixed guild allowlist: Interface/AddOns/ForeverCoach/Media/Reactions/flight.mp3"
const oldAppManualURL = "https://github.com/thegamersvibe/relax-coach/releases/latest"
const oldAppFallbackError = oldAppUnknownFile + ". This release comes with app 1.7.0; if updating the app here does not work, download RelaxCoach.exe from " + oldAppManualURL
const oldAppFirstError = "Update the app first: Relax Coach 1.7.0 needs app 1.7.0. Click Update App (or Update everything); the addons follow. If the app cannot update itself, download RelaxCoach.exe from " + oldAppManualURL

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
				err, wantReason = errors.New(strings.ReplaceAll(oldAppFallbackError, "flight.mp3", "different.mp3")), "message"
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
				addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Version: 1.7.0\n")
				wantReason = "version"
			case "result":
				result.Version, wantReason = "1.7.0", "result"
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

// Condition 1 proves the FIRST refusal; this proves it is the ONLY one. The
// real bundle with just the new media file and its manifest row removed must
// install cleanly through the same published installer, so nothing else in
// 1.7.0 (a stray file, a bad hash, a changed root) hides behind flight.mp3.
func TestOldAppRefusalStrippedBundleInstalls(t *testing.T) {
	payload, err := os.ReadFile(os.Getenv("RC_BUNDLE"))
	if err != nil {
		t.Fatal(err)
	}
	const newFile = "Interface/AddOns/ForeverCoach/Media/Reactions/flight.mp3"
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	w := zip.NewWriter(&out)
	removedFile, removedRow, kept := false, false, 0
	for _, f := range reader.File {
		if f.Name == newFile {
			removedFile = true
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
			var files []map[string]any
			if err := json.Unmarshal(m["files"], &files); err != nil {
				t.Fatal(err)
			}
			var keep []map[string]any
			for _, row := range files {
				if row["path"] == newFile {
					removedRow = true
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
	if !removedFile || !removedRow {
		t.Fatalf("real bundle must carry %s and its manifest row (file=%t row=%t)", newFile, removedFile, removedRow)
	}
	game := guildGame(t)
	installed := strings.TrimPrefix(os.Getenv("RC_OLD"), "v")
	addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Title: Relax Coach\n## Version: "+installed+"\n## SavedVariables: ForeverCoachDB\nCore.lua\n")
	stripped := out.Bytes()
	result, err := guildUpdaterForVersion(t, stripped, "1.7.0").apply(context.Background(), game, guildInfoForVersion("1.7.0"))
	if err != nil || result.Version != "1.7.0" {
		t.Fatalf("old app refused the 1.7.0 bundle even without %s: %+v %v", newFile, result, err)
	}
	if got := InstalledCoachVersion(game); got != "1.7.0" {
		t.Fatalf("installed version after stripped install = %q", got)
	}
	t.Logf("old app installed the stripped 1.7.0 bundle (%d archive entries, %d files updated); %s is the only refusal", kept, result.UpdatedFiles, newFile)
}

func TestOldAppRefusesRealBundle(t *testing.T) {
	payload, err := os.ReadFile(os.Getenv("RC_BUNDLE"))
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("RC_VERSION") != "1.7.0" {
		t.Fatal("refusal probe is pinned to 1.7.0")
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	var unknown []string
	for _, f := range reader.File {
		if !addonGuildPath(strings.TrimSuffix(f.Name, "/"), f.Mode().IsDir(), addonPublicFilesForVersion("1.7.0"), addonGuildRootsFor("1.7.0")) {
			unknown = append(unknown, f.Name)
		}
	}
	if !reflect.DeepEqual(unknown, []string{"Interface/AddOns/ForeverCoach/Media/Reactions/flight.mp3"}) {
		t.Fatalf("unexpected old-app unknown path set: %q", unknown)
	}
	t.Logf("only unknown path: %s", unknown[0])
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
					u := guildUpdaterForVersion(t, payload, "1.7.0")
					release := guildInfoForVersion("1.7.0")
					release.HasApp = true
					calls := 0
					var installerErr error
					b := newNativeBridge(a, false, nativeServices{
						selfUpdate: func() bool { return true },
						appLast: func() (string, string, error) {
							if route == "installer-fallback" {
								return "old-transaction", "failed", nil
							}
							return "", "", nil
						},
						check: func(context.Context, string) (AddonReleaseInfo, error) { return release, nil },
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
					fallback := route == "installer-fallback"
					if offer.Available != fallback || offer.NeedsApp == fallback || offer.Install || offer.Repair {
						t.Fatalf("unexpected offer: %+v", offer)
					}
					value, err = b.Call("applyAddon", nativeArgs("1.7.0"))
					after := oldAppDiskSnapshot(t, game)
					versionAfter := InstalledCoachVersion(game)
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
					t.Logf("exact UI message: %s", "Update failed: "+err.Error()+" Check again before retrying.")
					if fallback {
						t.Log("NOTE: fallback is an allowlist refusal plus manual app link, not the normal 'Update the app first' message")
					}
				})
			}
		})
	}
}
