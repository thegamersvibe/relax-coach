package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// addonMkdir creates one folder; a test seam to act between two steps.
var addonMkdir = os.Mkdir

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

// addonCreatedDir is a folder this install created, with its identity.
type addonCreatedDir struct {
	path string
	info os.FileInfo
}

// addonCreateDirs creates dirs (outermost first) with plain Mkdir. After each
// one it records the new folder's identity in ancestors and re-checks every
// recorded folder, so a parent replaced by a link in between is refused before
// anything is written through it. It returns what it created so a failed
// install can remove exactly those folders again.
func addonCreateDirs(dirs []string, ancestors map[string]os.FileInfo) ([]addonCreatedDir, error) {
	created := []addonCreatedDir{}
	for _, p := range dirs {
		if err := addonPlainDirectory(filepath.Dir(p)); err != nil {
			return created, err
		}
		if err := addonMkdir(p, 0755); err != nil {
			return created, err
		}
		info, err := addonDirectoryIdentity(p)
		if err == nil && info == nil {
			err = fmt.Errorf("created folder disappeared: %s", p)
		}
		if err != nil {
			return created, err
		}
		created = append(created, addonCreatedDir{p, info})
		ancestors[p] = info
		if err := addonCheckAncestors(ancestors); err != nil {
			return created, err
		}
	}
	return created, nil
}

// addonRemoveCreated removes folders this install created, innermost first,
// and only while each one and all its recorded parents are still the very
// folders seen before: never a replacement, never through a link. os.Remove
// only removes empty folders, so nothing a player put there is lost.
func addonRemoveCreated(created []addonCreatedDir, ancestors map[string]os.FileInfo) {
	for i := len(created) - 1; i >= 0; i-- {
		c := created[i]
		for p, info := range ancestors {
			if p != c.path && info != nil && strings.HasPrefix(c.path, p+string(filepath.Separator)) {
				if addonSameDirectory(p, info) != nil {
					return
				}
			}
		}
		if addonSameDirectory(c.path, c.info) != nil {
			return
		}
		if os.Remove(c.path) == nil {
			delete(ancestors, c.path)
		}
	}
}
