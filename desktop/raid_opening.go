package main

import (
	"errors"
	"fmt"
	"strconv"
)

// Only the chosen combat-log location and night index survive a request, in
// memory. An officer path, its bytes and its parsed prediction never live here.
type raidSelection struct {
	Game, Account, Path string
	Night               int
}

var errRaidPickerStale = errors.New("raid selection changed while choosing a file")

type raidNightChoice struct {
	Index int
	Label string
}
type raidScreen struct {
	Page           *raidPageModel
	Nights         []raidNightChoice
	Selected       int
	LogPath, Error string
	ConfigError    bool
}

func (b *nativeBridge) raidAction(method, arg string) (any, error) {
	b.app.mu.Lock()
	defer b.app.mu.Unlock()
	b.app.rescanAccountsLocked()
	save, err := b.app.raidSaveLocked()
	if err != nil {
		b.app.raidSelection = nil
		b.app.raidRevision++
		return nil, err
	}
	return b.raidActionLocked(method, arg, save)
}

// Caller holds the selected-account lock through file reading and rendering.
func (b *nativeBridge) raidActionLocked(method, arg string, save []byte) (any, error) {
	if b.raidRulesErr != nil {
		return renderRaidScreen(raidScreen{Error: "Raid configuration: " + b.raidRulesErr.Error(), ConfigError: true})
	}
	a := b.app
	if s := a.raidSelection; s != nil && (s.Game != a.game || s.Account != a.account) {
		a.raidSelection = nil
		a.raidRevision++
	}
	selection := raidSelection{Game: a.game, Account: a.account}
	if a.raidSelection != nil {
		selection = *a.raidSelection
	}
	officerPath := ""
	switch method {
	case "raidOpen":
		selection = raidSelection{Game: a.game, Account: a.account, Path: arg}
	case "raidNight":
		index, err := strconv.Atoi(arg)
		if err != nil || index < 0 {
			return renderRaidScreen(raidScreen{Error: "invalid night selection"})
		}
		selection.Night = index
	case "raidOfficerFile":
		if arg == "" {
			return renderRaidScreen(raidScreen{Error: "choose an officer file path"})
		}
		officerPath = arg
	}
	if a.raidSelection == nil && method != "raidOpen" {
		return renderRaidScreen(raidScreen{})
	}
	file, err := openRaidLog(a.game, selection.Path)
	if err != nil {
		return renderRaidScreen(raidScreen{Error: err.Error()})
	}
	defer file.Close()
	pages, err := readRaidPagesSelected(file, save, b.raidRules, officerPath, selection.Night)
	if err != nil {
		return renderRaidScreen(raidScreen{Error: err.Error()})
	}
	if len(pages) == 0 {
		return renderRaidScreen(raidScreen{Error: "no nights found in this combat log"})
	}
	if selection.Night >= len(pages) {
		return renderRaidScreen(raidScreen{Error: "invalid night selection"})
	}
	selection.Path = file.Name()
	if method == "raidOpen" || method == "raidNight" {
		a.raidSelection = &selection
		a.raidRevision++
	}
	view := raidScreen{Page: &pages[selection.Night], Selected: selection.Night, LogPath: selection.Path}
	for index, p := range pages {
		view.Nights = append(view.Nights, raidNightChoice{Index: index, Label: fmt.Sprintf("%s — %s", p.Title, p.Date)})
	}
	return renderRaidScreen(view)
}

// Like the existing folder chooser, the native picker runs on the UI thread.
// Do not hold App.mu across it; re-admit afterwards and reject stale choices,
// including an account switch away and back or a different night opened meanwhile.
func (b *nativeBridge) chooseRaidFile(kind string) (any, error) {
	a := b.app
	a.mu.Lock()
	a.rescanAccountsLocked()
	_, err := a.raidSaveLocked()
	revision, game, account := a.raidRevision, a.game, a.account
	if err != nil {
		a.mu.Unlock()
		return nil, err
	}
	if b.raidRulesErr != nil {
		a.mu.Unlock()
		return renderRaidScreen(raidScreen{Error: "Raid configuration: " + b.raidRulesErr.Error(), ConfigError: true})
	}
	if kind == "officer" && a.raidSelection == nil {
		a.mu.Unlock()
		return renderRaidScreen(raidScreen{Error: "open a night before choosing an officer file"})
	}
	a.mu.Unlock()
	if b.services.chooseFile == nil {
		return renderRaidScreen(raidScreen{Error: "file chooser is unavailable; paste the file path instead"})
	}
	path, pickErr := b.services.chooseFile(b.ctx, kind)
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := b.ctx.Err(); err != nil {
		return nil, err
	}
	a.rescanAccountsLocked()
	// A result belongs to the account that started the picker. After a switch
	// (even away and back) it is stale: no refusal may be latched for it.
	if game != a.game || account != a.account || revision != a.raidRevision {
		return nil, errRaidPickerStale
	}
	// Admission is rechecked after every picker outcome, failures included.
	save, err := a.raidSaveLocked()
	if err != nil {
		a.raidSelection = nil
		a.raidRevision++
		return nil, err
	}
	if pickErr != nil {
		return nil, pickErr
	}
	if path == "" {
		return nil, nil
	}
	method := "raidOpen"
	if kind == "officer" {
		method = "raidOfficerFile"
	}
	return b.raidActionLocked(method, path, save)
}
