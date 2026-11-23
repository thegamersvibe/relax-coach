package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Leave real flushed intent and moved bytes, as at a crash before publication.
func retirementInterruptedSave(t *testing.T) (string, string, retirementManifest) {
	t.Helper()
	game := retirementFixture(t)
	source := "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"
	addonPut(t, game, source, "original save")
	q, err := newRetirementQuarantine(game)
	if err != nil {
		t.Fatal(err)
	}
	defer q.close()
	if err := q.prepare(filepath.FromSlash(source)); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(q.run, source)
	entries, err := q.inventory(source, target)
	if err != nil {
		t.Fatal(err)
	}
	intent := retirementManifest{Schema: 1, Entries: entries}
	if err := q.atomicJSON("pending.json", intent); err != nil {
		t.Fatal(err)
	}
	q.inFlight = true
	if err := retirementRename(q.root, source, target); err != nil {
		t.Fatal(err)
	}
	return game, q.run, intent
}

func TestRetirementRecoveryVerifiesBeforeManifestRewrite(t *testing.T) {
	game, run, intent := retirementInterruptedSave(t)
	// Preserve even formatting: a failed recovery may not publish anything.
	manifest := "{\n  \"schema\": 1, \"entries\": []\n}\n"
	addonPut(t, game, filepath.Join(run, "manifest.json"), manifest)
	pending, err := os.ReadFile(filepath.Join(game, run, "pending.json"))
	if err != nil {
		t.Fatal(err)
	}
	addonPut(t, game, intent.Entries[0].Destination, "changed save")
	addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "later move must wait")
	r := retireGame(context.Background(), game, retirementClosed)
	if len(r.Moved) != 0 || !strings.Contains(strings.Join(r.Skipped, " "), "hash verification failed") {
		t.Errorf("mismatch did not fail closed: %+v", r)
	}
	for path, want := range map[string]string{"manifest.json": manifest, "pending.json": string(pending)} {
		if got, err := os.ReadFile(filepath.Join(game, run, path)); err != nil || string(got) != want {
			t.Errorf("mismatch rewrote %s: %q %v", path, got, err)
		}
	}
	retirementExists(t, filepath.Join(game, "Interface/AddOns/ForeverCoach/Core.lua"), true)
}

func TestRetirementJournalPreservesUnknownFields(t *testing.T) {
	for _, name := range []string{"pending.json", "manifest.json"} {
		t.Run(name, func(t *testing.T) {
			game, run, intent := retirementInterruptedSave(t)
			entry, err := json.Marshal(intent.Entries[0])
			if err != nil {
				t.Fatal(err)
			}
			raw := `{"schema":1,"future":{"number":9007199254740993},"entries":[` + strings.TrimSuffix(string(entry), "}") + `,"future_entry":[true,{"note":"keep"}]}]}`
			addonPut(t, game, filepath.Join(run, name), raw)
			q, err := newRetirementQuarantine(game)
			if err != nil {
				t.Fatal(err)
			}
			defer q.close()
			q.run = run
			doc, err := q.readManifest(name)
			if err != nil {
				t.Fatal(err)
			}
			if err := q.atomicJSON(name, doc); err != nil {
				t.Fatal(err)
			}
			got := retirementTestJSON(t, filepath.Join(game, run, name))
			var top map[string]json.RawMessage
			if err := json.Unmarshal(got["future"], &top); err != nil || string(top["number"]) != "9007199254740993" {
				t.Errorf("unknown top-level field lost or changed: %s %v", got["future"], err)
			}
			var entries []map[string]json.RawMessage
			if err := json.Unmarshal(got["entries"], &entries); err != nil || len(entries) != 1 {
				t.Fatalf("entries: %s %v", got["entries"], err)
			}
			var extra []any
			if err := json.Unmarshal(entries[0]["future_entry"], &extra); err != nil || len(extra) != 2 {
				t.Errorf("unknown entry field lost: %s %v", entries[0]["future_entry"], err)
			}
		})
	}
}

