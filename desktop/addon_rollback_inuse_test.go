//go:build !windows

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
)

// Adapted from Astra's review probe (astra_rw_probe_test.go, 1.5.2 round 1).
// Synthetic fixtures; Requests are served
// from memory; these probes never open a network connection.
type rwTransport func(*http.Request) (*http.Response, error)

func (f rwTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func rwUpdater(payload []byte, version string, extra bool) *addonUpdater {
	u := newAddonUpdater()
	name := addonAssetName(version)
	if extra {
		name = "ForeverMaps-" + version + ".zip"
	}
	u.client.Transport = rwTransport(func(r *http.Request) (*http.Response, error) {
		var body []byte
		switch {
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
			body = []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(payload), name))
		case strings.HasSuffix(r.URL.Path, "/"+name):
			body = payload
		default:
			return nil, fmt.Errorf("unexpected in-memory request %s", r.URL)
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: r}, nil
	})
	u.running = func(context.Context) (bool, error) { return true, nil }
	return u
}
func rwExtraInfo(v string) ExtraAddonInfo {
	p := "https://github.com/buiden/ForeverMaps/releases/download/v" + v + "/"
	return ExtraAddonInfo{ID: "forevermaps", Version: v, Available: true, AssetURL: p + "ForeverMaps-" + v + ".zip", ChecksumURL: p + "SHA256SUMS.txt"}
}
func rwSmall(entries []addonZipEntry, _ map[string]any) []addonZipEntry {
	out := []addonZipEntry{}
	version := ""
	for _, e := range entries {
		if strings.HasSuffix(e.name, "/ForeverCoach.toc") {
			for _, line := range strings.Split(e.body, "\n") {
				if strings.HasPrefix(line, "## Version: ") {
					version = strings.TrimPrefix(line, "## Version: ")
				}
			}
		}
	}
	for _, e := range entries {
		if strings.Contains(e.name, "/lib/sub/") {
			continue
		}
		// Distinguish generations in all four roots without introducing new paths.
		switch e.name {
		case "Interface/AddOns/Journey/Main.xml":
			e.body = "<Ui><!-- synthetic generation " + version + " --></Ui>\n"
		case "Interface/Content/Journey-Content/guides.html":
			e.body += " synthetic generation " + version
		case "Interface/AddOns/RelaxQuestPlanner/Init.lua":
			e.body += "-- synthetic generation " + version + "\n"
		}
		out = append(out, e)
	}
	return out
}
func rwSeed(t *testing.T, game string, extra, fresh bool) {
	t.Helper()
	addonPut(t, game, "WTF/Config.wtf", "original settings")
	addonPut(t, game, "RelaxCoach.exe", "original app")
	addonPut(t, game, "RelaxCoachBackups/prior-backup/keep.txt", "original backup")
	if fresh {
		return
	}
	if extra {
		addonPut(t, game, "Interface/AddOns/ForeverMaps/ForeverMaps.toc", fmt.Sprintf(extraTestTOC, "0.1.3"))
		addonPut(t, game, "Interface/AddOns/ForeverMaps/Core.lua", "-- core 0.1.3")
		addonPut(t, game, "Interface/AddOns/ForeverMaps/Data/World.lua", "-- world")
	} else {
		for _, e := range rwSmall(funModeEntries("1.5.1"), nil) {
			if guildRuntime(e.name) {
				addonPut(t, game, e.name, e.body)
			}
		}
	}
}
func rwSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			out[filepath.ToSlash(rel)] = "DIR"
			return nil
		}
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		out[filepath.ToSlash(rel)] = fmt.Sprintf("%x", sha256.Sum256(b))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Model Windows' Errno.Is(ErrPermission) for 5 while running its classifier on
// Linux. No Windows filesystem operation is being claimed or emulated here.
func rwRefusal(n syscall.Errno, a, b string) error {
	var e error = n
	if os.Getenv("RW_WIN_ERRORS") == "1" && n == 5 {
		e = errors.Join(n, os.ErrPermission)
	}
	return &os.LinkError{Op: "rename", Old: a, New: b, Err: e}
}

