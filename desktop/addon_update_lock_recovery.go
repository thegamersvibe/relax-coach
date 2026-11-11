package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// addonLockStaleAfter is longer than the whole apply timeout (5 minutes), so a
// lock this old cannot belong to an install that is still running.
const addonLockStaleAfter = 15 * time.Minute

// addonRecoverStaleLock lets a later Install/Update continue after the app was
// killed in the middle of a bundle install (crash, power loss), which leaves
// RelaxCoachBackups/.addon-update.lock behind. It acts only on an old regular
// lock file whose transaction is a ForeverCoach-* folder directly under
// RelaxCoachBackups (or that names nothing because the crash came before it
// was written):
//   - a folder that was moved aside (previous/<root>, or ForeverCoach for
//     legacy Coach-only updates) goes back where its target is missing;
//   - where the target exists, both it and the moved-aside copy are kept.
//
// Nothing is ever deleted: the transaction folder stays for inspection. The
// lock is removed only when every moved-aside folder is safe; otherwise it is
// kept and the next install keeps reporting the lock.
// It reports whether a stale lock was found (whether or not it could be
// cleared), i.e. whether an earlier install was interrupted.
func (u *addonUpdater) addonRecoverStaleLock(game string) (stale bool) {
	if !addonStaleBundleLock(game) {
		return false
	}
	backups := filepath.Join(game, "RelaxCoachBackups")
	lockPath := filepath.Join(backups, ".addon-update.lock")
	info, err := os.Lstat(lockPath)
	if err != nil {
		return false
	}
	stale = true
	b, err := addonReadSmallFile(lockPath)
	if err != nil {
		return stale
	}
	// Remove only this very lock (same file, same bytes): another instance may
	// have cleared it and taken a fresh one meanwhile.
	removeLock := func() {
		if addonCheckLock(lockPath, info, string(b)) == nil && os.Remove(lockPath) == nil {
			addonForgetLock(lockPath)
		}
	}
	if name := strings.TrimSpace(string(b)); name != "" {
		transaction := filepath.Clean(name)
		if filepath.Dir(transaction) != filepath.Clean(backups) || !strings.HasPrefix(filepath.Base(transaction), "ForeverCoach-") {
			return stale
		}
		if _, err := os.Lstat(transaction); err == nil {
			if addonPlainDirectory(transaction) != nil {
				return stale
			}
			pairs := [][2]string{{filepath.Join(transaction, "ForeverCoach"), filepath.Join(game, "Interface", "AddOns", "ForeverCoach")}}
			for _, root := range addonGuildRootsFor(addonBundleMinimumVersion) {
				rel := filepath.FromSlash(root)
				pairs = append(pairs, [2]string{filepath.Join(transaction, "previous", rel), filepath.Join(game, rel)})
			}
			for _, p := range pairs {
				moved, target := p[0], p[1]
				if _, err := os.Lstat(moved); os.IsNotExist(err) {
					continue
				}
				if addonPlainDirectory(moved) != nil {
					return stale
				}
				if _, err := os.Lstat(target); err == nil {
					continue // the target is there: keep both
				} else if !os.IsNotExist(err) {
					return stale
				}
				parent := filepath.Dir(target)
				if _, err := os.Lstat(parent); os.IsNotExist(err) {
					if addonPlainDirectory(filepath.Dir(parent)) != nil || os.Mkdir(parent, 0755) != nil {
						return stale
					}
				}
				if addonPlainDirectory(parent) != nil || u.rename(moved, target) != nil {
					return stale
				}
			}
		} else if !os.IsNotExist(err) {
			return stale
		}
	}
	removeLock()
	return stale
}

// addonStaleBundleLock reports a bundle lock left by an interrupted install:
// an old regular file in a plain RelaxCoachBackups folder, or the very lock
// this app process left after a refused rollback (addonAbandonLock).
func addonStaleBundleLock(game string) bool {
	if game == "" || !filepath.IsAbs(game) {
		return false
	}
	backups := filepath.Join(filepath.Clean(game), "RelaxCoachBackups")
	if addonPlainDirectory(backups) != nil {
		return false
	}
	lockPath := filepath.Join(backups, ".addon-update.lock")
	info, err := os.Lstat(lockPath)
	return err == nil && info.Mode().IsRegular() && (time.Since(info.ModTime()) >= addonLockStaleAfter || addonAbandonedLock(lockPath))
}

// addonRepairNeeded: Coach is installed but the bundle is incomplete (a
// missing or empty folder), Journey was replaced by another updater, or an
// earlier install was interrupted (stale lock).
func addonRepairNeeded(game string) bool {
	s := InstalledBundleState(game)
	return (!s.Install && (s.State == "partial" || s.State == "replaced")) || addonStaleBundleLock(game)
}