func TestRetirementRecoveryPreservesUnknownFields(t *testing.T) {
	for _, published := range []bool{false, true} {
		t.Run(fmt.Sprint(published), func(t *testing.T) {
			game, run, intent := retirementInterruptedSave(t)
			entry, err := json.Marshal(intent.Entries[0])
			if err != nil {
				t.Fatal(err)
			}
			manifestEntries := ""
			if published {
				manifestEntries = strings.TrimSuffix(string(entry), "}") + `,"manifest_entry":"keep manifest entry"}`
			}
			addonPut(t, game, filepath.Join(run, "manifest.json"), `{"schema":1,"manifest_extra":"keep manifest","entries":[`+manifestEntries+`]}`)
			addonPut(t, game, filepath.Join(run, "pending.json"), `{"schema":1,"intent_extra":"keep intent","entries":[`+strings.TrimSuffix(string(entry), "}")+`,"intent_entry":"keep intent entry"}]}`)
			r := retireGame(context.Background(), game, retirementClosed)
			if len(r.Skipped) != 0 || len(r.Moved) != 0 {
				t.Fatalf("recovery failed or repeated move: %+v", r)
			}
			got := retirementTestJSON(t, filepath.Join(game, run, "manifest.json"))
			for key, want := range map[string]string{"manifest_extra": `"keep manifest"`, "intent_extra": `"keep intent"`} {
				if string(got[key]) != want {
					t.Errorf("lost %s: %s", key, got[key])
				}
			}
			var entries []map[string]json.RawMessage
			if err := json.Unmarshal(got["entries"], &entries); err != nil || len(entries) != 1 {
				t.Fatalf("entries: %s %v", got["entries"], err)
			}
			if string(entries[0]["intent_entry"]) != `"keep intent entry"` || (published && string(entries[0]["manifest_entry"]) != `"keep manifest entry"`) {
				t.Errorf("lost entry extensions: %v", entries[0])
			}
			retirementExists(t, filepath.Join(game, run, "pending.json"), false)
			retirementTestManifest(t, game, filepath.Base(run))
		})
	}
}

func TestRetirementRecoveryUnknownFieldConflictKeepsJournals(t *testing.T) {
	for _, location := range []string{"top", "entry"} {
		t.Run(location, func(t *testing.T) {
			game, run, intent := retirementInterruptedSave(t)
			entry, err := json.Marshal(intent.Entries[0])
			if err != nil {
				t.Fatal(err)
			}
			journals := map[string]string{}
			for _, name := range []string{"manifest.json", "pending.json"} {
				top, row := "", string(entry)
				field := `,"future":"` + name + `"`
				if location == "top" {
					top = field
				} else {
					row = strings.TrimSuffix(row, "}") + field + "}"
				}
				journals[name] = `{"schema":1` + top + `,"entries":[` + row + `]}`
				addonPut(t, game, filepath.Join(run, name), journals[name])
			}
			r := retireGame(context.Background(), game, retirementClosed)
			if len(r.Skipped) == 0 || len(r.Moved) != 0 {
				t.Errorf("conflicting extensions not refused: %+v", r)
			}
			for name, want := range journals {
				if got, err := os.ReadFile(filepath.Join(game, run, name)); err != nil || string(got) != want {
					t.Errorf("conflict changed %s: %q %v", name, got, err)
				}
			}
		})
	}
}

func TestRetirementInvalidPendingBlocksLaterPasses(t *testing.T) {
	for _, raw := range []string{`{bad`, `{"schema":2,"entries":[]}`, `{"schema":1,"entries":[]}`, `{"schema":1,"entries":[{"source":"../outside","destination":"../outside","type":"dir"}]}`} {
		t.Run(raw, func(t *testing.T) {
			game := retirementFixture(t)
			addonPut(t, game, "RelaxCoachRetired/20260101T010101Z/pending.json", raw)
			addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "keep addon")
			addonPut(t, game, "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua", "keep save")
			before := retirementTestSnapshot(t, game)
			for start := 0; start < 2; start++ {
				r := retireGame(context.Background(), game, retirementClosed)
				encoded, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				var wire struct{ RecoveryBlocked bool }
				if err := json.Unmarshal(encoded, &wire); err != nil {
					t.Fatal(err)
				}
				text := strings.Join(r.Skipped, " ")
				if !wire.RecoveryBlocked || len(r.Moved) != 0 || !strings.Contains(text, "a previous move could not be verified; nothing more was moved") || strings.Contains(text, "will retry") {
					t.Errorf("blocked recovery misreported on start %d: %s", start, encoded)
				}
				if !reflect.DeepEqual(before, retirementTestSnapshot(t, game)) {
					t.Fatal("invalid intent changed game or journal bytes")
				}
			}
		})
	}
}

func retirementTestJSON(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
