package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetirementJourneySaveFailureRetries(t *testing.T) {
	game := retirementFixture(t)
	addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Relax Journey\n")
	save := "WTF/Account/ACCT/SavedVariables/Journey_Account.lua"
	addonPut(t, game, save, "retry this save")
	old := retirementRename
	t.Cleanup(func() { retirementRename = old })
	retirementRename = func(root *os.Root, from, to string) error {
		if filepath.ToSlash(from) == save {
			return errors.New("locked save")
		}
		return old(root, from, to)
	}
	first := retireGame(context.Background(), game, retirementClosed)
	if !strings.Contains(strings.Join(first.Skipped, " "), "left in place; will retry next start") {
		t.Fatal(first)
	}
	retirementExists(t, filepath.Join(game, save), true)
	retirementRename = old
	second := retireGame(context.Background(), game, retirementClosed)
	retirementKeptOrMoved(t, game, filepath.Join(game, save), false)
	if len(second.Skipped) != 0 {
		t.Fatal(second)
	}
	if retirementKnownJourney(game, &retirementResult{}) {
		t.Fatal("finished cleanup retained receipt")
	}
}
func TestRetirementDestinationAppearsAtFinalGate(t *testing.T) {
	game := retirementFixture(t)
	save := "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"
	addonPut(t, game, save, "keep")
	q, err := newRetirementQuarantine(game)
	if err != nil {
		t.Fatal(err)
	}
	defer q.close()
	result := retirementResult{}
	q.move(filepath.Join(game, save), filepath.FromSlash(save), &result, func() bool {
		if err := q.root.WriteFile(filepath.Join(q.run, save), []byte("existing"), 0600); err != nil {
			t.Fatal(err)
		}
		return true
	})
	retirementExists(t, filepath.Join(game, save), true)
	if len(result.Moved) != 0 || len(result.Skipped) == 0 {
		t.Fatal(result)
	}
}
func TestRetirementDestinationExistsNeverOverwrites(t *testing.T) {
	game := retirementFixture(t)
	source := "Interface/AddOns/ForeverCoach"
	addonPut(t, game, source+"/Core.lua", "source")
	q, err := newRetirementQuarantine(game)
	if err != nil {
		t.Fatal(err)
	}
	defer q.close()
	if err = q.prepare("AddOns/ForeverCoach"); err != nil {
		t.Fatal(err)
	}
	addonPut(t, game, filepath.Join(q.run, "AddOns/ForeverCoach/Core.lua"), "destination")
	result := retirementResult{}
	q.move(filepath.Join(game, source), "AddOns/ForeverCoach", &result)
	if len(result.Moved) != 0 || len(result.Skipped) != 1 {
		t.Fatal(result)
	}
	data, err := os.ReadFile(filepath.Join(game, source, "Core.lua"))
	if err != nil || string(data) != "source" {
		t.Fatalf("source changed: %q %v", data, err)
	}
	data, err = os.ReadFile(filepath.Join(game, q.run, "AddOns/ForeverCoach/Core.lua"))
	if err != nil || string(data) != "destination" {
		t.Fatalf("destination changed: %q %v", data, err)
	}
}
func TestRetirementJournalOrderingAndWriteFailureRecovery(t *testing.T) {
	game := retirementFixture(t)
	addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "first")
	addonPut(t, game, "Interface/AddOns/RelaxRaid/Core.lua", "second")
	old := retirementRename
	t.Cleanup(func() { retirementRename = old })
	var run string
	retirementRename = func(root *os.Root, from, to string) error {
		run = filepath.Join(retirementStorage, strings.Split(filepath.ToSlash(to), "/")[1])
		// No manifest may claim the pending move before rename.
		retirementExists(t, filepath.Join(game, run, "pending.json"), true)
		retirementExists(t, filepath.Join(game, run, "manifest.json"), false)
		if err := old(root, from, to); err != nil {
			return err
		}
		// A file publication failure after rename must retain the intent and
		// stop further moves, not delete data or invent a completed manifest.
		return root.Mkdir(filepath.Join(run, "manifest.json"), 0700)
	}
	result := retireGame(context.Background(), game, retirementClosed)
	if len(result.Moved) != 1 || !strings.Contains(strings.Join(result.Skipped, " "), "journal kept") {
		t.Fatal(result)
	}
	retirementExists(t, filepath.Join(game, "Interface/AddOns/RelaxRaid/Core.lua"), true)
	retirementExists(t, filepath.Join(game, run, "pending.json"), true)
	if err := os.Remove(filepath.Join(game, run, "manifest.json")); err != nil {
		t.Fatal(err)
	}
	retirementRename = old
	result = retireGame(context.Background(), game, retirementClosed)
	if len(result.Skipped) != 0 || len(result.Moved) != 1 {
		t.Fatal(result)
	}
	entries := retirementTestManifest(t, game, filepath.Base(run))
	if len(entries) != 2 {
		t.Fatal(entries)
	}
	retirementExists(t, filepath.Join(game, run, "pending.json"), false)
}
