package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestAddonTasklistDetectsGameButNotVoiceProxy(t *testing.T) {
	for _, tc := range []struct {
		names   []string
		running bool
	}{
		{[]string{"System", "RelaxCoach.exe", "WoWVoiceProxy.exe", "VoiceProxy.exe"}, false},
		{[]string{"Wow.exe"}, true},
		{[]string{"wow-64.exe"}, true},
		{[]string{"WowClassic.exe"}, true},
		{[]string{"WowClassicT.exe"}, true},
		{[]string{"WowB.exe"}, true},
		{[]string{"WoWVoiceProxy-64.exe", "WOWFOREVER.EXE"}, true},
	} {
		var csv strings.Builder
		for _, name := range tc.names {
			fmt.Fprintf(&csv, "%q,\"123\",\"Console\",\"1\",\"20,000 K\"\r\n", name)
		}
		got, err := addonTasklistRunning([]byte(csv.String()))
		if err != nil || got != tc.running {
			t.Errorf("processes %v: running=%v, err=%v", tc.names, got, err)
		}
	}
	for _, bad := range []string{"", "ERROR: Access denied", "\"System\",\"not-a-pid\",\"Console\",\"1\",\"1 K\"", "\"System\",\"12\"", "\"unclosed"} {
		if _, err := addonTasklistRunning([]byte(bad)); err == nil {
			t.Errorf("accepted failed process detection: %q", bad)
		}
	}
}

// From 1.5.2 a running WoW (or unknown detection) no longer blocks: the
// install goes ahead, detection is still asked before download and right
// before the swap, and the result says what loads the new files.
func TestAddonApplyWhileGameRunningInstallsAndAdvises(t *testing.T) {
	for _, tc := range []struct {
		name        string
		runningAt   int
		detectorErr error
	}{
		{"already running", 1, nil},
		{"started while downloading", 2, nil},
		{"detection failed", 1, errors.New("tasklist access denied")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := addonPayloadUpdater(t, addonZip(t, addonFixtureEntries("1.4.1")), "")
			calls := 0
			u.running = func(context.Context) (bool, error) {
				calls++
				if calls == tc.runningAt {
					return true, tc.detectorErr
				}
				return false, nil
			}
			game := addonTestGame(t)
			res, err := u.apply(context.Background(), game, addonTestInfo())
			if err != nil {
				t.Fatalf("refused while game running/detection unknown: %v", err)
			}
			if calls != 2 {
				t.Fatalf("detection calls = %d; want 2", calls)
			}
			// The last check (right before the swap) decides; the fixture
			// release adds files the test game does not have yet.
			want := addonAdviceStart
			if tc.runningAt == 2 {
				want = addonAdviceRestart
			}
			if !strings.HasSuffix(res.Message, want) {
				t.Fatalf("message %q, want advice %q", res.Message, want)
			}
		})
	}
}

func TestAddonApplyRechecksInstalledVersionAtCommit(t *testing.T) {
	u := addonPayloadUpdater(t, addonZip(t, addonFixtureEntries("1.4.1")), "")
	game := addonTestGame(t)
	calls := 0
	u.running = func(context.Context) (bool, error) {
		calls++
		if calls == 2 {
			addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Title: Relax Coach\n## Version: 1.6.0\n## SavedVariables: ForeverCoachDB\n")
		}
		return false, nil
	}
	if _, err := u.apply(context.Background(), game, addonTestInfo()); err == nil {
		t.Fatal("overwrote a concurrent newer install")
	}
	if got := InstalledCoachVersion(game); got != "1.6.0" {
		t.Fatalf("downgraded concurrently updated addon: %s", got)
	}
}

func TestAddonApplyRollsBackMidSwapAndVerificationFailure(t *testing.T) {
	for _, mode := range []string{"rename-failure", "corrupt-installed", "cancel-after-rename"} {
		t.Run(mode, func(t *testing.T) {
			u := addonPayloadUpdater(t, addonZip(t, addonFixtureEntries("1.4.1")), "")
			game := addonTestGame(t)
			before := addonTreeHashes(t, game)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			calls := 0
			u.rename = func(from, to string) error {
				calls++
				if calls == 2 && mode == "rename-failure" {
					return os.ErrPermission
				}
				if err := os.Rename(from, to); err != nil {
					return err
				}
				if calls == 2 {
					switch mode {
					case "corrupt-installed":
						return os.WriteFile(filepath.Join(to, "Core.lua"), []byte("bad bytes"), 0o644)
					case "cancel-after-rename":
						cancel()
					}
				}
				return nil
			}
			_, err := u.apply(ctx, game, addonTestInfo())
			if err == nil {
				t.Fatal("accepted incomplete/corrupt installation")
			}
			if !strings.Contains(err.Error(), "restored") {
				t.Fatalf("missing rollback result: %v", err)
			}
			if after := addonTreeHashes(t, game); !reflect.DeepEqual(before, after) {
				t.Fatalf("original bytes were not restored: before=%v, after=%v", before, after)
			}
			if calls < 3 {
				t.Fatalf("did not attempt rollback: %d renames", calls)
			}
		})
	}
}
