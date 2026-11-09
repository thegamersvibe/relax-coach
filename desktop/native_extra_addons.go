package main

import (
	"context"
	"errors"
	"strings"
	"time"
)

// Bridge methods for the optional companion addons. The UI only ever sends an
// addon id (and the version it showed); URLs and checksums stay in Go memory.
type nativeExtraOffer struct {
	ID, Name, Blurb, InstalledVersion, Version, Notes, HTMLURL, Status string
	Problem                                                            string
	Available, Install                                                 bool
}

type nativeExtraState struct {
	game, installed string
	info            ExtraAddonInfo
}

func (b *nativeBridge) startExtra() (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return "", errors.New("application is closing")
	}
	if b.disabled {
		return "", errors.New("updates are disabled by -no-update")
	}
	if b.busy || b.appBusy || b.appRestarting {
		return "", errors.New("another addon operation is already running")
	}
	game := b.app.snapshot().Game
	if game == "" {
		return "", errors.New("choose your WoW Forever game folder first")
	}
	b.busy = true
	return game, nil
}

// extraList is read-only and offline: installed versions only.
func (b *nativeBridge) extraList() ([]nativeExtraOffer, error) {
	b.mu.Lock()
	closed := b.closed
	game := b.app.snapshot().Game
	b.mu.Unlock()
	if closed {
		return nil, errors.New("application is closing")
	}
	out := make([]nativeExtraOffer, 0, len(extraAddons))
	for _, a := range extraAddons {
		v := "0.0.0"
		if game != "" {
			v = InstalledExtraVersion(game, a)
		}
		out = append(out, nativeExtraOffer{ID: a.ID, Name: a.Name, Blurb: a.Blurb, InstalledVersion: v, Problem: ExtraInstalledProblem(game, a)})
	}
	return out, nil
}

func (b *nativeBridge) checkExtra(id string) (nativeExtraOffer, error) {
	a, ok := extraAddonByID(id)
	if !ok {
		return nativeExtraOffer{}, errors.New("unknown addon")
	}
	game, err := b.startExtra()
	if err != nil {
		return nativeExtraOffer{}, err
	}
	defer b.finishUpdate()
	b.mu.Lock()
	delete(b.extra, id)
	b.mu.Unlock()
	current := InstalledExtraVersion(game, a)
	if p := ExtraInstalledProblem(game, a); p != "" {
		return nativeExtraOffer{ID: a.ID, Name: a.Name, Blurb: a.Blurb, InstalledVersion: current, Problem: p,
			Status: "Unrecognized " + a.Folder + " folder: " + p + ". Nothing will be replaced."}, nil
	}
	ctx, cancel := context.WithTimeout(b.ctx, 45*time.Second)
	defer cancel()
	info, err := b.services.checkExtra(ctx, id, current)
	if err != nil {
		return nativeExtraOffer{}, err
	}
	if err = ctx.Err(); err != nil {
		return nativeExtraOffer{}, err
	}
	b.mu.Lock()
	if !b.closed {
		if b.extra == nil {
			b.extra = map[string]nativeExtraState{}
		}
		b.extra[id] = nativeExtraState{game: game, installed: current, info: info}
	}
	b.mu.Unlock()
	link := info.HTMLURL
	if validateExternalURL(link) != nil {
		link = ""
	}
	return nativeExtraOffer{ID: a.ID, Name: a.Name, Blurb: a.Blurb, InstalledVersion: current, Version: info.Version, Notes: info.Notes,
		HTMLURL: link, Status: info.Status, Available: info.Available, Install: info.Install}, nil
}

// arg is "<id>@<version>": the version the user saw and confirmed.
func (b *nativeBridge) applyExtra(arg string) (AddonUpdateResult, error) {
	id, version, ok := strings.Cut(arg, "@")
	a, known := extraAddonByID(id)
	if !ok || !known {
		return AddonUpdateResult{}, errors.New("unknown addon")
	}
	game, err := b.startExtra()
	if err != nil {
		return AddonUpdateResult{}, err
	}
	defer b.finishUpdate()
	current := InstalledExtraVersion(game, a)
	b.mu.Lock()
	s, found := b.extra[id]
	delete(b.extra, id) // a failed install needs a fresh check too
	b.mu.Unlock()
	if !found || !s.info.Available || s.info.Version != version || s.game != game || s.installed != current {
		return AddonUpdateResult{}, errors.New("check again and confirm the offered version")
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Minute)
	defer cancel()
	return b.services.applyExtra(ctx, game, s.info)
}
