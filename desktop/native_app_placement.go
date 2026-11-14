package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

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
	return b.placementWork(func(e appPlacementEnv) (AppPlacement, error) { return e.install() })
}

// repairAppLinks takes a comma list of shortcuts the player confirmed may be
// replaced ("desktop", "startmenu"); empty creates missing ones only.
func (b *nativeBridge) repairAppLinks(list string) (AppPlacement, error) {
	var replace []string
	for _, r := range strings.Split(list, ",") {
		if r = strings.TrimSpace(r); r != "" {
			replace = append(replace, r)
		}
	}
	return b.placementWork(func(e appPlacementEnv) (AppPlacement, error) { return e.repairLinks(replace) })
}

func (b *nativeBridge) openInstalledApp() (AppPlacement, error) {
	return b.placementWork(func(e appPlacementEnv) (AppPlacement, error) { return e.openInstalled() })
}

// placementWork runs one placement action under the update busy lock.
func (b *nativeBridge) placementWork(work func(appPlacementEnv) (AppPlacement, error)) (AppPlacement, error) {
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
	return work(b.services.place())
}

// nativeLinkError keeps "access denied" recognisable (os.ErrPermission), so the
// message can say Windows blocked the write.
func nativeLinkError(err error) error {
	var hr uint32
	if err != nil {
		if _, scanErr := fmt.Sscanf(err.Error(), "COM method %d failed (HRESULT 0x%08x)", new(int), &hr); scanErr == nil && (hr == 0x80070005 || hr == 0x80030005) { // E_ACCESSDENIED, STG_E_ACCESSDENIED
			return fmt.Errorf("%w (%v)", os.ErrPermission, err)
		}
	}
	return err
}
