//go:build linux && amd64

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Kill-point harness adapted from مانع's review probe (astra_fj_probe_test.go).
var fjRoots = []string{"Interface/AddOns/ForeverCoach", "Interface/AddOns/Journey", "Interface/Content/Journey-Content", "Interface/AddOns/RelaxQuestPlanner"}
var fjNames = []string{"C", "J", "G", "P"}

// All HTTP responses are explicit test fixtures in memory, never network I/O.
type fjTransport map[string][]byte

func (m fjTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	b, ok := m[r.URL.String()]
	if !ok {
		return nil, fmt.Errorf("unexpected fixture request %s", r.URL)
	}
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(b)), ContentLength: int64(len(b)), Request: r}, nil
}
func fjNew(m fjTransport) *addonUpdater {
	u := newAddonUpdater()
	u.client.Transport = m
	u.running = func(context.Context) (bool, error) { return false, nil }
	return u
}
func fjBundle(t *testing.T, v string) []byte {
	return bundleZip(t, v, func(e []addonZipEntry, m map[string]any) []addonZipEntry {
		for i, root := range addonGuildRootsFor(v) {
			if i == 0 {
				continue
			}
			ext := ".txt"
			if i == 2 {
				ext = ".html"
			}
			e = append(e, addonZipEntry{root + "/__fj_generation" + ext, v, 0644})
		}
		return e
	})
}
func fjUpdater(t *testing.T, v string) *addonUpdater {
	p := fjBundle(t, v)
	i := guildInfoForVersion(v)
	r := addonTestRelease("v" + v)
	r["assets"] = []map[string]string{{"name": i.AssetName, "browser_download_url": i.AssetURL}, {"name": "SHA256SUMS.txt", "browser_download_url": i.ChecksumURL}}
	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	return fjNew(fjTransport{addonLatestURL: raw, i.AssetURL: p, i.ChecksumURL: []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(p), i.AssetName))})
}
func fjExtra(t *testing.T, v string) *addonUpdater {
	a, _ := extraAddonByID("forevermaps")
	p := extraGoodZip(t, v)
	prefix := a.releasePrefix() + "download/v" + v + "/"
	raw, err := json.Marshal(map[string]any{"tag_name": "v" + v, "assets": []map[string]string{{"name": a.assetName(v), "browser_download_url": prefix + a.assetName(v)}, {"name": "SHA256SUMS.txt", "browser_download_url": prefix + "SHA256SUMS.txt"}}})
	if err != nil {
		t.Fatal(err)
	}
	return fjNew(fjTransport{a.latestURL(): raw, prefix + a.assetName(v): p, prefix + "SHA256SUMS.txt": []byte(fmt.Sprintf("%x  %s\n", sha256.Sum256(p), a.assetName(v)))})
}
func fjExists(p string) bool { _, e := os.Lstat(p); return e == nil }
func fjAge(t *testing.T, p string) {
	t.Helper()
	old := time.Now().Add(-20 * time.Minute)
	if e := os.Chtimes(p, old, old); e != nil {
		t.Fatal(e)
	}
}
func fjGame(t *testing.T) string {
	t.Helper()
	p := t.TempDir()
	addonPut(t, p, "WTF/Config.wtf", "SET locale \"enUS\"\n")
	return p
}
func fjSeed(t *testing.T, g string) {
	t.Helper()
	_, e := fjUpdater(t, "1.4.5").apply(context.Background(), g, guildInfoForVersion("1.4.5"))
	if e != nil {
		t.Fatal(e)
	}
}

type fjFolder struct {
	Exists                      bool
	Version, Generation, Digest string
	Files                       int
}

