package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"html/template"
	"strings"
	"sync"
)

const retirementMessage = "Relax Coach is retired. Its addons have been moved out of the game (kept in RelaxCoachRetired, not deleted)."

//go:embed retirement.html
var retirementHTML string

type retirementRun struct {
	mu      sync.Mutex
	cleanup func(context.Context) retirementResult
	done    bool
	result  retirementResult
}

func newRetirementApp(game string) *App {
	return &App{view: View{Retired: true}, retirement: &retirementRun{cleanup: func(ctx context.Context) retirementResult {
		return retireGame(ctx, game, addonGameRunning)
	}}}
}

func (b *nativeBridge) retirementCall(method string, args []json.RawMessage) (any, error) {
	if method == "applyApp" {
		var version string
		if len(args) != 1 || !strings.HasPrefix(strings.TrimSpace(string(args[0])), `"`) || json.Unmarshal(args[0], &version) != nil || len(version) > 4096 || strings.IndexByte(version, 0) >= 0 {
			return nil, errors.New("expected one text argument")
		}
		return b.applyApp(version)
	}
	if len(args) != 0 {
		return nil, errors.New("this method takes no arguments")
	}
	switch method {
	case "checkAppStartup":
		return b.checkApp(startupAppCheckTimeout)
	case "reportRefresh":
		b.mu.Lock()
		b.reportRendered = true
		b.mu.Unlock()
		return retirementMessage, nil
	case "appReady":
		// ready uses the unchanged startup validation, receipt and acknowledgement.
		// A failed/missing receipt returns here without calling cleanup at all.
		if err := b.ready(); err != nil {
			return nil, err
		}
		run := b.app.retirement
		run.mu.Lock()
		defer run.mu.Unlock()
		if err := b.ctx.Err(); err != nil {
			return nil, err
		}
		if !run.done {
			run.result = run.cleanup(b.ctx)
			run.done = true
		}
		return run.result, nil
	default:
		return nil, errors.New("Relax Coach is retired")
	}
}

func renderRetirementShell(token string, disabled bool) (string, error) {
	nonce, err := nativeToken()
	if err != nil {
		return "", err
	}
	tpl, err := template.New("retirement").Parse(retirementHTML)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	err = tpl.Execute(&out, struct {
		Token, Nonce, Message string
		Disabled              bool
	}{token, nonce, retirementMessage, disabled})
	return out.String(), err
}
