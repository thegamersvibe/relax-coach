package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const retirementJourneyKey = "retirementJourney"

// A small per-game ownership receipt is needed because addon removal is allowed
// while WoW runs. Without it, the next start has no TOC to identify the deferred
// Journey settings. A currently foreign/unreadable Journey revokes this receipt.
// Existing application preferences are retained by writeCoachConfig.
func retirementConfigPath(create bool) error {
	p := configPath()
	if p == "" {
		return fmt.Errorf("configuration directory unavailable")
	}
	dir := filepath.Dir(p)
	if create {
		if err := appPlainDirectory(filepath.Dir(dir)); err != nil {
			return err
		}
		if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
			return err
		}
	}
	if err := appPlainDirectory(dir); err != nil {
		return err
	}
	info, err := os.Lstat(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("linked/non-regular configuration")
	}
	return appRejectReparse(p)
}

func retirementKnownJourney(game string, result *retirementResult) bool {
	if err := retirementConfigPath(false); err != nil {
		if !os.IsNotExist(err) {
			result.skip("Journey ownership receipt", err)
		}
		return false
	}
	configMu.Lock()
	defer configMu.Unlock()
	c, err := readCoachConfig()
	if err != nil {
		result.skip("Journey ownership receipt", err)
		return false
	}
	var known map[string]bool
	if raw := c[retirementJourneyKey]; len(raw) > 0 {
		if err = json.Unmarshal(raw, &known); err != nil {
			result.skip("Journey ownership receipt", err)
			return false
		}
	}
	return known[installationKey(game)]
}

func retirementRecordJourney(game string, ours bool, result *retirementResult) {
	// Do not create or rewrite config just to record a previously unknown folder.
	if !ours && !retirementKnownJourney(game, result) {
		return
	}
	err := retirementConfigPath(true)
	if err == nil {
		err = writeCoachConfig(func(c map[string]json.RawMessage) error {
			known := map[string]bool{}
			if raw := c[retirementJourneyKey]; len(raw) > 0 {
				if err := json.Unmarshal(raw, &known); err != nil {
					return err
				}
				if known == nil {
					known = map[string]bool{}
				}
			}
			if ours {
				known[installationKey(game)] = true
			} else {
				delete(known, installationKey(game))
			}
			c[retirementJourneyKey], _ = json.Marshal(known)
			return nil
		})
	}
	if err != nil {
		result.skip("Journey ownership receipt (deferred settings may need manual removal)", err)
	}
}
