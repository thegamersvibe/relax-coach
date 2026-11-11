package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// File.Stat captures an identity now (Lstat identity can be lazy on Windows).
func addonDirectoryIdentity(p string) (os.FileInfo, error) {
	info, err := os.Lstat(p)
	if os.IsNotExist(err) {
		// Missing leaves (and an optional missing Content parent) are valid,
		// but must not hide a linked or invalid existing ancestor.
		if parent := filepath.Dir(p); parent != p {
			if _, err := addonDirectoryIdentity(parent); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := addonPlainDirectory(p); err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	owned, err := f.Stat()
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return nil, errors.Join(err, closeErr)
	}
	if !os.SameFile(info, owned) {
		return nil, fmt.Errorf("update directory changed: %s", p)
	}
	return owned, nil
}
func addonSameDirectory(p string, want os.FileInfo) error {
	now, err := addonDirectoryIdentity(p)
	if err != nil {
		return err
	}
	if (want == nil) != (now == nil) || (want != nil && !os.SameFile(want, now)) {
		return fmt.Errorf("update directory changed: %s", p)
	}
	return nil
}

type addonTreeSwap struct {
	relative, target, stage, backup, failed string
	original, staged                        os.FileInfo
	want                                    map[string]addonFileDigest
	backed, installed                       bool
}

// The existing updater transaction covers either its legacy single tree or the
// three fixed Guild trees. It never delegates installation to bundled scripts.
func addonSnapshot(game string, version string) ([]*addonTreeSwap, map[string]os.FileInfo, error) {
	roots := []string{"Interface/AddOns/ForeverCoach"}
	if addonGuildRelease(version) {
		roots = addonGuildRootsFor(version)
	}
	ancestors := map[string]os.FileInfo{}
	trees := []*addonTreeSwap{}
	for _, relative := range roots {
		target := filepath.Join(game, filepath.FromSlash(relative))
		original, err := addonDirectoryIdentity(target)
		if err != nil {
			return nil, nil, err
		}
		trees = append(trees, &addonTreeSwap{relative: relative, target: target, original: original})
		for p := filepath.Dir(target); ; p = filepath.Dir(p) {
			if _, ok := ancestors[p]; ok {
				break
			}
			info, err := addonDirectoryIdentity(p)
			if err != nil {
				return nil, nil, err
			}
			ancestors[p] = info
			if filepath.Dir(p) == p {
				break
			}
		}
	}
	return trees, ancestors, nil
}
func addonCheckAncestors(ancestors map[string]os.FileInfo) error {
	for p, info := range ancestors {
		if err := addonSameDirectory(p, info); err != nil {
			return err
		}
	}
	return nil
}
func addonCheckTargets(trees []*addonTreeSwap) error {
	for _, tree := range trees {
		// A published backup remains part of the transaction state until its
		// captured original has been restored, not just until install succeeds.
		if tree.backed {
			if err := addonSameDirectory(tree.backup, tree.original); err != nil {
				return fmt.Errorf("retained original backup changed: %w", err)
			}
		}
		expected := tree.original
		if tree.installed {
			expected = tree.staged
		} else if tree.backed {
			expected = nil
		}
		if err := addonSameDirectory(tree.target, expected); err != nil {
			return err
		}
	}
	return nil
}

func addonCheckLock(path string, owned os.FileInfo, contents string) error {
	if err := addonPlainDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || !os.SameFile(owned, info) {
		return errors.New("addon backup lock identity changed")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(owned, info) {
		return errors.New("addon backup lock changed while opening")
	}
	b, err := io.ReadAll(io.LimitReader(f, int64(len(contents))+1))
	if err != nil {
		return err
	}
	if string(b) != contents {
		return errors.New("addon backup lock ownership bytes changed")
	}
	info, err = os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || !os.SameFile(owned, info) {
		return errors.New("addon backup lock changed while reading")
	}
	return nil
}

func (u *addonUpdater) installTrees(ctx context.Context, game string, release AddonReleaseInfo, payload []byte, before addonInstalledState, trees []*addonTreeSwap, ancestors map[string]os.FileInfo) (result AddonUpdateResult, err error) {
	if err := addonCheckAncestors(ancestors); err != nil {
		return result, err
	}
	if err := addonCheckTargets(trees); err != nil {
		return result, err
	}
	backups := filepath.Join(game, "RelaxCoachBackups")
	if err := os.Mkdir(backups, 0700); err != nil && !os.IsExist(err) {
		return result, err
	}
	backupIdentity, err := addonDirectoryIdentity(backups)
	if err != nil {
		return result, err
	}
	ancestors[backups] = backupIdentity
	lockPath := filepath.Join(backups, ".addon-update.lock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		return result, errors.New("another addon update or an unfinished recovery holds the backup lock; close other helpers and inspect RelaxCoachBackups before retrying")
	}
	if err != nil {
		return result, err
	}
	lockIdentity, err := lock.Stat()
	if err != nil {
		lock.Close()
		return result, err
	}
	keepLock := false
	lockContents := ""
	defer func() {
		lock.Close()
		if !keepLock {
			if lockContents != "" {
				if e := addonCheckLock(lockPath, lockIdentity, lockContents); e != nil {
					result = AddonUpdateResult{}
					err = errors.Join(err, fmt.Errorf("backup lock cleanup refused; recovery location %s: %w", strings.TrimSpace(lockContents), e))
					return
				}
			}
			if e := appRemoveOwned(lockPath, lockIdentity); e != nil {
				result = AddonUpdateResult{}
				err = errors.Join(err, e)
			}
		}
	}()
	transaction, err := os.MkdirTemp(backups, "ForeverCoach-"+time.Now().UTC().Format("20060102T150405.000000000Z")+"-")
	if err != nil {
		return result, err
	}
	transactionIdentity, err := addonDirectoryIdentity(transaction)
	if err != nil {
		return result, err
	}
	ancestors[transaction] = transactionIdentity
	keepTransaction := false
	// Both forward commit and cancellation-independent recovery require the
	// original lock inode/file ID AND its exact transaction ownership bytes.
	checkTransaction := func() error {
		if e := addonCheckAncestors(ancestors); e != nil {
			return e
		}
		if lockContents != "" {
			if e := addonCheckLock(lockPath, lockIdentity, lockContents); e != nil {
				keepTransaction, keepLock = true, true
				return fmt.Errorf("addon backup lock ownership lost; recovery location: %s: %w", transaction, e)
			}
		}
		return nil
	}
	defer func() {
		if !keepTransaction {
			if e := checkTransaction(); e == nil {
				err = errors.Join(err, os.RemoveAll(transaction))
			} else {
				result = AddonUpdateResult{}
				keepLock = true
				err = errors.Join(err, fmt.Errorf("cleanup refused; recovery location %s: %w", transaction, e))
			}
		}
	}()
	if _, err = lock.WriteString(transaction + "\n"); err == nil {
		err = lock.Sync()
	}
	if err != nil {
		return result, err
	}
	if err = lock.Close(); err != nil {
		return result, err
	}
	lockContents = transaction + "\n"
	stage := filepath.Join(transaction, "staged")
	digests, err := u.extract(ctx, payload, stage, release.Version)
	if err != nil {
		return result, err
	}
	guild := addonGuildRelease(release.Version)
	backupRoot := filepath.Join(transaction, "previous")
	for _, tree := range trees {
		tree.stage, tree.backup, tree.failed = stage, filepath.Join(transaction, "ForeverCoach"), filepath.Join(transaction, "failed")
		tree.want = digests
		if guild {
			tree.stage = filepath.Join(stage, filepath.FromSlash(tree.relative))
			tree.backup = filepath.Join(backupRoot, filepath.FromSlash(tree.relative))
			tree.failed = filepath.Join(transaction, "failed", filepath.FromSlash(tree.relative))
			tree.want = map[string]addonFileDigest{}
			for name, digest := range digests {
				if strings.HasPrefix(name, tree.relative+"/") {
					tree.want[strings.TrimPrefix(name, tree.relative+"/")] = digest
				}
			}
		}
		if err := verifyAddonTree(ctx, tree.stage, tree.want); err != nil {
			return result, err
		}
		tree.staged, err = addonDirectoryIdentity(tree.stage)
		if err != nil {
			return result, err
		}
		if err := os.MkdirAll(filepath.Dir(tree.backup), 0700); err != nil {
			return result, err
		}
		if err := os.MkdirAll(filepath.Dir(tree.failed), 0700); err != nil {
			return result, err
		}
		for _, child := range []string{tree.stage, tree.backup, tree.failed} {
			for parent := filepath.Dir(child); parent != transaction; parent = filepath.Dir(parent) {
				identity, e := addonDirectoryIdentity(parent)
				if e != nil {
					return result, e
				}
				if previous, ok := ancestors[parent]; ok {
					if e := addonSameDirectory(parent, previous); e != nil {
						return result, e
					}
				} else {
					ancestors[parent] = identity
				}
			}
		}
	}
	// WoW may run: this only decides the advice and how a refused move reads.
	running, err := u.observeGame(ctx)
	if err != nil {
		return result, err
	}
	if err := checkTransaction(); err != nil {
		return result, err
	}
	if err := addonCheckTargets(trees); err != nil {
		return result, err
	}
	now, err := addonInstalled(trees[0].target)
	if err != nil {
		return result, err
	}
	remote, _ := parseAddonVersion(release.Version)
	sameForRepair := release.Repair && !remote.newerThan(now.version) && !now.version.newerThan(remote)
	if !remote.newerThan(now.version) && !sameForRepair {
		return result, errors.New("installed addon is already this version or newer; refusing to reinstall or downgrade")
	}
	if before.exists != now.exists || (before.exists && (!os.SameFile(before.info, now.info) || !bytes.Equal(before.toc, now.toc))) {
		return result, errors.New("the installed addon changed during download; check again")
	}
	// A missing Content parent is valid, and so are a fresh client's missing
	// Interface and Interface\AddOns. Create only those fixed parents inside the
	// game folder, outermost first, after all downloaded bytes have passed
	// verification.
	createdParents := []string{}
	for _, tree := range trees {
		missing := []string{}
		for p := filepath.Dir(tree.target); p != game && ancestors[p] == nil; p = filepath.Dir(p) {
			if _, known := ancestors[p]; !known || filepath.Dir(p) == p {
				return result, fmt.Errorf("unexpected addon folder: %s", p)
			}
			missing = append([]string{p}, missing...)
		}
		for _, parent := range missing {
			err := checkTransaction()
			if err == nil {
				err = addonMkdir(parent, 0755)
			}
			var info os.FileInfo
			if err == nil {
				// Never adopt a populated folder swapped in after Mkdir.
				if info, err = addonNewDirIdentity(parent); err == nil {
					ancestors[parent] = info
					createdParents = append(createdParents, parent)
				}
			}
			if err != nil {
				// Nothing was moved yet: remove only the parents this install
				// created, innermost first, while the transaction still holds
				// (os.Remove never removes a folder that is not empty).
				for i := len(createdParents) - 1; i >= 0; i-- {
					if checkTransaction() != nil || os.Remove(createdParents[i]) != nil {
						break
					}
					ancestors[createdParents[i]] = nil
				}
				return result, err
			}
		}
	}
	rollback := func(cause error) (AddonUpdateResult, error) {
		var failures []error
		// Recovery is deliberately not governed by the canceled request context.
		for i := len(trees) - 1; i >= 0; i-- {
			tree := trees[i]
			if !tree.backed && !tree.installed {
				continue
			}
			if e := checkTransaction(); e != nil {
				failures = append(failures, e)
				continue
			}
			// A bad backup must not trigger further moves of this tree, but
			// must not prevent restoration of independently safe trees either.
			if e := addonCheckTargets([]*addonTreeSwap{tree}); e != nil {
				failures = append(failures, e)
				continue
			}
			if tree.installed {
				if e := addonSameDirectory(tree.target, tree.staged); e != nil {
					failures = append(failures, e)
					continue
				}
				if e := addonPlainDirectory(filepath.Dir(tree.failed)); e != nil {
					failures = append(failures, e)
					continue
				}
				if e := u.rename(tree.target, tree.failed); e != nil {
					failures = append(failures, e)
					continue
				}
				tree.installed = false
			}
			// Quarantine is a completed operation: neither side of the next
			// rename may rely on the ancestor snapshot checked before it.
			if e := checkTransaction(); e != nil {
				failures = append(failures, e)
				continue
			}
			if tree.backed {
				if e := addonCheckTargets([]*addonTreeSwap{tree}); e != nil {
					failures = append(failures, e)
					continue
				}
				if e := u.rename(tree.backup, tree.target); e != nil {
					failures = append(failures, e)
					continue
				}
				tree.backed = false
				if e := addonSameDirectory(tree.target, tree.original); e != nil {
					failures = append(failures, e)
				}
			}
		}
		if len(failures) == 0 {
			for i := len(createdParents) - 1; i >= 0; i-- {
				parent := createdParents[i]
				if e := checkTransaction(); e != nil {
					failures = append(failures, e)
					continue
				}
				if e := os.Remove(parent); e != nil {
					failures = append(failures, e)
				} else {
					ancestors[parent] = nil
				}
			}
		}
		if e := checkTransaction(); e != nil {
			failures = append(failures, e)
		}
		if e := addonCheckTargets(trees); e != nil {
			failures = append(failures, e)
		}
		if len(failures) > 0 {
			keepTransaction, keepLock = true, true
			originals := []string{}
			for _, tree := range trees {
				if tree.backed {
					originals = append(originals, tree.backup)
				}
			}
			if refusals := append([]error{cause}, failures...); addonInUse(errors.Join(refusals...), running) {
				// A file in use (WoW) also blocked the way back. Lead with what
				// to do; the lock may be recovered right away in this session.
				addonAbandonLock(lockPath, lockIdentity, lockContents)
				where := "the previous folders are safe in the backup at " + addonShortPath(game, filepath.Join(transaction, "previous"))
				if len(originals) == 0 {
					where = "no earlier Relax Coach folders were there; the partial install is kept at " + addonShortPath(game, transaction)
				}
				// %v-free: no wrapped cause, so a refused "access denied" is not
				// reworded as a missing write permission.
				return AddonUpdateResult{}, fmt.Errorf("Close WoW, then click Check and Repair (or Update). Windows refused to move a file that is in use, so the install could not be fully undone; %s. (%s)", where, addonInUseDetail(running, refusals...))
			}
			return AddonUpdateResult{}, fmt.Errorf("addon install failed (%w); automatic rollback failed (%v); recovery location: %s (backup lock cleanup withheld); original backups: %s", cause, errors.Join(failures...), transaction, strings.Join(originals, ", "))
		}
		keepTransaction = false
		if addonInUse(cause, running) {
			// %v, not %w: an in-use "access denied" must not read as a
			// missing write permission.
			return AddonUpdateResult{}, fmt.Errorf("%s The previous addon folders were restored; nothing was changed. (%v)", addonCloseWoW, cause)
		}
		return AddonUpdateResult{}, fmt.Errorf("addon install failed; previous addon trees restored: %w", cause)
	}
	// Decided from the real file lists before anything moves: a root that is
	// not there yet or any new file path needs a game restart.
	restart := false
	for _, tree := range trees {
		if tree.original == nil {
			restart = true
		} else if fresh, _ := addonHasNewPaths(tree.target, tree.want); fresh {
			restart = true
		}
	}
	for _, tree := range trees {
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		if err := checkTransaction(); err != nil {
			return rollback(err)
		}
		if err := addonCheckTargets(trees); err != nil {
			return rollback(err)
		}
		if err := addonSameDirectory(tree.stage, tree.staged); err != nil {
			return rollback(err)
		}
		if err := verifyAddonTree(ctx, tree.stage, tree.want); err != nil {
			return rollback(err)
		}
		if tree.original != nil {
			if err := u.rename(tree.target, tree.backup); err != nil {
				return rollback(err)
			}
			tree.backed = true
			keepTransaction = true
		}
		if err := ctx.Err(); err != nil {
			return rollback(err)
		}
		if err := checkTransaction(); err != nil {
			return rollback(err)
		}
		if err := addonCheckTargets(trees); err != nil {
			return rollback(err)
		}
		if err := addonSameDirectory(tree.stage, tree.staged); err != nil {
			return rollback(err)
		}
		if err := verifyAddonTree(ctx, tree.stage, tree.want); err != nil {
			return rollback(err)
		}
		if err := u.rename(tree.stage, tree.target); err != nil {
			return rollback(err)
		}
		tree.installed = true
		if err := verifyAddonTree(ctx, tree.target, tree.want); err != nil {
			return rollback(err)
		}
	}
	if err := checkTransaction(); err != nil {
		return rollback(err)
	}
	if err := addonCheckTargets(trees); err != nil {
		return rollback(err)
	}
	// Recheck every tree: a later component must not hide changes to earlier ones.
	for _, tree := range trees {
		if err := verifyAddonTree(ctx, tree.target, tree.want); err != nil {
			return rollback(err)
		}
	}
	if err := checkTransaction(); err != nil {
		return rollback(err)
	}
	if err := addonCheckTargets(trees); err != nil {
		return rollback(err)
	}
	for _, tree := range trees {
		if tree.backed {
			result.BackupPath = tree.backup
			if guild {
				result.BackupPath = backupRoot
			}
			break
		}
	}
	result.Version, result.UpdatedFiles = release.Version, len(digests)
	result.Message = "Addon updated and every installed file verified. Journey and SavedVariables were not touched. The desktop helper was not updated."
	if guild {
		result.Message = "Coach, Journey and guide content updated together; every installed file verified. SavedVariables, settings and unrelated addons were not touched. The desktop helper was not updated."
		if addonBundleRelease(release.Version) {
			result.Message = "Coach, Journey, Relax Quest Planner and guide content updated together; every installed file verified. SavedVariables, settings and unrelated addons were not touched. The desktop helper was not updated."
		}
		if release.Repair {
			result.Message = "Relax Coach " + release.Version + " installed again to complete an interrupted or incomplete install: Coach, Journey, Relax Quest Planner and guide content, every installed file verified. The folders that were there are kept in the backup. SavedVariables, settings and unrelated addons were not touched."
		}
		if release.journeyReplaced {
			result.Message = "Relax Journey installed: Coach, Journey, Relax Quest Planner and guide content " + release.Version + ", every installed file verified. The Journey that was there is kept in the backup, not deleted. If CurseForge, Wago or Journey Guide Manager put it there, remove Journey there or turn off its auto-update, or it can happen again."
		}
		if !before.exists {
			// First install (Coach was not there): same transaction, install wording.
			result.Message = "Coach, Journey and guide content " + release.Version + " installed; every installed file verified."
			if addonBundleRelease(release.Version) {
				result.Message = "Coach, Journey, Relax Quest Planner and guide content " + release.Version + " installed; every installed file verified."
			}
			result.Message += " Enable them in the AddOns list if needed. SavedVariables, settings and unrelated addons were not touched."
			if result.BackupPath != "" {
				result.Message += " Folders that were already there are kept in the backup."
			}
		}
	}
	result.Message += " " + addonAdvice(restart, running)
	return result, nil
}
