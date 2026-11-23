package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRetirementRenameFailureLeavesOriginalAndRetries(t *testing.T) {
	game := retirementFixture(t)
	source := "Interface/AddOns/ForeverCoach/Core.lua"
	addonPut(t, game, source, "untouched")
	old := retirementRename
	t.Cleanup(func() { retirementRename = old })
	retirementRename = func(*os.Root, string, string) error { return errors.New("rename refused") }
	r := retireGame(context.Background(), game, retirementClosed)
	data, err := os.ReadFile(filepath.Join(game, source))
	if err != nil || string(data) != "untouched" {
		t.Fatalf("failed rename changed source: %q %v", data, err)
	}
	if !strings.Contains(strings.Join(r.Skipped, " "), "left in place; will retry next start") {
		t.Fatalf("failure not actionable: %+v", r)
	}
	if runs := retirementTestRuns(t, game); len(runs) != 0 {
		t.Fatalf("failed pass left run dirs: %v", runs)
	}
	retirementRename = old
	retireGame(context.Background(), game, retirementClosed)
	retirementExists(t, filepath.Join(game, source), false)
	runs := retirementTestRuns(t, game)
	if len(runs) != 1 {
		t.Fatal(runs)
	}
	retirementTestManifest(t, game, runs[0])
}
func TestRetirementQuarantineLinkMovesNothing(t *testing.T) {
	game := retirementFixture(t)
	for _, p := range []string{"Interface/AddOns/ForeverCoach/Core.lua", "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"} {
		addonPut(t, game, p, "keep")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(game, "RelaxCoachRetired")); err != nil {
		t.Skip(err)
	}
	r := retireGame(context.Background(), game, retirementClosed)
	for _, p := range []string{"Interface/AddOns/ForeverCoach/Core.lua", "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"} {
		retirementExists(t, filepath.Join(game, p), true)
	}
	if len(r.Skipped) == 0 {
		t.Fatal("link was not reported")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("wrote through link: %v %v", entries, err)
	}
}
func retirementTestSnapshot(t *testing.T, path string) map[string]string {
	t.Helper()
	data := map[string]string{}
	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(path, p)
		if err != nil {
			return err
		}
		if info.IsDir() {
			data[rel] = "dir"
			return nil
		}
		b, err := os.ReadFile(p)
		data[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
func TestRetirementTimestampCollisionNeverOverwrites(t *testing.T) {
	game := retirementFixture(t)
	old := retirementNow
	t.Cleanup(func() { retirementNow = old })
	retirementNow = func() time.Time { return time.Date(2026, 11, 23, 1, 45, 0, 0, time.UTC) }
	addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "first")
	retireGame(context.Background(), game, retirementClosed)
	first := filepath.Join(game, "RelaxCoachRetired", "20261123T014500Z")
	before := retirementTestSnapshot(t, first)
	addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "second")
	retireGame(context.Background(), game, retirementClosed)
	if got := retirementTestRuns(t, game); !reflect.DeepEqual(got, []string{"20261123T014500Z", "20261123T014500Z-1"}) {
		t.Fatalf("collision: %v", got)
	}
	if !reflect.DeepEqual(before, retirementTestSnapshot(t, first)) {
		t.Fatal("previous quarantine changed")
	}
	entries := retirementTestManifest(t, game, "20261123T014500Z-1")
	if entries[1].SHA256 != retirementTestDigest("second") {
		t.Fatal(entries)
	}
}
func TestRetirementCrashAfterRenameRecoversManifest(t *testing.T) {
	game := retirementFixture(t)
	addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "recover me")
	old := retirementRename
	t.Cleanup(func() { retirementRename = old })
	retirementRename = func(root *os.Root, from, to string) error {
		if err := root.Rename(from, to); err != nil {
			t.Fatal(err)
		}
		panic("simulated process exit after rename")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("crash seam not reached")
			}
		}()
		retireGame(context.Background(), game, retirementClosed)
	}()
	retirementRename = old
	retirementExists(t, filepath.Join(game, "Interface/AddOns/ForeverCoach"), false)
	r := retireGame(context.Background(), game, retirementClosed)
	if len(r.Skipped) != 0 {
		t.Fatalf("recovery failed: %+v", r)
	}
	runs := retirementTestRuns(t, game)
	if len(runs) != 1 {
		t.Fatal(runs)
	}
	entries := retirementTestManifest(t, game, runs[0])
	if len(entries) != 2 || entries[1].SHA256 != retirementTestDigest("recover me") {
		t.Fatal(entries)
	}
}
func TestRetirementRechecksWoWImmediatelyBeforeSaveMove(t *testing.T) {
	game := retirementFixture(t)
	addonPut(t, game, "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua", "keep")
	checks := 0
	running := func(context.Context) (bool, error) { checks++; return checks >= 3, nil }
	r := retireGame(context.Background(), game, running)
	if !r.SavedSettingsDeferred {
		t.Fatalf("no final gate (checks=%d): %+v", checks, r)
	}
	retirementExists(t, filepath.Join(game, "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"), true)
	if len(retirementTestRuns(t, game)) != 0 {
		t.Fatal("deferred-only run directory")
	}
}
func TestRetirementDetectsNewFileDuringMove(t *testing.T) {
	game := retirementFixture(t)
	addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "original")
	old := retirementRename
	t.Cleanup(func() { retirementRename = old })
	retirementRename = func(root *os.Root, from, to string) error {
		if err := old(root, from, to); err != nil {
			return err
		}
		return root.WriteFile(filepath.Join(to, "new.lua"), []byte("new"), 0600)
	}
	r := retireGame(context.Background(), game, retirementClosed)
	if !strings.Contains(strings.Join(r.Skipped, " "), "hash verification failed") {
		t.Fatalf("unmanifested file was not detected: %+v", r)
	}
}

func TestRetirementVerifiesHashesAfterMove(t *testing.T) {
	game := retirementFixture(t)
	addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "before")
	old := retirementRename
	t.Cleanup(func() { retirementRename = old })
	retirementRename = func(root *os.Root, from, to string) error {
		if err := root.Rename(from, to); err != nil {
			return err
		}
		return root.WriteFile(filepath.Join(to, "Core.lua"), []byte("after"), 0600)
	}
	r := retireGame(context.Background(), game, retirementClosed)
	if !strings.Contains(strings.Join(r.Skipped, " "), "hash verification failed") {
		t.Fatalf("post-move hash not checked: %+v", r)
	}
	runs := retirementTestRuns(t, game)
	retirementExists(t, filepath.Join(game, "RelaxCoachRetired", runs[0], "pending.json"), true)
}