// rwFinish runs the follow-up the error message asks for (close WoW, then
// Check and Repair or Update) with renames working again: it must complete in
// the same app session, without waiting for the 15-minute stale-lock rule.
func rwFinish(t *testing.T, u *addonUpdater, game string, extra bool) {
	t.Helper()
	u.rename = os.Rename
	if extra {
		if _, err := u.applyExtra(context.Background(), game, rwExtraInfo("0.1.4")); err != nil {
			t.Fatalf("Update after closing WoW: %v", err)
		}
		s, err := extraInstalled(game, extraAddons[1])
		if err != nil || !s.exists || s.version.text != "0.1.4" {
			t.Fatalf("after Update: %+v %v", s, err)
		}
		return
	}
	info := guildInfoForVersion("1.5.2")
	if InstalledCoachVersion(game) == "1.5.2" {
		if !addonRepairNeeded(game) {
			t.Fatalf("Check would not offer Repair: %+v", InstalledBundleState(game))
		}
		info.Repair = true
	}
	if _, err := u.apply(context.Background(), game, info); err != nil {
		t.Fatalf("Repair/Update after closing WoW: %v", err)
	}
	if s := InstalledBundleState(game); s.State != "installed" || InstalledCoachVersion(game) != "1.5.2" {
		t.Fatalf("after Repair/Update: %+v", s)
	}
	if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")); !os.IsNotExist(err) {
		t.Fatal("lock left after Repair/Update")
	}
}

// rwCloseAdvice: a refused rollback leads with the close-WoW step and a short
// backup location inside the game folder, without raw absolute paths.
func rwCloseAdvice(t *testing.T, err error, game string) {
	t.Helper()
	msg := err.Error()
	if !strings.HasPrefix(msg, "Close WoW, then click") || !strings.Contains(msg, "RelaxCoachBackups") || strings.Contains(msg, game) {
		t.Errorf("rollback refused: want close-WoW advice with a short backup location, got %q", msg)
	}
}

