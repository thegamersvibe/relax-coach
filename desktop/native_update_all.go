package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// App first (Relax 1.6): an app from 1.6 on never installs an addon bundle newer
// than itself while the same release also carries RelaxCoach.exe. App and bundle
// come from one release, so the app that installs bundle N is app N, which knows
// N's pinned file set (addon_update_archive.go). Old apps cannot be changed; they
// still need their own Update App once.

// appFirst is true when release must wait for an app update. Escape: when this
// exe's last app update failed, rolled back or needs recovery, the bundle may be
// tried directly; the pinned file set still refuses an unknown set before any
// disk change.
func (b *nativeBridge) appFirst(release AddonReleaseInfo) bool {
	if !release.HasApp {
		return false
	}
	remote, err := parseAddonVersion(release.Version)
	if err != nil {
		return false
	}
	local, err := parseAddonVersion(Version)
	if err != nil || !remote.newerThan(local) {
		return false
	}
	if _, state, err := b.services.appLast(); err == nil && (state == "failed" || state == "rolled-back" || state == "recovery") {
		return false
	}
	return true
}

// appLastResultState reads ID and State from last-result.json next to target.
func appLastResultState(target string) (id, state string, err error) {
	raw, err := appReadFile(filepath.Join(appRoot(target), "last-result.json"), 16384)
	if os.IsNotExist(err) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	var r struct{ ID, State string }
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", "", errors.New("last app update result is unreadable")
	}
	return r.ID, r.State, nil
}

func currentAppLastState() (string, string, error) {
	target, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	return appLastResultState(target)
}

// currentAppStartup is this process's app update transaction (started by an
// app update), with the version it replaced.
func currentAppStartup() (id, from string, ok bool) {
	if appStartup == nil {
		return "", "", false
	}
	return appStartup.transaction.ID, appStartup.transaction.OldVersion, true
}

// AfterAppUpdate tells the page that this process is the new app of an app
// update that has committed, so it may update the addons by itself.
type AfterAppUpdate struct {
	Updated    bool
	From, Game string
}

// nativeAfterAppWait and nativeAfterAppPoll bound the wait for the worker to
// record the commit. Until then the worker owns this process and stops it on
// any failure, so nothing may start before committed.
var nativeAfterAppWait, nativeAfterAppPoll = 15 * time.Second, 250 * time.Millisecond

// afterAppUpdate answers Updated true at most once per process, only when this
// process was started by an app update and last-result.json records committed
// for that transaction's ID within nativeAfterAppWait.
func (b *nativeBridge) afterAppUpdate() (AfterAppUpdate, error) {
	b.mu.Lock()
	if b.afterAppAnswered {
		b.mu.Unlock()
		return AfterAppUpdate{}, nil
	}
	b.afterAppAnswered = true
	b.mu.Unlock()
	id, from, ok := b.services.startup()
	if !ok || id == "" {
		return AfterAppUpdate{}, nil
	}
	ctx, cancel := context.WithTimeout(b.ctx, nativeAfterAppWait)
	defer cancel()
	for {
		lastID, state, err := b.services.appLast()
		if err == nil && lastID == id {
			if state == "committed" {
				return AfterAppUpdate{Updated: true, From: from, Game: b.app.snapshot().Game}, nil
			}
			if state == "rolled-back" || state == "failed" || state == "recovery" {
				return AfterAppUpdate{}, nil
			}
		}
		select {
		case <-ctx.Done():
			return AfterAppUpdate{}, nil
		case <-time.After(nativeAfterAppPoll):
		}
	}
}

// gameState is "open", "closed" or "unknown" (the check failed); read-only.
func (b *nativeBridge) gameState() string {
	open, err := b.services.gameOpen(b.ctx)
	switch {
	case err != nil:
		return "unknown"
	case open:
		return "open"
	}
	return "closed"
}
