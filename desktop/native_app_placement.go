package main

import "errors"

// First-run placement: read-only state, then an explicit copy on the player's click.
func (b *nativeBridge) appPlacement() (AppPlacement, error) {
	b.mu.Lock()
	closed := b.closed
	b.mu.Unlock()
	if closed {
		return AppPlacement{}, errors.New("application is closing")
	}
	if b.services.place == nil {
		return AppPlacement{State: "unavailable"}, nil
	}
	return b.services.place().placement(), nil
}

func (b *nativeBridge) installAppCopy() (AppPlacement, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return AppPlacement{}, errors.New("application is closing")
	}
	if b.busy || b.appBusy || b.appRestarting {
		b.mu.Unlock()
		return AppPlacement{}, errors.New("another operation is already running")
	}
	if b.services.place == nil {
		b.mu.Unlock()
		return AppPlacement{}, errors.New("copying the app is not available here")
	}
	b.busy = true
	b.mu.Unlock()
	defer b.finishUpdate()
	return b.services.place().install()
}
