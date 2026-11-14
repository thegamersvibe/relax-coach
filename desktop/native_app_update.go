package main

import (
	"context"
	"errors"
	"time"
)

func (b *nativeBridge) startAppUpdate() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("application is closing")
	}
	if b.disabled {
		return errors.New("updates are disabled by -no-update")
	}
	if b.busy || b.appBusy || b.appRestarting {
		return errors.New("another update operation is already running")
	}
	b.appBusy = true
	return nil
}
func (b *nativeBridge) finishAppUpdate() { b.mu.Lock(); b.appBusy = false; b.mu.Unlock() }

// The manual check keeps the long budget; the read-only launch check gives up
// sooner so a slow GitHub cannot hold every button disabled for 45 seconds.
const (
	appCheckTimeout        = 45 * time.Second
	startupAppCheckTimeout = 10 * time.Second
)

func (b *nativeBridge) checkApp(timeout time.Duration) (nativeOffer, error) {
	if err := b.startAppUpdate(); err != nil {
		return nativeOffer{}, err
	}
	defer b.finishAppUpdate()
	b.mu.Lock()
	b.appRelease = nil
	b.mu.Unlock()
	current := Version
	ctx, cancel := context.WithTimeout(b.ctx, timeout)
	defer cancel()
	r, err := b.services.checkApp(ctx, current)
	if err != nil {
		return nativeOffer{}, err
	}
	if err = ctx.Err(); err != nil {
		return nativeOffer{}, err
	}
	b.mu.Lock()
	if !b.closed {
		copy := r
		b.appRelease = &copy
		b.checkedAppVersion = current
	}
	b.mu.Unlock()
	link := r.HTMLURL
	if validateExternalURL(link) != nil {
		link = ""
	}
	return nativeOffer{Version: r.Version, Name: r.Name, Notes: r.Notes, HTMLURL: link, InstalledVersion: current, Available: r.Available}, nil
}
func (b *nativeBridge) applyApp(expected string) (AppUpdateResult, error) {
	if err := b.startAppUpdate(); err != nil {
		return AppUpdateResult{}, err
	}
	defer b.finishAppUpdate()
	b.mu.Lock()
	r := b.appRelease
	b.appRelease = nil
	current := b.checkedAppVersion
	b.mu.Unlock()
	if r == nil || !r.Available || r.Version != expected || current != Version {
		return AppUpdateResult{}, errors.New("check app updates again and confirm the offered version")
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Minute)
	defer cancel()
	result, err := b.services.applyApp(ctx, *r)
	if err != nil {
		// A failure before the app update had a transaction leaves no
		// last-result; remember it so the addons are not held back (appFirst).
		if b.ctx.Err() == nil {
			b.mu.Lock()
			b.appApplyFailed = true
			b.mu.Unlock()
		}
		return AppUpdateResult{}, err
	}
	if result.RestartReady {
		b.mu.Lock()
		b.appRestarting = true
		b.mu.Unlock()
	}
	return result, nil
}
func (b *nativeBridge) ready() error {
	b.mu.Lock()
	if !b.reportRendered {
		b.mu.Unlock()
		return errors.New("native shell has not completed its initial report RPC")
	}
	if b.healthSent {
		b.mu.Unlock()
		return nil
	}
	if b.healthPending {
		b.mu.Unlock()
		return errors.New("native startup health is already pending")
	}
	b.healthPending = true
	b.mu.Unlock()
	err := b.services.ready(b.ctx)
	b.mu.Lock()
	b.healthPending = false
	b.healthSent = err == nil
	b.mu.Unlock()
	return err
}

// Called only in the UI-thread response dispatch, after PostWebMessage succeeds.
func nativeAppRestartReady(method string, value any, callErr, errorPosting error) bool {
	r, ok := value.(AppUpdateResult)
	return method == "applyApp" && ok && r.RestartReady && callErr == nil && errorPosting == nil
}

// nativeOpenedInstalled is true when the installed copy was just started, so
// this window closes once the response is posted (like an app update restart).
func nativeOpenedInstalled(method string, value any, callErr, errorPosting error) bool {
	p, ok := value.(AppPlacement)
	return method == "openInstalledApp" && ok && p.State == "opened" && callErr == nil && errorPosting == nil
}
