package raidrules

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
)

// Compare the complete embedded inventory (not merely the files cfg reads) to
// the canonical JSON files. CI must run this before any generate step.
func raidRuleBytes(fsys fs.FS, root string, jsonOnly bool) (map[string]string, error) {
	out := map[string]string{}
	err := fs.WalkDir(fsys, root, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if jsonOnly && (path.Dir(name) != root || path.Ext(name) != ".json") {
			return nil
		}
		b, err := fs.ReadFile(fsys, name)
		if err != nil {
			return err
		}
		out[strings.TrimPrefix(name, root+"/")] = string(b)
		return nil
	})
	return out, err
}

func TestRaidEmbeddedBytes(t *testing.T) {
	want, err := raidRuleBytes(os.DirFS("../.."), "rules/raid", true)
	if err != nil {
		t.Fatal(err)
	}
	got, err := raidRuleBytes(embedded, "raid_rules", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(want) != 9 || !reflect.DeepEqual(got, want) {
		t.Fatal("embedded raid rules differ (bytes, missing or extra files); run go generate in desktop/raidrules")
	}
}

func TestRaidConfigCachedSnapshot(t *testing.T) {
	files := fstest.MapFS{}
	data, err := raidRuleBytes(embedded, "raid_rules", false)
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range data {
		files["raid_rules/"+name] = &fstest.MapFile{Data: []byte(b)}
	}
	load := cached(files)
	first, err := load()
	if err != nil {
		t.Fatal(err)
	}
	want := first.Scoring.Fights.TailMs
	// Caller edits cannot change the cached rules. Nor can the source change
	// the first validated config halfway through a process lifetime.
	first.Realms.Labels[0] = "Caller edit"
	files["raid_rules/scoring.json"].Data = bytes.Replace(files["raid_rules/scoring.json"].Data, []byte(`"tail_ms": 2500`), []byte(`"tail_ms": 2501`), 1)
	var wg sync.WaitGroup
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			next, err := load()
			if err != nil || next.Scoring.Fights.TailMs != want || next.Realms.Labels[0] != "PvP-EU" {
				t.Errorf("cached snapshot changed: %v", err)
			}
			next.Realms.Labels[0] = "Another edit"
		}()
	}
	wg.Wait()
}

func TestRaidConfigRetainsError(t *testing.T) {
	files := fstest.MapFS{"raid_rules/scoring.json": &fstest.MapFile{Data: []byte("invalid")}}
	load := cached(files)
	first, err := load()
	if err == nil || !reflect.DeepEqual(first, reflect.Zero(reflect.TypeOf(first)).Interface()) {
		t.Fatal("invalid config admitted")
	}
	want := err
	data, err := raidRuleBytes(embedded, "raid_rules", false)
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range data {
		files["raid_rules/"+name] = &fstest.MapFile{Data: []byte(b)}
	}
	_, err = load()
	if err != want {
		t.Fatalf("initial error was retried/lost: %v, want %v", err, want)
	}
	_, err = Config()
	if err != nil {
		t.Fatal(fmt.Errorf("shipped config: %w", err))
	}
}
