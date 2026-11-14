package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// appFirstBridge: Coach 1.6.0 installed, this app is 1.6.0, the checked release
// is release; applied counts real installs.
func appFirstBridge(t *testing.T, release AddonReleaseInfo, lastState string) (*nativeBridge, *int) {
	t.Helper()
	old := Version
	Version = "1.6.0"
	t.Cleanup(func() { Version = old })
	a, _ := nativeTestApp(t)
	nativeInstalled(t, a, "1.6.0")
	applied := 0
	b := newNativeBridge(a, false, nativeServices{
		selfUpdate: func() bool { return true },
		check:      func(context.Context, string) (AddonReleaseInfo, error) { return release, nil },
		apply: func(_ context.Context, _ string, r AddonReleaseInfo) (AddonUpdateResult, error) {
			applied++
			return AddonUpdateResult{Version: r.Version}, nil
		},
		appLast: func() (string, string, error) { return "tx", lastState, nil },
	})
	t.Cleanup(b.Close)
	return b, &applied
}

func TestAppFirstRefusesABundleNewerThanTheApp(t *testing.T) {
	release := AddonReleaseInfo{Version: "1.7.0", AssetURL: "a", ChecksumURL: "c", Available: true, HasApp: true}
	for _, last := range []string{"", "committed"} {
		b, applied := appFirstBridge(t, release, last)
		value, err := b.Call("checkAddon", nil)
		if err != nil {
			t.Fatal(err)
		}
		o := value.(nativeOffer)
		if !o.NeedsApp || o.Available || o.Version != "1.7.0" {
			t.Fatalf("last %q: offer %+v", last, o)
		}
		_, err = b.Call("applyAddon", nativeArgs("1.7.0"))
		if err == nil || !strings.Contains(err.Error(), "Update the app first") || !strings.Contains(err.Error(), "https://github.com/thegamersvibe/relax-coach/releases/latest") || *applied != 0 {
			t.Fatalf("last %q: apply %v, installs %d", last, err, *applied)
		}
	}
}

func TestAppFirstLeavesOtherReleasesAlone(t *testing.T) {
	for name, release := range map[string]AddonReleaseInfo{
		"no app in the release":        {Version: "1.7.0", AssetURL: "a", ChecksumURL: "c", Available: true},
		"same version as the app":      {Version: "1.6.0", AssetURL: "a", ChecksumURL: "c", Available: true, HasApp: true, Repair: true},
		"older than the app":           {Version: "1.5.9", AssetURL: "a", ChecksumURL: "c", Available: true, HasApp: true},
		"unparsable version (no rule)": {Version: "x", AssetURL: "a", ChecksumURL: "c", Available: true, HasApp: true},
	} {
		b, applied := appFirstBridge(t, release, "committed")
		value, err := b.Call("checkAddon", nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if o := value.(nativeOffer); o.NeedsApp || !o.Available {
			t.Fatalf("%s: %+v", name, o)
		}
		if _, err := b.Call("applyAddon", nativeArgs(release.Version)); err != nil || *applied != 1 {
			t.Fatalf("%s: %v %d", name, err, *applied)
		}
	}
}

func TestAppFirstEscapeAfterAFailedAppUpdate(t *testing.T) {
	release := AddonReleaseInfo{Version: "1.7.0", AssetURL: "a", ChecksumURL: "c", Available: true, HasApp: true}
	for _, last := range []string{"failed", "rolled-back", "recovery"} {
		b, applied := appFirstBridge(t, release, last)
		value, err := b.Call("checkAddon", nil)
		if err != nil {
			t.Fatal(err)
		}
		if o := value.(nativeOffer); o.NeedsApp || !o.Available {
			t.Fatalf("%s: %+v", last, o)
		}
		if _, err := b.Call("applyAddon", nativeArgs("1.7.0")); err != nil || *applied != 1 {
			t.Fatalf("%s: the bundle did not reach the normal validation: %v %d", last, err, *applied)
		}
	}
	// An unreadable last result is not an escape.
	b, applied := appFirstBridge(t, release, "")
	b.services.appLast = func() (string, string, error) { return "", "failed", errors.New("unreadable") }
	b.Call("checkAddon", nil)
	if _, err := b.Call("applyAddon", nativeArgs("1.7.0")); err == nil || *applied != 0 {
		t.Fatal("an unreadable result opened the escape")
	}
}

func TestAddonCheckNotesTheAppAsset(t *testing.T) {
	// The release parser records RelaxCoach.exe next to the bundle.
	withApp := addonReleaseJSON("v1.7.0", "RelaxCoach.exe", "RelaxCoach-Bundle-1.7.0.zip", "SHA256SUMS.txt")
	without := addonReleaseJSON("v1.7.0", "RelaxCoach-Bundle-1.7.0.zip", "SHA256SUMS.txt")
	for body, want := range map[string]bool{withApp: true, without: false} {
		u := addonTestUpdater(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) })
		info, err := u.check(context.Background(), "1.6.0")
		if err != nil || info.HasApp != want || !info.Available {
			t.Fatalf("HasApp %v want %v: %+v %v", info.HasApp, want, info, err)
		}
	}
}

