package main

import (
	"errors"
	"io"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// Input budgets, not raid-size rules. Ranks hold local characters; the save's
// byte budget also accommodates the bounded preparation store.
const raidSaveMaxBytes = 16 << 20
const raidSaveMaxValues = 1000000
const raidRankMaxCharacters = 50

// Starting admission limit, deliberately tunable. Compare server seconds with
// PC UTC; a badly set PC clock fails closed rather than keeping a stale unlock.
const raidOfficerMaxAge = 14 * 24 * time.Hour
const raidOfficerFutureAllowance = 24 * time.Hour

var errRaidRankLocked = errors.New("Raids locked: no validated officer reading in the selected account's save")
var errRaidTimeLocked = errors.New("Raids locked: the officer reading is too old or its time is invalid")

func raidRankText(s string, limit int) bool {
	return s != "" && len(s) <= limit && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, func(r rune) bool { return r < 32 || r == 127 }) < 0
}

// Bad row values never admit, but do not poison another valid officer. Raw
// key type/name, duplicates and the 50-entry cap are whole-save boundaries.
func raidAdmission(data []byte, now time.Time) error {
	if len(data) > raidSaveMaxBytes {
		return errRaidRankLocked
	}
	vars, err := raidSaveVariables(data)
	if err != nil {
		return errRaidRankLocked
	}
	db, ok := vars["RelaxRaidDB"].(map[string]any)
	if !ok || db["version"] != float64(1) {
		return errRaidRankLocked
	}
	ranks, ok := db["ranks"].(map[string]any)
	if !ok {
		return errRaidRankLocked
	}
	officer, badTime := false, false
	nowS := now.UTC().Unix()
	for _, raw := range ranks {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		valid := true
		for key := range row {
			if key != "result" && key != "guild" && key != "readAt" {
				valid = false
			}
		}
		result, ok := row["result"].(string)
		if !valid || !ok || result != "officer" {
			continue
		}
		if g, exists := row["guild"]; exists {
			s, ok := g.(string)
			if !ok || s != "" && !raidRankText(s, 96) {
				continue
			}
		}
		at, ok := row["readAt"].(float64)
		if !ok || math.IsNaN(at) || math.IsInf(at, 0) || at <= 0 || at > 4294967295 || math.Trunc(at) != at {
			badTime = true
			continue
		}
		delta := nowS - int64(at)
		if nowS <= 0 || delta > int64(raidOfficerMaxAge/time.Second) || delta < -int64(raidOfficerFutureAllowance/time.Second) {
			badTime = true
			continue
		}
		officer = true
	}
	if officer {
		return nil
	}
	if badTime {
		return errRaidTimeLocked
	}
	return errRaidRankLocked
}

// Called under App.mu. Never consult another account or a last-good save.
func (a *App) raidSaveLocked() ([]byte, error) {
	available := false
	for _, account := range a.accounts {
		if account.ID == a.account {
			available = true
		}
	}
	if a.game == "" || !available {
		return nil, errors.New("Raids locked: choose an available account")
	}
	root, err := openAccountsRoot(a.game)
	if err != nil {
		return nil, errRaidRankLocked
	}
	defer root.Close()
	f, err := openAccountFile(root, a.account, "RelaxRaid.lua")
	if err != nil {
		return nil, errRaidRankLocked
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, raidSaveMaxBytes+1))
	if err != nil {
		return nil, errRaidRankLocked
	}
	now := time.Now()
	if a.raidNow != nil {
		now = a.raidNow()
	}
	if err := raidAdmission(data, now); err != nil {
		return nil, err
	}
	return data, nil
}
func (a *App) raidAllowedLocked() bool { _, err := a.raidSaveLocked(); return err == nil }

// Admission is checked on each call, not from the rendered report. Holding the
// selection lock through rendering prevents an account change crossing the read.
func (b *nativeBridge) raidPage() (any, error) {
	return b.raidAction("raidPage", "")
}
