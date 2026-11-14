package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type nativeServices struct {
	checkApp   func(context.Context, string) (AppReleaseInfo, error)
	applyApp   func(context.Context, AppReleaseInfo) (AppUpdateResult, error)
	ready      func(context.Context) error
	check      func(context.Context, string) (AddonReleaseInfo, error)
	apply      func(context.Context, string, AddonReleaseInfo) (AddonUpdateResult, error)
	choose     func(context.Context) (string, error)
	checkExtra func(context.Context, string, string) (ExtraAddonInfo, error)
	applyExtra func(context.Context, string, ExtraAddonInfo) (AddonUpdateResult, error)
	open       func(string) error
	// place describes where this executable runs and how to copy it to the
	// usual folder (first run). nil: nothing is offered.
	place func() appPlacementEnv
	// appLast reads this executable's last app update result (ID, State);
	// startup is this process's app update transaction, if it was started by one.
	appLast  func() (id, state string, err error)
	startup  func() (id, from string, ok bool)
	gameOpen func(context.Context) (bool, error)
}

type nativeBridge struct {
	appRelease                                                        *AppReleaseInfo
	checkedAppVersion                                                 string
	appBusy, appRestarting, reportRendered, healthPending, healthSent bool
	app                                                               *App
	services                                                          nativeServices
	ctx                                                               context.Context
	cancel                                                            context.CancelFunc
	mu                                                                sync.Mutex
	workers                                                           sync.WaitGroup
	closed, busy, disabled                                            bool
	release                                                           *AddonReleaseInfo
	checkedGame, checkedVersion                                       string
	extra                                                             map[string]nativeExtraState
	afterAppAnswered                                                  bool
}

type nativeReport struct {
	AppVersion, AppUpdateStatus              string
	HTML, Game, InstalledVersion             string
	Account, SVPath, SaveTime, ConfigWarning string
	Accounts                                 []AccountChoice
	Stale                                    bool
	Bundle                                   AddonBundleState
	// Farming page (AH Profit, same account): HasFarm shows its nav entry.
	HasFarm  bool
	FarmHTML string
}

// Deliberately omit the download URL/checksum fields from the UI projection.
type nativeOffer struct {
	Version, Name, Notes, HTMLURL, InstalledVersion string
	Available                                       bool
	// Install: Coach is not installed (or a repair), so the action is Install.
	Install bool
	// Repair: the installed version again, to complete an incomplete install.
	Repair bool
	// NeedsApp: the release is newer than this app and carries RelaxCoach.exe,
	// so the app updates itself first (Available is then false).
	NeedsApp bool
	Bundle   AddonBundleState
}

