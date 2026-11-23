package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// These stems are exactly the SavedVariables and SavedVariablesPerCharacter
// declarations in the four repository TOCs; they are not prefix patterns.
var retirementSaveNames = []string{"ForeverCoachDB", "RelaxRaidDB", "RelaxRaidCharDB", "RelaxQuestPlannerDB"}
var retirementJourneySaveNames = []string{"Journey_Account", "Journey_Character"}

func retirementDirectory(path string, result *retirementResult) bool {
	if err := appPlainDirectory(path); err != nil {
		if !os.IsNotExist(err) {
			result.skip(path, err)
		}
		return false
	}
	return true
}

// Enumerate one level only, refusing links/reparse points before descent. The
// callers explicitly describe Account/account/realm/character, never Walk/Glob.
func retirementDirectories(path string, result *retirementResult) []string {
	if !retirementDirectory(path, result) {
		return nil
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		result.skip(path, err)
		return nil
	}
	var dirs []string
	for _, entry := range entries {
		child := filepath.Join(path, entry.Name())
		if entry.Type()&os.ModeSymlink != 0 {
			result.skip(child, fmt.Errorf("linked directory"))
			continue
		}
		if !entry.IsDir() {
			continue
		}
		if retirementDirectory(child, result) {
			dirs = append(dirs, child)
		}
	}
	return dirs
}

func retirementSaves(ctx context.Context, game string, journey bool, observer *addonUpdater, quarantine *retirementQuarantine, result *retirementResult) {
	wtf := filepath.Join(game, "WTF")
	if !retirementDirectory(wtf, result) {
		return
	}
	names := append([]string(nil), retirementSaveNames...)
	if journey {
		names = append(names, retirementJourneySaveNames...)
	}
	for _, account := range retirementDirectories(filepath.Join(wtf, "Account"), result) {
		retirementSaveDirectory(ctx, filepath.Join(account, "SavedVariables"), names, observer, quarantine, result)
		for _, realm := range retirementDirectories(account, result) {
			if filepath.Base(realm) == "SavedVariables" {
				continue
			}
			for _, char := range retirementDirectories(realm, result) {
				retirementSaveDirectory(ctx, filepath.Join(char, "SavedVariables"), names, observer, quarantine, result)
			}
		}
	}
}

func retirementSaveDirectory(ctx context.Context, path string, names []string, observer *addonUpdater, quarantine *retirementQuarantine, result *retirementResult) {
	if result.SavedSettingsDeferred || !retirementDirectory(path, result) {
		return
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		result.skip(path, err)
		return
	}
	defer root.Close()
	for _, name := range names {
		for _, suffix := range []string{".lua", ".lua.bak"} {
			filename := name + suffix
			full := filepath.Join(path, filename)
			info, err := root.Lstat(filename)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				result.skip(full, err)
				continue
			}
			if !info.Mode().IsRegular() {
				result.skip(full, fmt.Errorf("linked or non-regular save"))
				continue
			}
			if err = appRejectReparse(full); err != nil {
				result.skip(full, err)
				continue
			}
			// Recheck immediately before each save mutation: a process launched after
			// the initial observation must stop the remainder of this cleanup.
			open, err := observer.observeGame(ctx)
			if open || err != nil {
				result.SavedSettingsDeferred = true
				return
			}
			relative, err := filepath.Rel(quarantine.game, full)
			if err != nil {
				result.skip(full, err)
				continue
			}
			movedBefore := len(result.Moved)
			quarantine.move(full, relative, result, func() bool {
				open, err := observer.observeGame(ctx)
				if open || err != nil {
					result.SavedSettingsDeferred = true
					return false
				}
				return true
			})
			if len(result.Moved) == movedBefore && (name == "Journey_Account" || name == "Journey_Character") {
				// The TOC may already have moved. Keep ownership only while a known
				// Journey save is still pending, so a failed rename can retry next start.
				result.journeyRetry = true
			}
		}
	}
}