func addonReleaseJSON(tag string, assets ...string) string {
	var parts []string
	for _, a := range assets {
		parts = append(parts, `{"name":"`+a+`","browser_download_url":"`+addonReleasePrefix+`download/`+tag+`/`+a+`"}`)
	}
	return `{"tag_name":"` + tag + `","name":"r","body":"","draft":false,"prerelease":false,"assets":[` + strings.Join(parts, ",") + `]}`
}

func afterAppBridge(t *testing.T, startup func() (string, string, bool), last func() (string, string, error)) *nativeBridge {
	t.Helper()
	a, _ := nativeTestApp(t)
	b := newNativeBridge(a, false, nativeServices{startup: startup, appLast: last})
	t.Cleanup(b.Close)
	return b
}

func TestAfterAppUpdateOnlyAfterThisTransactionCommitted(t *testing.T) {
	oldWait, oldPoll := nativeAfterAppWait, nativeAfterAppPoll
	nativeAfterAppWait, nativeAfterAppPoll = 300*time.Millisecond, 10*time.Millisecond
	t.Cleanup(func() { nativeAfterAppWait, nativeAfterAppPoll = oldWait, oldPoll })
	started := func() (string, string, bool) { return "tx1", "1.5.6", true }
	for _, c := range []struct {
		name    string
		startup func() (string, string, bool)
		last    func() (string, string, error)
	}{
		{"not started by an app update", func() (string, string, bool) { return "", "", false }, func() (string, string, error) { return "tx1", "committed", nil }},
		{"another transaction committed", started, func() (string, string, error) { return "tx0", "committed", nil }},
		{"this transaction rolled back", started, func() (string, string, error) { return "tx1", "rolled-back", nil }},
		{"this transaction failed", started, func() (string, string, error) { return "tx1", "failed", nil }},
		{"nothing recorded in time", started, func() (string, string, error) { return "", "", nil }},
		{"unreadable result", started, func() (string, string, error) { return "tx1", "committed", errors.New("bad") }},
	} {
		b := afterAppBridge(t, c.startup, c.last)
		begin := time.Now()
		value, err := b.Call("afterAppUpdate", nil)
		if err != nil || value.(AfterAppUpdate).Updated {
			t.Fatalf("%s: %+v %v", c.name, value, err)
		}
		if time.Since(begin) > 2*time.Second {
			t.Fatalf("%s: waited too long", c.name)
		}
	}
	// Committed after a few polls: true once, with the old version and the game.
	polls := 0
	b := afterAppBridge(t, started, func() (string, string, error) {
		polls++
		if polls < 4 {
			return "tx0", "committed", nil // the previous result until the worker writes ours
		}
		return "tx1", "committed", nil
	})
	value, err := b.Call("afterAppUpdate", nil)
	got := value.(AfterAppUpdate)
	if err != nil || !got.Updated || got.From != "1.5.6" || got.Game != b.app.snapshot().Game || polls < 4 {
		t.Fatalf("%+v %v polls %d", got, err, polls)
	}
	if value, _ := b.Call("afterAppUpdate", nil); value.(AfterAppUpdate).Updated {
		t.Fatal("answered true twice in one process")
	}
	if _, err := b.Call("afterAppUpdate", nativeArgs("x")); err == nil {
		t.Fatal("afterAppUpdate accepted an argument")
	}
}

func TestGameStateMapsTheDetector(t *testing.T) {
	a, _ := nativeTestApp(t)
	for want, fn := range map[string]func(context.Context) (bool, error){
		"open":    func(context.Context) (bool, error) { return true, nil },
		"closed":  func(context.Context) (bool, error) { return false, nil },
		"unknown": func(context.Context) (bool, error) { return true, errors.New("tasklist failed") },
	} {
		b := newNativeBridge(a, false, nativeServices{gameOpen: fn})
		value, err := b.Call("gameState", nil)
		b.Close()
		if err != nil || value != want {
			t.Fatalf("want %s: %v %v", want, value, err)
		}
	}
}

func TestAppLastResultStateReadsTheFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "RelaxCoach.exe")
	if id, state, err := appLastResultState(target); id != "" || state != "" || err != nil {
		t.Fatalf("missing file: %q %q %v", id, state, err)
	}
	os.MkdirAll(appRoot(target), 0755)
	os.WriteFile(filepath.Join(appRoot(target), "last-result.json"), []byte(`{"ID":"abc","State":"rolled-back","Message":"m"}`), 0644)
	if id, state, err := appLastResultState(target); id != "abc" || state != "rolled-back" || err != nil {
		t.Fatalf("%q %q %v", id, state, err)
	}
	os.WriteFile(filepath.Join(appRoot(target), "last-result.json"), []byte(`{`), 0644)
	if _, _, err := appLastResultState(target); err == nil {
		t.Fatal("garbage accepted")
	}
}

// Release rule for 1.6.0: the Coach file set must not change, so a 1.5.6 app
// that presses Update Addon before Update App still installs the 1.6.0 bundle.
func TestCoachFileSetUnchangedFrom156To160(t *testing.T) {
	// The Coach file set of 1.5.6, copied from v1.5.6 (62ecfa9). A 1.5.6 app
	// accepts exactly this set; 1.6.0 must ship it unchanged.
	want := map[string]bool{}
	for _, name := range []string{
		"ForeverCoach.toc", "Core.lua", "Rules.lua", "Route.lua", "Session.lua", "Gear.lua",
		"Talents.lua", "Dungeons.lua", "Audit.lua", "Macros.lua", "UI.lua", "ForeverCoach.tga",
		"Reactions.lua",
		"Media/Reactions/bags_1.mp3", "Media/Reactions/bags_2.mp3", "Media/Reactions/death.mp3",
		"Media/Reactions/level_up.mp3", "Media/Reactions/level_up.tga",
		"Media/Reactions/campfire_1.mp3", "Media/Reactions/campfire_2.mp3",
	} {
		want[name] = true
	}
	for _, v := range []string{"1.5.6", "1.6.0"} {
		if got := addonPublicFilesForVersion(v); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s Coach file set differs from 1.5.6's (apps 1.5.6 and older would refuse the bundle): %v", v, got)
		}
	}
}

func TestAppFirstNeverHoldsBackAnAppThatCannotUpdateItself(t *testing.T) {
	release := AddonReleaseInfo{Version: "1.7.0", AssetURL: "a", ChecksumURL: "c", Available: true, HasApp: true}
	// Self-update unavailable in this process (appRuntime nil): no gate.
	b, applied := appFirstBridge(t, release, "")
	b.services.selfUpdate = func() bool { return false }
	if o, _ := b.Call("checkAddon", nil); o.(nativeOffer).NeedsApp || !o.(nativeOffer).Available {
		t.Fatalf("no self-update: %+v", o)
	}
	if _, err := b.Call("applyAddon", nativeArgs("1.7.0")); err != nil || *applied != 1 {
		t.Fatalf("no self-update: %v %d", err, *applied)
	}
	// An app update that fails in this process before it has a transaction
	// (bad checksum entry, staging) writes no last-result: the escape applies.
	for _, cause := range []string{"the app checksum is missing from SHA256SUMS.txt", "could not stage the update"} {
		b, applied := appFirstBridge(t, release, "")
		b.services.checkApp = func(context.Context, string) (AppReleaseInfo, error) {
			return AppReleaseInfo{Version: "1.7.0", Available: true}, nil
		}
		b.services.applyApp = func(context.Context, AppReleaseInfo) (AppUpdateResult, error) {
			return AppUpdateResult{}, errors.New(cause)
		}
		if o, _ := b.Call("checkAddon", nil); !o.(nativeOffer).NeedsApp {
			t.Fatalf("%s: gate missing before the failure: %+v", cause, o)
		}
		b.Call("checkApp", nil)
		if _, err := b.Call("applyApp", nativeArgs("1.7.0")); err == nil {
			t.Fatal("app update did not fail")
		}
		if o, _ := b.Call("checkAddon", nil); o.(nativeOffer).NeedsApp || !o.(nativeOffer).Available {
			t.Fatalf("%s: still held back: %+v", cause, o)
		}
		if _, err := b.Call("applyAddon", nativeArgs("1.7.0")); err != nil || *applied != 1 {
			t.Fatalf("%s: %v %d", cause, err, *applied)
		}
	}
	// A refused click (no checked app release) is not a failed update: the gate stays.
	b, _ = appFirstBridge(t, release, "")
	b.Call("applyApp", nativeArgs("1.7.0"))
	if o, _ := b.Call("checkAddon", nil); !o.(nativeOffer).NeedsApp {
		t.Fatalf("a refused click opened the escape: %+v", o)
	}
}
