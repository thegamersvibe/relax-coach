package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRetirementCloseForgetsPublishedTempName(t *testing.T) {
	game := retirementFixture(t)
	q, err := newRetirementQuarantine(game)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.prepare("AddOns/ForeverCoach"); err != nil {
		t.Fatal(err)
	}
	if err := q.atomicJSON("pending.json", retirementManifest{Schema: 1}); err != nil {
		t.Fatal(err)
	}
	pending := filepath.Join(game, q.run, "pending.json")
	before, err := os.ReadFile(pending)
	if err != nil {
		t.Fatal(err)
	}
	// atomicJSON renamed metadata-1.tmp, so this is now another writer's file,
	// not our temporary journal. An otherwise failed/empty pass calls close.
	temp := filepath.Join(game, q.run, "metadata-1.tmp")
	if err := os.WriteFile(temp, []byte("another writer\x00\xff"), 0600); err != nil {
		t.Fatal(err)
	}
	q.close()
	if got, err := os.ReadFile(temp); err != nil || string(got) != "another writer\x00\xff" {
		t.Errorf("close removed a relinquished temp name: %q %v", got, err)
	}
	if got, err := os.ReadFile(pending); err != nil || string(got) != string(before) {
		t.Errorf("close touched pending.json: %q %v", got, err)
	}
}

func TestRetirementCloseDoesNotRemoveReplacementEntry(t *testing.T) {
	game := retirementFixture(t)
	q, err := newRetirementQuarantine(game)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.prepare("AddOns/ForeverCoach"); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(game, q.run, "AddOns")
	if err := os.Rename(parent, parent+".kept"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte("not created by this pass"), 0600); err != nil {
		t.Fatal(err)
	}
	q.close()
	if got, err := os.ReadFile(parent); err != nil || string(got) != "not created by this pass" {
		t.Fatalf("close removed a replacement entry: %q %v", got, err)
	}
}
