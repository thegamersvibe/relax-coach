package cfg

import (
	"fmt"
	"os"
	"path/filepath"
)

// Load reads all files from a rules/raid directory on every call. Nothing is
// embedded, cached or written. Callers keep their previous snapshot on error.
func Load(dir string) (*Config, error) {
	c := new(Config)
	files := []struct {
		name  string
		value any
	}{
		{"scoring.json", &c.Scoring},
		{"specs.json", &c.Specs},
		{"spec-ids.json", &c.SpecIDs},
		{"support.json", &c.Support},
		{"encounters-candidate.json", &c.Candidates},
		{"encounters-enabled.json", &c.Encounters},
		{"consumables.json", &c.Consumables},
		{"realm-labels.json", &c.Realms},
		{"instances.json", &c.Instances},
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, f.name))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}
		if err := decode(b, f.value); err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return c, nil
}
