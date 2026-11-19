package cfg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const seedDir = "../../../../rules/raid"

var dataFiles = []string{
	"scoring.json", "specs.json", "spec-ids.json", "support.json",
	"encounters-candidate.json", "encounters-enabled.json", "consumables.json",
	"realm-labels.json", "instances.json",
}

func TestSeedFiles(t *testing.T) {
	for _, name := range dataFiles {
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(seedDir, name))
			if err != nil {
				t.Fatal(err)
			}
			var file struct {
				Schema  int    `json:"schema"`
				Version string `json:"version"`
				Source  string `json:"source"`
			}
			if err := json.Unmarshal(b, &file); err != nil {
				t.Fatal(err)
			}
			if file.Schema != 1 || file.Version == "" || file.Source == "" {
				t.Fatalf("missing version/provenance: %+v", file)
			}
		})
	}
}
