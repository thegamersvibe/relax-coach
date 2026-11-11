package main

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// A lock this app process had to leave behind because Windows refused a
// rollback move (a file in use) belongs to no running install: the install
// that owned it has returned. Remember it (same file, same bytes), so the
// player can close WoW and click Repair/Update right away in the same session
// instead of waiting for the 15-minute stale-lock rule. After a restart of the
// app the 15-minute rule applies again. Recovery itself is unchanged: moved-
// aside folders go back where the target is missing, nothing is deleted.
var addonAbandoned = struct {
	sync.Mutex
	locks map[string]addonAbandonedEntry
}{locks: map[string]addonAbandonedEntry{}}

type addonAbandonedEntry struct {
	info     os.FileInfo
	contents string
}

func addonAbandonLock(lockPath string, info os.FileInfo, contents string) {
	addonAbandoned.Lock()
	defer addonAbandoned.Unlock()
	addonAbandoned.locks[filepath.Clean(lockPath)] = addonAbandonedEntry{info, contents}
}

// addonAbandonedLock: lockPath is still the very lock this process left.
func addonAbandonedLock(lockPath string) bool {
	addonAbandoned.Lock()
	entry, ok := addonAbandoned.locks[filepath.Clean(lockPath)]
	addonAbandoned.Unlock()
	return ok && addonCheckLock(lockPath, entry.info, entry.contents) == nil
}

func addonForgetLock(lockPath string) {
	addonAbandoned.Lock()
	defer addonAbandoned.Unlock()
	delete(addonAbandoned.locks, filepath.Clean(lockPath))
}

// addonInUseDetail is the system's own text for the first in-use refusal
// (without the paths a raw rename error carries).
func addonInUseDetail(running bool, errs ...error) string {
	for _, err := range errs {
		if !addonInUse(err, running) {
			continue
		}
		var link *os.LinkError
		if errors.As(err, &link) && link.Err != nil {
			return link.Err.Error()
		}
		return "file in use"
	}
	return ""
}

// addonShortPath shows p relative to the game folder (e.g.
// RelaxCoachBackups\ForeverCoach-…\previous) instead of the full path.
func addonShortPath(game, p string) string {
	if rel, err := filepath.Rel(game, p); err == nil && rel != "" && rel != "." && !filepath.IsAbs(rel) && rel[0] != '.' {
		return rel
	}
	return filepath.Base(p)
}
