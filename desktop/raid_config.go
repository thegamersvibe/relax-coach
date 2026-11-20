package main

import (
	"embed"
	"io/fs"
	"sync"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

// The generated copy is committed; go test checks it against rules/raid before
// regeneration. One app release has one raid config, with no disk override.
//
//go:generate python3 ../scripts/gen_raid_rules.py
//go:embed all:raid_rules
var embeddedRaidRules embed.FS

// raidConfig retains the first load error as well as success. Startup primes it;
// callers get detached snapshots so page projections cannot edit the cache.
var raidConfig = cachedRaidConfig(embeddedRaidRules)

func cachedRaidConfig(files fs.FS) func() (cfg.Config, error) {
	once := sync.OnceValues(func() (*cfg.Config, error) {
		root, err := fs.Sub(files, "raid_rules")
		if err != nil {
			return nil, err
		}
		return cfg.LoadFS(root)
	})
	return func() (cfg.Config, error) {
		c, err := once()
		if err != nil {
			return cfg.Config{}, err
		}
		return c.Snapshot(), nil
	}
}