// S1 (مانع/Astra): every rename refused (in use), and every rollback rename
// after it refused once or for good.
func TestRWRenameMatrix(t *testing.T) {
	errs := []syscall.Errno{syscall.EBUSY, syscall.ETXTBSY}
	payload := bundleZip(t, "1.5.2", rwSmall)
	extraPayload := extraGoodZip(t, "0.1.4")
	for _, kind := range []string{"bundle-update", "bundle-fresh", "extra-update", "extra-fresh"} {
		extra, fresh := strings.HasPrefix(kind, "extra"), strings.HasSuffix(kind, "fresh")
		forward := 8
		if fresh {
			forward = 4
		}
		if extra {
			forward = 2
			if fresh {
				forward = 1
			}
		}
		for _, eno := range errs {
			for k := 1; k <= forward; k++ {
				rolls := []int{0}
				for r := 1; r < k; r++ {
					rolls = append(rolls, r)
				}
				if k > 1 {
					rolls = append(rolls, -1)
				}
				for _, r := range rolls {
					t.Run(fmt.Sprintf("%s/errno%d/k%d/r%d", kind, eno, k, r), func(t *testing.T) {
						game := t.TempDir()
						rwSeed(t, game, extra, fresh)
						before := rwSnapshot(t, game)
						roots := addonGuildRootsFor("1.5.2")
						lockName := ".addon-update.lock"
						if extra {
							roots = []string{"Interface/AddOns/ForeverMaps"}
							lockName = ".extra-addon-update.lock"
						}
						old := map[string]map[string]string{}
						if !fresh {
							for _, root := range roots {
								old[root] = rwSnapshot(t, filepath.Join(game, root))
							}
						}
						u := rwUpdater(payload, "1.5.2", false)
						if extra {
							u = rwUpdater(extraPayload, "0.1.4", true)
						}
						calls, refused := 0, 0
						u.rename = func(a, b string) error {
							calls++
							if calls == k || (r == -1 && calls > k) || (r > 0 && calls == k+r) {
								refused++
								return rwRefusal(eno, a, b)
							}
							return os.Rename(a, b)
						}
						var err error
						if extra {
							_, err = u.applyExtra(context.Background(), game, rwExtraInfo("0.1.4"))
						} else {
							_, err = u.apply(context.Background(), game, guildInfoForVersion("1.5.2"))
						}
						if err == nil {
							t.Fatal("injected fault was accepted")
						}
						if refused < 1 || (r != 0 && refused < 2) {
							t.Fatalf("fault did not fire (%d refused)", refused)
						}
						after := rwSnapshot(t, game)
						exact := reflect.DeepEqual(before, after)
						lb, le := os.ReadFile(filepath.Join(game, "RelaxCoachBackups", lockName))
						locked := le == nil
						if r == 0 && (!exact || locked || !strings.HasPrefix(err.Error(), "Close WoW and try again")) {
							t.Errorf("single fault: exact=%t lock=%t err=%v", exact, locked, err)
						}
						if r != 0 {
							if !locked || exact {
								t.Errorf("rollback fault: exact=%t lock=%t err=%v", exact, locked, err)
							}
							rwCloseAdvice(t, err, game)
						}
						tx := strings.TrimSpace(string(lb))
						for _, root := range roots {
							live := filepath.Join(game, root)
							state := "absent"
							if _, e := os.Stat(live); e == nil {
								state = "new"
								if !fresh && reflect.DeepEqual(old[root], rwSnapshot(t, live)) {
									state = "old"
								}
							}
							if !fresh && state != "old" {
								backup := filepath.Join(tx, "previous", root)
								if extra {
									backup = filepath.Join(tx, "previous")
								}
								if _, e := os.Stat(backup); e != nil || !reflect.DeepEqual(old[root], rwSnapshot(t, backup)) {
									t.Errorf("original %s not preserved at its live or recorded backup path", root)
								}
							}
						}
						for _, p := range []string{"WTF/Config.wtf", "RelaxCoach.exe", "RelaxCoachBackups/prior-backup/keep.txt"} {
							if before[p] != after[p] {
								t.Errorf("unrelated bytes changed: %s", p)
							}
						}
						rwFinish(t, u, game, extra)
					})
				}
			}
		}
	}
}

// A cancel after the last publish, with one rollback rename refused.
func TestRWLastPublishRollback(t *testing.T) {
	for _, fresh := range []bool{false, true} {
		forward, kind := 8, "bundle-update"
		if fresh {
			forward, kind = 4, "bundle-fresh"
		}
		for _, eno := range []syscall.Errno{syscall.EBUSY, syscall.ETXTBSY} {
			for r := 0; r <= forward; r++ {
				t.Run(fmt.Sprintf("%s/errno%d/r%d", kind, eno, r), func(t *testing.T) {
					game := t.TempDir()
					rwSeed(t, game, false, fresh)
					before := rwSnapshot(t, game)
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					u := rwUpdater(bundleZip(t, "1.5.2", rwSmall), "1.5.2", false)
					calls := 0
					u.rename = func(a, b string) error {
						calls++
						if r > 0 && calls == forward+r {
							return rwRefusal(eno, a, b)
						}
						err := os.Rename(a, b)
						if calls == forward && err == nil {
							cancel()
						}
						return err
					}
					_, err := u.apply(ctx, game, guildInfoForVersion("1.5.2"))
					if err == nil {
						t.Fatal("post-publish cancellation accepted")
					}
					exact := reflect.DeepEqual(before, rwSnapshot(t, game))
					_, le := os.Stat(filepath.Join(game, "RelaxCoachBackups/.addon-update.lock"))
					if r == 0 && (!exact || le == nil) {
						t.Fatal("clean rollback did not restore")
					}
					if r > 0 {
						rwCloseAdvice(t, err, game)
					}
					rwFinish(t, u, game, false)
				})
			}
		}
	}
}