func fjFolderAt(t *testing.T, p string, i int) fjFolder {
	t.Helper()
	s := fjFolder{Exists: fjExists(p)}
	if !s.Exists {
		return s
	}
	toc := map[int]string{0: "ForeverCoach.toc", 1: "Journey.toc", 3: "RelaxQuestPlanner.toc"}[i]
	if toc != "" {
		b, e := os.ReadFile(filepath.Join(p, toc))
		if e != nil {
			t.Fatal(e)
		}
		for _, l := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(l, "## Version:") {
				s.Version = strings.TrimSpace(strings.TrimPrefix(l, "## Version:"))
			}
		}
	}
	if i == 0 {
		s.Generation = s.Version
	} else {
		ext := ".txt"
		if i == 2 {
			ext = ".html"
			s.Version = "4.00.06 (fixture manifest)"
		}
		b, e := os.ReadFile(filepath.Join(p, "__fj_generation"+ext))
		if e != nil {
			t.Fatal(e)
		}
		s.Generation = string(b)
	}
	hashes := addonTreeHashes(t, p)
	keys := make([]string, 0, len(hashes))
	for k := range hashes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	h := sha256.New()
	for _, k := range keys {
		fmt.Fprintf(h, "%s\x00%s\n", k, hashes[k])
	}
	s.Digest = fmt.Sprintf("%x", h.Sum(nil))
	s.Files = len(keys)
	return s
}

type fjSnapshot struct {
	Coach                     string
	State                     AddonBundleState
	Lock                      bool
	Targets, Previous, Staged map[string]fjFolder
	TransactionExists         bool
}

func fjSnapshotAt(t *testing.T, g, tx string) fjSnapshot {
	t.Helper()
	s := fjSnapshot{Coach: InstalledCoachVersion(g), State: InstalledBundleState(g), Lock: fjExists(filepath.Join(g, "RelaxCoachBackups/.addon-update.lock")), TransactionExists: fjExists(tx), Targets: map[string]fjFolder{}, Previous: map[string]fjFolder{}, Staged: map[string]fjFolder{}}
	for i, r := range fjRoots {
		s.Targets[fjNames[i]] = fjFolderAt(t, filepath.Join(g, r), i)
		if tx != "" {
			s.Previous[fjNames[i]] = fjFolderAt(t, filepath.Join(tx, "previous", r), i)
			s.Staged[fjNames[i]] = fjFolderAt(t, filepath.Join(tx, "staged", r), i)
		}
	}
	return s
}

// Run the actual transaction until the k-th successful rename, then SIGKILL.
// k=0 is the boundary immediately before the first rename after staging.
func TestFJWorker(t *testing.T) {
	g := os.Getenv("FJ_GAME")
	if g == "" {
		return
	}
	k, e := strconv.Atoi(os.Getenv("FJ_K"))
	if e != nil {
		t.Fatal(e)
	}
	u := fjUpdater(t, "1.5.0")
	n := 0
	kill := func() {
		if e := syscall.Kill(os.Getpid(), syscall.SIGKILL); e != nil {
			t.Fatal(e)
		}
		select {}
	}
	u.rename = func(a, b string) error {
		if k == 0 {
			fmt.Println("FJ_BEFORE_RENAME")
			kill()
		}
		if e := os.Rename(a, b); e != nil {
			return e
		}
		n++
		fmt.Printf("FJ_RENAME %d %s -> %s\n", n, a, b)
		if n == k {
			kill()
		}
		return nil
	}
	if os.Getenv("FJ_EXTRA") == "1" {
		u = fjExtra(t, "0.1.3")
		u.rename = func(a, b string) error {
			if e := os.Rename(a, b); e != nil {
				return e
			}
			n++
			fmt.Printf("FJ_RENAME %d %s -> %s\n", n, a, b)
			if n == k {
				kill()
			}
			return nil
		}
		a, _ := extraAddonByID("forevermaps")
		i, e := u.checkExtra(context.Background(), a.ID, InstalledExtraVersion(g, a))
		if e != nil {
			t.Fatal(e)
		}
		_, e = u.applyExtra(context.Background(), g, i)
		t.Fatalf("worker returned without kill: %v", e)
	}
	_, e = u.apply(context.Background(), g, guildInfoForVersion("1.5.0"))
	t.Fatalf("worker returned without kill: %v, renames=%d", e, n)
}
func fjCrash(t *testing.T, g string, k int, extra bool) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFJWorker$", "-test.v")
	cmd.Env = append(os.Environ(), "FJ_GAME="+g, "FJ_K="+strconv.Itoa(k))
	if extra {
		cmd.Env = append(cmd.Env, "FJ_EXTRA=1")
	}
	out, e := cmd.CombinedOutput()
	if e == nil {
		t.Fatalf("worker not killed: %s", out)
	}
	ee, ok := e.(*exec.ExitError)
	if !ok {
		t.Fatal(e)
	}
	st, ok := ee.Sys().(syscall.WaitStatus)
	if !ok || !st.Signaled() || st.Signal() != syscall.SIGKILL || ctx.Err() != nil {
		t.Fatalf("wrong worker exit %v %s", e, out)
	}
	if k > 0 && !strings.Contains(string(out), fmt.Sprintf("FJ_RENAME %d ", k)) {
		t.Fatalf("missing rename evidence %s", out)
	}
	return string(out)
}

