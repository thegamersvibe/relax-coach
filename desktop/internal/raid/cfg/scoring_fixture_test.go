package cfg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

// Scoring and schema boundary cases still exercise the real nine-file Load path, without
// repeatedly decoding every unrelated profile/support row under the race tool.
// Cache only immutable bytes; each case gets its own writable directory.
var scoringFixture = sync.OnceValues(func() (map[string][]byte, error) {
	full, err := cfg.Load(seedDir)
	if err != nil {
		return nil, err
	}
	c := full.Snapshot()
	p := c.Specs.Profiles[0]
	c.Specs.Profiles = []cfg.Profile{p}
	for _, id := range c.SpecIDs.Classes[p.ClassName] {
		if id.ID == p.Spec {
			c.SpecIDs.Classes = map[string][]cfg.SpecID{p.ClassName: {id}}
			break
		}
	}
	needed := map[string]bool{}
	for _, id := range p.Support {
		needed[id] = true
	}
	effects := []cfg.SupportEffect{}
	for _, e := range c.Support.Effects {
		if needed[e.ID] {
			effects = append(effects, e)
		}
	}
	c.Support.Effects = effects
	docs := map[string]any{
		"scoring.json": c.Scoring, "specs.json": c.Specs, "spec-ids.json": c.SpecIDs,
		"support.json": c.Support, "encounters-candidate.json": c.Candidates,
		"encounters-enabled.json": c.Encounters, "consumables.json": c.Consumables,
		"realm-labels.json": c.Realms, "instances.json": c.Instances,
	}
	files := map[string][]byte{}
	for name, doc := range docs {
		b, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}
		files[name] = b
	}
	return files, nil
})

func copyScoringData(t *testing.T) string {
	t.Helper()
	files, err := scoringFixture()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, name := range dataFiles {
		if err := os.WriteFile(filepath.Join(dir, name), files[name], 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestScoringFixture(t *testing.T) {
	full := load(t, seedDir).Snapshot()
	dir := copyScoringData(t)
	small := load(t, dir).Snapshot()
	if len(small.Specs.Profiles) != 1 || !reflect.DeepEqual(small.Specs.Profiles[0], full.Specs.Profiles[0]) {
		t.Fatal("compact fixture changed the retained profile")
	}
	p := full.Specs.Profiles[0]
	ids := small.SpecIDs.Classes[p.ClassName]
	if len(small.SpecIDs.Classes) != 1 || len(ids) != 1 || ids[0].ID != p.Spec {
		t.Fatal("compact fixture must retain exactly its profile's spec id")
	}
	found := false
	for _, id := range full.SpecIDs.Classes[p.ClassName] {
		if reflect.DeepEqual(ids[0], id) {
			found = true
		}
	}
	if !found {
		t.Fatal("compact fixture changed the retained spec id")
	}
	if len(small.Support.Effects) != len(p.Support) {
		t.Fatal("compact fixture must retain every referenced support effect")
	}
	for i, id := range p.Support {
		matched := false
		for _, e := range full.Support.Effects {
			for _, retained := range small.Support.Effects {
				if retained.ID == id && reflect.DeepEqual(retained, e) {
					matched = true
				}
			}
		}
		if !matched {
			t.Fatalf("compact fixture changed support reference %d: %s", i, id)
		}
	}
	// After accounting for only these three trimmed collections, all other
	// metadata, scoring values and catalogue contents must be identical.
	full.Specs.Profiles = small.Specs.Profiles
	full.SpecIDs.Classes = small.SpecIDs.Classes
	full.Support.Effects = small.Support.Effects
	if !reflect.DeepEqual(full, small) {
		t.Fatal("compact fixture changed unrelated configuration")
	}
	change(t, dir, "scoring.json", func(d map[string]any) { d["gcd_ms"] = 7 })
	if got := load(t, copyScoringData(t)); *got.Scoring.GCDMs != *full.Scoring.GCDMs {
		t.Fatal("a test edit contaminated cached fixture bytes")
	}
}
