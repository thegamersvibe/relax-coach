package cfg_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
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

// Seeded boss-unit override (architect, from both real Stratholme logs): Balnazzar's boss unit is NPC 10812
// on every line; 10813 is a guard from Classic data. The other Stratholme encounters need none.
func TestSeedBalnazzarBossUnits(t *testing.T) {
	c, err := cfg.Load(seedDir)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := c.Encounter(478)
	if !ok || e.Name != "Balnazzar" || len(e.BossUnits) != 2 || e.BossUnits[0].ID != 10812 || e.BossUnits[1].ID != 10813 {
		t.Fatalf("encounter 478: %+v %v", e, ok)
	}
	if got := c.OutputTargets(478); got.Mode != "boss_only" {
		t.Errorf("output stays boss-only: %+v", got)
	}
	if _, ok := c.Encounter(479); ok {
		t.Error("no other encounter is seeded")
	}
}
