package main

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRetirementStartHealthBeforeAnyDeletion(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			game := retirementFixture(t)
			addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "keep until ack")
			target := filepath.Join(t.TempDir(), "RelaxCoach.exe")
			old, candidate := appTestPE(), appTestPE()
			old[513] = 1
			candidate[513] = 2
			if err := os.WriteFile(target, old, 0700); err != nil {
				t.Fatal(err)
			}
			tx, err := prepareAppTransaction(target, "1.8.2", "1.9.0", candidate, []string{"-game", game, "-no-update"}, os.Getpid(), "synthetic-parent")
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(target, candidate, 0700); err != nil {
				t.Fatal(err)
			}
			prevRT, prevStartup := appRuntime, appStartup
			t.Cleanup(func() { appRuntime, appStartup = prevRT, prevStartup })
			appRuntime = &appRuntimeState{Target: target, Version: "1.9.0", SHA: tx.NewSHA, PID: os.Getpid()}
			mode, id, args, err := parseAppMode(append([]string{"-app-update-start", tx.ID}, tx.Args...))
			if err != nil || mode != "start" || id != tx.ID || !reflect.DeepEqual(args, tx.Args) {
				t.Fatalf("start args: %s %s %v %v", mode, id, args, err)
			}
			if err = initAppStartup(id); err != nil {
				t.Fatal(err)
			}
			var events []string
			appStartup.send = func(ctx context.Context, got appTransaction) error {
				retirementExists(t, filepath.Join(game, "Interface/AddOns/ForeverCoach/Core.lua"), true)
				msg := appHealthMessage(got, os.Getpid())
				want := fmt.Sprintf("healthy %s 1.9.0 %s %d", tx.fingerprint(), tx.NewSHA, os.Getpid())
				if msg != want {
					t.Fatalf("receipt %q want %q", msg, want)
				}
				events = append(events, "health receipt")
				if fail {
					return errors.New("worker did not ack")
				}
				events = append(events, "ack")
				return nil
			}
			a := newRetirementApp(game)
			a.retirement.cleanup = func(ctx context.Context) retirementResult {
				if !appStartup.acked {
					t.Fatal("deletion call before health ack")
				}
				events = append(events, "delete")
				return retireGame(ctx, game, retirementClosed)
			}
			b := newNativeBridge(a, true, nativeServices{})
			defer b.Close()
			if _, err = b.Call("appReady", nil); err == nil {
				t.Fatal("cleanup before initial rendered report")
			}
			if len(events) != 0 {
				t.Fatalf("premature work: %v", events)
			}
			if _, err = b.Call("reportRefresh", nil); err != nil {
				t.Fatal(err)
			}
			_, err = b.Call("appReady", nil)
			if fail {
				if err == nil || !reflect.DeepEqual(events, []string{"health receipt"}) {
					t.Fatalf("failed ack: %v %v", events, err)
				}
				retirementExists(t, filepath.Join(game, "Interface/AddOns/ForeverCoach"), true)
			} else {
				if err != nil || !reflect.DeepEqual(events, []string{"health receipt", "ack", "delete"}) {
					t.Fatalf("ordering: %v %v", events, err)
				}
				retirementExists(t, filepath.Join(game, "Interface/AddOns/ForeverCoach"), false)
				if _, err = b.Call("appReady", nil); err != nil {
					t.Fatal(err)
				}
				if len(events) != 3 {
					t.Fatalf("duplicate work: %v", events)
				}
			}
		})
	}
}

func TestRetirementPlainLaunchAndOfflineBridge(t *testing.T) {
	prior := appStartup
	appStartup = nil
	t.Cleanup(func() { appStartup = prior })
	game := retirementFixture(t)
	addonPut(t, game, "Interface/AddOns/RelaxRaid/Core.lua", "synthetic")
	a := newRetirementApp(game)
	calls := 0
	b := newNativeBridge(a, false, nativeServices{checkApp: func(context.Context, string) (AppReleaseInfo, error) { calls++; return AppReleaseInfo{}, nil }, check: func(context.Context, string) (AddonReleaseInfo, error) { calls++; return AddonReleaseInfo{}, nil }})
	defer b.Close()
	for _, method := range []string{"checkApp", "checkAppStartup", "checkAddon", "extraList", "afterAppUpdate", "reportRefresh-old", "installAppCopy", "setWowheadNames", "openExternal", "setGame"} {
		if _, err := b.Call(method, nil); err == nil {
			t.Fatalf("retired method allowed: %s", method)
		}
	}
	if calls != 0 {
		t.Fatal("network service reached")
	}
	if _, err := b.Call("reportRefresh", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call("appReady", nil); err != nil {
		t.Fatal(err)
	}
	retirementExists(t, filepath.Join(game, "Interface/AddOns/RelaxRaid"), false)
}

func TestRetirementRunningEntrypointIsOfflineStatic(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var main *ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "main" {
			main = fn
		}
	}
	if main == nil {
		t.Fatal("main missing")
	}
	calls := map[string]bool{}
	ast.Inspect(main, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			switch fn := c.Fun.(type) {
			case *ast.Ident:
				calls[fn.Name] = true
			case *ast.SelectorExpr:
				calls[fn.Sel.Name] = true
			}
		}
		return true
	})
	for _, name := range []string{"newWowheadNames", "serveDiagnostic", "reload", "setGame", "raidConfig", "CheckAppRelease", "CheckAddonRelease", "newAddonUpdater", "retireGame"} {
		if calls[name] {
			t.Errorf("running entrypoint reaches %s", name)
		}
	}
	for _, name := range []string{"newRetirementApp", "parseAppMode", "initAppStartup", "initAppUpdateRuntime", "runNative"} {
		if !calls[name] {
			t.Errorf("missing startup boundary %s", name)
		}
	}
	source, err := os.ReadFile("retirement_native.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CheckAppRelease", "CheckAddonRelease", "newWowheadNames", "newAddonUpdater", "net/http", "openExternal"} {
		if strings.Contains(string(source), name) {
			t.Errorf("online capability in retirement: %s", name)
		}
	}
}

func TestRetirementPlainPage(t *testing.T) {
	html, err := renderNativeShell(newRetirementApp("").snapshot(), true, "retirement-token")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Relax Coach is retired. Its addons have been removed from this PC.", "nothing left to remove", "close WoW and start Relax Coach again to remove saved settings", "default-src 'none'"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{"<button", "<a ", "<nav", "native_shell.js", "checkAppStartup", "setInterval", "fetch(", "XMLHttpRequest", "Wowhead", "More addons"} {
		if strings.Contains(html, bad) {
			t.Errorf("old/online UI remains: %s", bad)
		}
	}
	if out := os.Getenv("NATIVE_HTML_OUT"); out != "" {
		if err = os.WriteFile(out+".retirement", []byte(html), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRetirementVersion(t *testing.T) {
	if Version != "1.9.0" {
		t.Fatalf("version=%s", Version)
	}
}
