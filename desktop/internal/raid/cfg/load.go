package cfg

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Load reads all files from a rules/raid directory on every call. Nothing is
// cached or written. Callers keep their previous snapshot on error.
func Load(dir string) (*Config, error) {
	// Match the lexical filepath.Join used before LoadFS, including an empty
	// directory (cwd), missing/.. and symlink/.. components.
	dir = filepath.Clean(dir)
	c, err := LoadFS(os.DirFS(dir))
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		// DirFS makes PathError paths relative; retain Load's original diagnostic
		// and wrapped OS error without changing LoadFS's filesystem-relative API.
		full := *pathErr
		full.Path = filepath.Join(dir, pathErr.Path)
		return nil, fmt.Errorf("%s: %w", pathErr.Path, &full)
	}
	return c, err
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