// مانع S1 kill matrix (from his review probe): the real transaction runs in a
// subprocess that is SIGKILLed right before the first rename (k=0) or right
// after the k-th rename. Afterwards one check and one click must complete the
// install, at the same release (repair) as well as at a newer one, keeping
// every original folder somewhere (target, the interrupted transaction or the
// new backup) and leaving no lock.
func TestFIKillMatrixOneClickCompletes(t *testing.T) {
	for _, scenario := range []string{"update", "fresh"} {
		max := 7
		if scenario == "fresh" {
			max = 4
		}
		for k := 0; k <= max; k++ {
			for _, release := range []string{"1.5.0", "1.5.1"} {
				t.Run(fmt.Sprintf("%s_k%d_%s", scenario, k, release), func(t *testing.T) {
					g := fjGame(t)
					if scenario == "update" {
						fjSeed(t, g)
					}
					originals := map[string]fjFolder{}
					for i, r := range fjRoots {
						originals[fjNames[i]] = fjFolderAt(t, filepath.Join(g, r), i)
					}
					fjCrash(t, g, k, false)
					lock := filepath.Join(g, "RelaxCoachBackups/.addon-update.lock")
					raw, e := os.ReadFile(lock)
					if e != nil {
						t.Fatal(e)
					}
					tx := strings.TrimSpace(string(raw))
					fjAge(t, lock)
					u := fjUpdater(t, release)
					a := &App{}
					a.setGame(g)
					b := newNativeBridge(a, false, nativeServices{check: u.check, apply: u.apply})
					defer b.Close()
					v, e := b.Call("checkAddon", nil)
					if e != nil {
						t.Fatal(e)
					}
					offer := v.(nativeOffer)
					if !offer.Available || offer.Version != release {
						t.Fatalf("no install offered after the crash: %+v (state %+v)", offer, InstalledBundleState(g))
					}
					if offer.InstalledVersion == release && !offer.Install {
						t.Fatalf("a same-version repair must be labelled Install: %+v", offer)
					}
					v, e = b.Call("applyAddon", nativeArgs(offer.Version))
					if e != nil {
						t.Fatalf("one click did not complete the install: %v", e)
					}
					res := v.(AddonUpdateResult)
					after := fjSnapshotAt(t, g, tx)
					if after.Lock || after.State.State != "installed" {
						t.Fatalf("incomplete after one click: lock=%v state=%+v", after.Lock, after.State)
					}
					for n, f := range after.Targets {
						if f.Generation != release {
							t.Fatalf("%s is generation %q, want %s", n, f.Generation, release)
						}
					}
					for n, o := range originals {
						if !o.Exists {
							continue
						}
						found := false
						for _, s := range []fjFolder{after.Previous[n], fjFolderAt(t, filepath.Join(res.BackupPath, fjRoots[map[string]int{"C": 0, "J": 1, "G": 2, "P": 3}[n]]), map[string]int{"C": 0, "J": 1, "G": 2, "P": 3}[n])} {
							if s.Exists && s.Digest == o.Digest {
								found = true
							}
						}
						if !found {
							t.Fatalf("original %s lost", n)
						}
					}
				})
			}
		}
	}
}
