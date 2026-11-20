package cfg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

// Validation cases edit the first records, not the seed's catalogue sizes.
// Keep all three Warrior profiles and their matching ids/support, plus two
// candidates for duplicate tests. Full seed, reload and ownership tests still
// use the complete directory; every validation assertion remains unchanged.
// Only immutable fixture BYTES are shared. Every case writes its own directory
// and calls the real Load; neither decoded Configs nor loader results are cached.
var validationSample = sync.OnceValues(func() (map[string][]byte, error) {
	files := map[string][]byte{}
	for _, name := range dataFiles {
		b, err := os.ReadFile(filepath.Join(seedDir, name))
		if err != nil {
			return nil, err
		}
		switch name {
		case "specs.json", "spec-ids.json", "support.json", "encounters-candidate.json":
			var d map[string]any
			if err := json.Unmarshal(b, &d); err != nil {
				return nil, err
			}
			switch name {
			case "specs.json":
				d["profiles"] = d["profiles"].([]any)[:3]
			case "spec-ids.json":
				d["classes"] = map[string]any{"WARRIOR": d["classes"].(map[string]any)["WARRIOR"]}
			case "support.json":
				d["effects"] = d["effects"].([]any)[:4]
			case "encounters-candidate.json":
				d["encounters"] = d["encounters"].([]any)[:2]
			}
			b, err = json.Marshal(d)
			if err != nil {
				return nil, err
			}
		}
		files[name] = b
	}
	return files, nil
})

func copyValidationData(t *testing.T) string {
	t.Helper()
	files, err := validationSample()
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

func TestRound2ValidationSample(t *testing.T) {
	full := load(t, seedDir)
	small := load(t, copyValidationData(t))
	if len(small.Specs.Profiles) != 3 || len(small.SpecIDs.Classes) != 1 || len(small.Support.Effects) != 4 || len(small.Candidates.Encounters) != 2 {
		t.Fatalf("validation fixture still repeats the full catalogues: profiles=%d classes=%d effects=%d candidates=%d",
			len(small.Specs.Profiles), len(small.SpecIDs.Classes), len(small.Support.Effects), len(small.Candidates.Encounters))
	}
	if !reflect.DeepEqual(full.Specs.Profiles[:3], small.Specs.Profiles) ||
		!reflect.DeepEqual(full.SpecIDs.Classes["WARRIOR"], small.SpecIDs.Classes["WARRIOR"]) ||
		!reflect.DeepEqual(full.Support.Effects[:4], small.Support.Effects) ||
		!reflect.DeepEqual(full.Candidates.Encounters[:2], small.Candidates.Encounters) {
		t.Fatal("sampling changed a retained record")
	}
	if !reflect.DeepEqual(full.Scoring, small.Scoring) || !reflect.DeepEqual(full.Encounters, small.Encounters) ||
		!reflect.DeepEqual(full.Consumables, small.Consumables) || !reflect.DeepEqual(full.Realms, small.Realms) ||
		!reflect.DeepEqual(full.Instances, small.Instances) {
		t.Fatal("sampling changed an unrelated catalogue")
	}
}