type nativeRequest struct {
	ID     int64             `json:"id"`
	Method string            `json:"method"`
	Params []json.RawMessage `json:"params"`
	Token  string            `json:"token"`
}
type nativeResponse struct {
	Kind   string `json:"kind"`
	ID     int64  `json:"id"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

func decodeNativeRequest(raw, token string) (nativeRequest, error) {
	var req nativeRequest
	if len(raw) > 16384 {
		return req, errors.New("native request too large")
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return req, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return req, errors.New("invalid trailing request data")
	}
	if req.ID <= 0 || req.ID > 9007199254740991 || req.Method == "" || len(req.Params) > 1 || token == "" || subtle.ConstantTimeCompare([]byte(req.Token), []byte(token)) != 1 {
		return req, errors.New("invalid native request")
	}
	return req, nil
}

func newNativeBridge(a *App, disabled bool, s nativeServices) *nativeBridge {
	if s.checkApp == nil {
		s.checkApp = CheckAppRelease
	}
	if s.applyApp == nil {
		s.applyApp = ApplyAppRelease
	}
	if s.ready == nil {
		s.ready = acknowledgeAppStartup
	}
	if s.check == nil {
		s.check = CheckAddonRelease
	}
	if s.apply == nil {
		s.apply = ApplyAddonRelease
	}
	if s.checkExtra == nil {
		s.checkExtra = CheckExtraAddon
	}
	if s.applyExtra == nil {
		s.applyExtra = ApplyExtraAddon
	}
	if s.appLast == nil {
		s.appLast = currentAppLastState
	}
	if s.startup == nil {
		s.startup = currentAppStartup
	}
	if s.gameOpen == nil {
		s.gameOpen = addonGameRunning
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &nativeBridge{app: a, services: s, ctx: ctx, cancel: cancel, disabled: disabled}
}

// Admission and Close share one lock, so Wait cannot race a late Add. Close
// cancels network work and joins it; install rollback completes before process exit.
func (b *nativeBridge) Close() {
	b.mu.Lock()
	b.closed = true
	b.appRelease = nil
	b.release = nil
	b.cancel()
	b.mu.Unlock()
	b.workers.Wait()
}

func (b *nativeBridge) Call(method string, args []json.RawMessage) (any, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, errors.New("application is closing")
	}
	b.workers.Add(1)
	b.mu.Unlock()
	defer b.workers.Done()
	if err := b.ctx.Err(); err != nil {
		return nil, err
	}
	var arg string
	switch method {
	case "setGame", "selectAccount", "applyAddon", "applyApp", "openExternal", "checkExtra", "applyExtra", "repairAppLinks":
		// A JSON string only: null would decode as "" (for repairAppLinks, "all").
		if len(args) != 1 || !strings.HasPrefix(strings.TrimSpace(string(args[0])), `"`) || json.Unmarshal(args[0], &arg) != nil || len(arg) > 4096 || strings.IndexByte(arg, 0) >= 0 {
			return nil, errors.New("expected one text argument")
		}
	case "reportRefresh", "chooseGame", "checkAddon", "checkApp", "checkAppStartup", "appReady", "extraList", "appPlacement", "installAppCopy", "openInstalledApp", "afterAppUpdate", "gameState":
		if len(args) != 0 {
			return nil, errors.New("this method takes no arguments")
		}
	default:
		return nil, errors.New("unknown native method")
	}
	switch method {
	case "reportRefresh":
		return b.report()
	case "setGame":
		return b.setGame(arg)
	case "selectAccount":
		return b.selectAccount(arg)
	case "chooseGame":
		if b.services.choose == nil {
			return nil, errors.New("folder chooser is unavailable; paste the game path instead")
		}
		game, err := b.services.choose(b.ctx)
		if err != nil {
			return nil, err
		}
		if game == "" {
			return nil, nil
		}
		return b.setGame(game)
	case "checkApp":
		return b.checkApp(appCheckTimeout)
	case "checkAppStartup":
		return b.checkApp(startupAppCheckTimeout)
	case "applyApp":
		return b.applyApp(arg)
	case "appReady":
		return nil, b.ready()
	case "checkAddon":
		return b.check()
	case "applyAddon":
		return b.apply(arg)
	case "extraList":
		return b.extraList()
	case "checkExtra":
		return b.checkExtra(arg)
	case "applyExtra":
		return b.applyExtra(arg)
	case "appPlacement":
		return b.appPlacement()
	case "installAppCopy":
		return b.installAppCopy()
	case "repairAppLinks":
		return b.repairAppLinks(arg)
	case "openInstalledApp":
		return b.openInstalledApp()
	case "afterAppUpdate":
		return b.afterAppUpdate()
	case "gameState":
		return b.gameState(), nil
	case "openExternal":
		if err := validateExternalURL(arg); err != nil {
			return nil, err
		}
		if b.services.open == nil {
			return nil, errors.New("external browser is unavailable")
		}
		return nil, b.services.open(arg)
	}
	return nil, errors.New("unknown native method")
}

func (b *nativeBridge) report() (nativeReport, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.reportLocked()
}

func (b *nativeBridge) reportLocked() (nativeReport, error) {
	if b.closed {
		return nativeReport{}, errors.New("application is closing")
	}
	b.app.refresh()
	v := b.app.snapshot()
	html, err := renderReportBody(v)
	if err == nil {
		b.reportRendered = true
	}
	farmHTML, ferr := renderFarmBody(v)
	if err == nil {
		err = ferr
	}
	saveTime := ""
	if !v.SaveTime.IsZero() {
		saveTime = v.SaveTime.Format(time.RFC3339)
	}
	return nativeReport{HTML: html, Game: v.Game, InstalledVersion: InstalledCoachVersion(v.Game),
		AppVersion: Version, AppUpdateStatus: currentAppUpdateStatus(),
		Account: v.Account, Accounts: v.Accounts, SVPath: v.SVPath, SaveTime: saveTime,
		Stale: v.Warning != "", ConfigWarning: v.ConfigWarning, Bundle: InstalledBundleState(v.Game),
		HasFarm: v.Farm.Present, FarmHTML: farmHTML}, err
}

