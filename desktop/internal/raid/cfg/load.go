package cfg

import (
	"fmt"
	"io/fs"
	"os"
)

// Load reads all files from a rules/raid directory on every call. Nothing is
// cached or written. Callers keep their previous snapshot on error.
func Load(dir string) (*Config, error) {
	return LoadFS(os.DirFS(dir))
}

// LoadFS reads the rule files at the root of files on every call, applying the
// same decoding, validation and snapshot ownership as Load. Errors never return
// a partial config. The caller owns the filesystem and any caching policy.
func LoadFS(fsys fs.FS) (*Config, error) {
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
		b, err := fs.ReadFile(fsys, f.name)
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
	view := c.clone()
	view.snapshot = c
	return &view, nil
}
