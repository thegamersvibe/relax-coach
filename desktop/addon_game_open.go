package main

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
)

// Installing and updating no longer require WoW to be closed. WoW reads addon
// files when it loads them, so the folders can be swapped while it runs; the
// player is then told what loads the new files:
//   - every root existed and no new file path came: /reload is enough;
//   - a new root folder or a new file path: restart WoW. This is the safe
//     side, not a known limit: since 9.0.1 /reload is documented to pick up
//     TOC changes and new files under Interface/AddOns (Warcraft Wiki), but
//     that is unverified on the Forever client and does not cover the guide
//     content in Interface/Content.
// The only case that still asks to close WoW is Windows refusing a move
// because a file is in use; the transaction is then rolled back as for any
// other failure.

const (
	addonAdviceReload  = "Done. Type /reload in game."
	addonAdviceRestart = "Done. Exit WoW completely and start it again so the new files load."
	addonAdviceStart   = "Done. Start WoW to load it."
	addonCloseWoW      = "Close WoW and try again: Windows refused to move a file that is in use."
)

// observeGame asks whether WoW is running, only to choose the advice and to
// read an "access denied" move correctly. It never refuses: an unknown answer
// (no detector, detection error) counts as running. It keeps the cancel check
// the old refusal had.
func (u *addonUpdater) observeGame(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if u.running == nil {
		return true, nil
	}
	running, err := u.running(ctx)
	if err != nil {
		running = true
	}
	return running, ctx.Err()
}

// addonHasNewPaths reports whether want (slash paths relative to root) has a
// file path that root does not have yet. Windows paths ignore case, so names
// are compared case-insensitively. Any error counts as new (restart advice).
func addonHasNewPaths(root string, want map[string]addonFileDigest) (bool, error) {
	have := map[string]bool{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			have[strings.ToLower(filepath.ToSlash(rel))] = true
		}
		return nil
	})
	if err != nil {
		return true, err
	}
	for name := range want {
		if !have[strings.ToLower(name)] {
			return true, nil
		}
	}
	return false, nil
}

// addonFilesUnder lists the regular files under root as slash paths.
func addonFilesUnder(root string) (map[string]addonFileDigest, error) {
	files := map[string]addonFileDigest{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			files[filepath.ToSlash(rel)] = addonFileDigest{}
		}
		return nil
	})
	return files, err
}

func addonAdvice(restart, running bool) string {
	switch {
	case !running:
		return addonAdviceStart
	case restart:
		return addonAdviceRestart
	default:
		return addonAdviceReload
	}
}

// addonInUse: the error is a move refused because a file is in use (see
// addonErrnoInUse per platform).
func addonInUse(err error, running bool) bool {
	return err != nil && addonErrnoInUse(err, running)
}