func (b *nativeBridge) selectAccount(id string) (nativeReport, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nativeReport{}, errors.New("application is closing")
	}
	if b.busy || b.appBusy || b.appRestarting {
		return nativeReport{}, errors.New("wait for the addon operation before changing accounts")
	}
	if err := b.app.selectAccount(id); err != nil {
		return nativeReport{}, err
	}
	return b.reportLocked()
}

func (b *nativeBridge) setGame(input string) (nativeReport, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nativeReport{}, errors.New("application is closing")
	}
	if b.busy || b.appBusy || b.appRestarting {
		return nativeReport{}, errors.New("wait for the addon operation before changing folders")
	}
	game := NormalizeGame(input)
	if game == "" {
		return nativeReport{}, errors.New("that folder does not look like WoW Forever; choose or paste the _forever_ folder")
	}
	if err := saveGame(game); err != nil {
		return nativeReport{}, err
	}
	b.release = nil
	b.extra = nil
	b.app.setGame(game)
	return b.reportLocked()
}

func (b *nativeBridge) startUpdate() (string, string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return "", "", errors.New("application is closing")
	}
	if b.disabled {
		return "", "", errors.New("updates are disabled by -no-update")
	}
	if b.busy || b.appBusy || b.appRestarting {
		return "", "", errors.New("another addon operation is already running")
	}
	game := b.app.snapshot().Game
	if game == "" {
		return "", "", errors.New("choose your WoW Forever game folder first")
	}
	b.busy = true
	return game, InstalledCoachVersion(game), nil
}
func (b *nativeBridge) finishUpdate() { b.mu.Lock(); b.busy = false; b.mu.Unlock() }

func (b *nativeBridge) check() (nativeOffer, error) {
	game, current, err := b.startUpdate()
	if err != nil {
		return nativeOffer{}, err
	}
	defer b.finishUpdate()
	b.mu.Lock()
	b.release = nil
	b.mu.Unlock()
	ctx, cancel := context.WithTimeout(b.ctx, 45*time.Second)
	defer cancel()
	release, err := b.services.check(ctx, current)
	if err != nil {
		return nativeOffer{}, err
	}
	// The latest release is the installed one, but the install is incomplete
	// (a missing folder, or an install interrupted by a crash): offer it again
	// as Install, through the same verified transaction.
	if !release.Available && release.sameVersion != nil && addonRepairNeeded(game) {
		release = *release.sameVersion
	}
	if err = ctx.Err(); err != nil {
		return nativeOffer{}, err
	}
	b.mu.Lock()
	if !b.closed {
		copy := release
		b.release = &copy
		b.checkedGame = game
		b.checkedVersion = current
	}
	b.mu.Unlock()
	// A bad release-page URL cannot become an executable/shell capability.
	link := release.HTMLURL
	if validateExternalURL(link) != nil {
		link = ""
	}
	bundle := InstalledBundleState(game)
	needsApp := release.Available && b.appFirst(release)
	return nativeOffer{Version: release.Version, Name: release.Name, Notes: release.Notes, HTMLURL: link, InstalledVersion: current,
		Available: release.Available && !needsApp, Install: current == "0.0.0" || release.Repair, Repair: release.Repair, NeedsApp: needsApp, Bundle: bundle}, nil
}

func (b *nativeBridge) apply(expected string) (AddonUpdateResult, error) {
	game, current, err := b.startUpdate()
	if err != nil {
		return AddonUpdateResult{}, err
	}
	defer b.finishUpdate()
	b.mu.Lock()
	r := b.release
	if r == nil || !r.Available || r.Version != expected || b.checkedGame != game || b.checkedVersion != current {
		b.mu.Unlock()
		return AddonUpdateResult{}, errors.New("check again and confirm the offered version")
	}
	release := *r
	b.release = nil // A failed install requires a fresh explicit check too.
	b.mu.Unlock()
	// App first: this app never installs a bundle newer than itself while the
	// same release has a new app; the new app knows the new bundle's file set.
	if b.appFirst(release) {
		return AddonUpdateResult{}, errors.New("Update the app first: Relax Coach " + release.Version + " needs app " + release.Version + ". Click Update App (or Update everything); the addons follow.")
	}
	ctx, cancel := context.WithTimeout(b.ctx, 5*time.Minute)
	defer cancel()
	result, err := b.services.apply(ctx, game, release)
	if err != nil {
		return result, fmt.Errorf("%w", err)
	}
	b.app.refresh()
	return result, nil
}
