package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// addonMissingDirs returns the folders between game (exclusive) and dir
// (inclusive) that do not exist yet, outermost first. game must be a plain
// directory and every existing folder on the way a plain directory, never a
// link: a fresh client without Interface\AddOns is valid, a linked one is not.
func addonMissingDirs(game, dir string) ([]string, error) {
	game, dir = filepath.Clean(game), filepath.Clean(dir)
	if err := addonPlainDirectory(game); err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(game, dir)
	if err != nil || rel == "." || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("folder is outside the game folder: %s", dir)
	}
	var missing []string
	for p := dir; p != game; p = filepath.Dir(p) {
		_, err := os.Lstat(p)
		if os.IsNotExist(err) {
			missing = append([]string{p}, missing...)
			continue
		}
		if err != nil {
			return nil, err
		}
		if err := addonPlainDirectory(p); err != nil {
			return nil, err
		}
	}
	return missing, nil
}

// addonCreateDirs creates dirs (outermost first) with plain Mkdir, never
// following or replacing anything that appeared meanwhile, and returns what it
// created so a failed install can remove exactly those again.
func addonCreateDirs(dirs []string) ([]string, error) {
	created := []string{}
	for _, p := range dirs {
		if err := addonPlainDirectory(filepath.Dir(p)); err != nil {
			return created, err
		}
		if err := os.Mkdir(p, 0755); err != nil {
			return created, err
		}
		created = append(created, p)
	}
	return created, nil
}

// addonRemoveCreated removes folders this install created, innermost first.
// os.Remove only removes empty folders, so nothing a player put there is lost.
func addonRemoveCreated(created []string) {
	for i := len(created) - 1; i >= 0; i-- {
		os.Remove(created[i])
	}
}
