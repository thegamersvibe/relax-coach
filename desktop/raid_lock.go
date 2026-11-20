package main

import (
	"errors"
	"io"
	"math"
	"strings"
	"unicode/utf8"
)

// Input budgets, not guild/raid-size rules. The rank table is local characters,
// not the raid roster. The byte budget also accommodates the bounded prep store.
const raidSaveMaxBytes = 16 << 20
const raidSaveMaxValues = 1000000
const raidRankMaxCharacters = 256

func raidRankText(s string, limit int) bool {
	return s != "" && len(s) <= limit && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, func(r rune) bool { return r < 32 || r == 127 }) < 0
}

// Validate every row before accepting any officer. Do not return early on an
// officer: a malformed sibling means the untrusted table cannot unlock anything.
func raidOfficer(data []byte) bool {
	if len(data) > raidSaveMaxBytes {
		return false
	}
	vars, err := ParseSavedVariablesLimited(string(data), raidSaveMaxValues)
	if err != nil {
		return false
	}
	db, ok := vars["RelaxRaidDB"].(map[string]any)
	if !ok || db["version"] != float64(1) {
		return false
	}
	ranks, ok := db["ranks"].(map[string]any)
	if !ok || len(ranks) > raidRankMaxCharacters {
		return false
	}
	officer := false
	for name, raw := range ranks {
		if !raidRankText(name, 64) || !strings.Contains(name, "-") {
			return false
		}
		row, ok := raw.(map[string]any)
		if !ok {
			return false
		}
		for key := range row {
			if key != "result" && key != "guild" && key != "readAt" {
				return false
			}
		}
		result, ok := row["result"].(string)
		if !ok {
			return false
		}
		switch result {
		case "officer":
			officer = true
		case "member", "noguild", "unreadable":
		default:
			return false
		}
		at, ok := row["readAt"].(float64)
		if !ok || math.IsNaN(at) || math.IsInf(at, 0) || at <= 0 || at > 4294967295 || math.Trunc(at) != at {
			return false
		}
		if g, exists := row["guild"]; exists {
			s, ok := g.(string)
			if !ok || s != "" && !raidRankText(s, 256) {
				return false
			}
		}
	}
	return officer
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
		return nil, err
	}
	defer root.Close()
	f, err := openAccountFile(root, a.account, "RelaxRaid.lua")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, raidSaveMaxBytes+1))
	if err != nil || !raidOfficer(data) {
		return nil, errors.New("Raids locked: no validated officer reading in the selected account's save")
	}
	return data, nil
}
func (a *App) raidAllowedLocked() bool { _, err := a.raidSaveLocked(); return err == nil }

// Admission is checked on each call, not from the rendered report. Holding the
// selection lock through rendering prevents an account change crossing the read.
func (b *nativeBridge) raidPage() (any, error) {
	b.app.mu.Lock()
	defer b.app.mu.Unlock()
	b.app.rescanAccountsLocked()
	if _, err := b.app.raidSaveLocked(); err != nil {
		return nil, errors.New("Raids locked: no validated officer reading in the selected account's save")
	}
	return renderRaidShell()
}
