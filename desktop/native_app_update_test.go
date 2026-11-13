package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNativeAppWorksWithoutGameAndOwnsOffer(t *testing.T) {
	a := &App{}
	a.setGame("")
	offer := AppReleaseInfo{Version: "1.5.0", Available: true, AssetURL: "private-exe-url", ChecksumURL: "private-checksum"}
	applied := 0
	b := newNativeBridge(a, false, nativeServices{checkApp: func(ctx context.Context, current string) (AppReleaseInfo, error) {
		if current != Version {
			t.Errorf("not backend Version: %s", current)
		}
		return offer, nil
	}, applyApp: func(ctx context.Context, r AppReleaseInfo) (AppUpdateResult, error) {
		applied++
		if r != offer {
			t.Error("untrusted release used")
		}
		return AppUpdateResult{RestartReady: true}, nil
	}})
	defer b.Close()
	if _, err := b.Call("applyApp", nativeArgs("1.5.0")); err == nil {
		t.Fatal("unchecked app installed")
	}
	value, err := b.Call("checkApp", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(value)
	if strings.Contains(string(raw), "private-") {
		t.Fatal("release capability leaked")
	}
	if _, err = b.Call("applyApp", nativeArgs("1.6.0")); err == nil {
		t.Fatal("wrong confirmation")
	}
	if _, err = b.Call("applyApp", nativeArgs("1.5.0")); err == nil {
		t.Fatal("stale confirmation survived failure")
	}
	if _, err = b.Call("checkApp", nil); err != nil {
		t.Fatal(err)
	}
	if _, err = b.Call("applyApp", nativeArgs(map[string]any{"Version": "1.5.0"})); err == nil {
		t.Fatal("object accepted")
	}
	result, err := b.Call("applyApp", nativeArgs("1.5.0"))
	if err != nil || !result.(AppUpdateResult).RestartReady || applied != 1 {
		t.Fatalf("handoff %v %v", result, err)
	}
	if _, err = b.Call("checkApp", nil); err == nil {
		t.Fatal("new work after restart handoff")
	}
}
func TestNativeAppFailedCheckAndInstallInvalidateOnlyAppOffer(t *testing.T) {
	a, _ := nativeTestApp(t)
	nativeInstalled(t, a, "1.3.1")
	fail := false
	b := newNativeBridge(a, false, nativeServices{check: func(context.Context, string) (AddonReleaseInfo, error) {
		return AddonReleaseInfo{Version: "1.4.0", Available: true}, nil
	}, checkApp: func(context.Context, string) (AppReleaseInfo, error) {
		if fail {
			return AppReleaseInfo{}, errors.New("offline")
		}
		return AppReleaseInfo{Version: "1.5.0", Available: true}, nil
	}, applyApp: func(context.Context, AppReleaseInfo) (AppUpdateResult, error) {
		return AppUpdateResult{}, errors.New("worker failed")
	}, apply: func(context.Context, string, AddonReleaseInfo) (AddonUpdateResult, error) {
		return AddonUpdateResult{}, nil
	}})
	defer b.Close()
	if _, err := b.Call("checkAddon", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call("checkApp", nil); err != nil {
		t.Fatal(err)
	}
	fail = true
	if _, err := b.Call("checkApp", nil); err == nil {
		t.Fatal("failed check succeeded")
	}
	if _, err := b.Call("applyApp", nativeArgs("1.5.0")); err == nil {
		t.Fatal("failed check retained offer")
	}
	fail = false
	b.Call("checkApp", nil)
	if _, err := b.Call("applyApp", nativeArgs("1.5.0")); err == nil {
		t.Fatal("failed worker succeeded")
	}
	if _, err := b.Call("applyApp", nativeArgs("1.5.0")); err == nil {
		t.Fatal("failed worker retained offer")
	}
	if _, err := b.Call("applyAddon", nativeArgs("1.4.0")); err != nil {
		t.Fatalf("app check disturbed addon offer: %v", err)
	}
}
func TestNativeAppDisableCancelAndHealthAdmission(t *testing.T) {
	a := &App{}
	a.setGame("")
	for _, disabled := range []bool{true, false} {
		entered := make(chan struct{})
		returned := make(chan struct{})
		b := newNativeBridge(a, disabled, nativeServices{checkApp: func(ctx context.Context, _ string) (AppReleaseInfo, error) {
			close(entered)
			<-ctx.Done()
			close(returned)
			return AppReleaseInfo{}, ctx.Err()
		}})
		if disabled {
			if _, err := b.Call("checkApp", nil); err == nil {
				t.Fatal("no-update ignored")
			}
			if _, err := b.Call("applyApp", nativeArgs("1.5.0")); err == nil {
				t.Fatal("no-update apply")
			}
			b.Close()
			continue
		}
		done := make(chan struct{})
		go func() { defer close(done); b.Call("checkApp", nil) }()
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("app check not admitted")
		}
		if _, err := b.Call("checkApp", nil); err == nil {
			t.Fatal("concurrent app work")
		}
		b.Close()
		<-done
		select {
		case <-returned:
		default:
			t.Fatal("close did not join canceled app check")
		}
	}
	ready := 0
	b := newNativeBridge(a, true, nativeServices{ready: func(context.Context) error { ready++; return nil }})
	defer b.Close()
	if _, err := b.Call("appReady", nil); err == nil {
		t.Fatal("health before initial report RPC")
	}
	if _, err := b.Call("reportRefresh", nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := b.Call("appReady", nil); err != nil {
			t.Fatal(err)
		}
	}
	if ready != 1 {
		t.Fatalf("health sent %d times", ready)
	}
}

func TestNativeAppMethodHasExplicitAdmission(t *testing.T) {
	b := newNativeBridge(&App{}, true, nativeServices{})
	defer b.Close()
	for _, method := range []string{"checkApp", "checkAppStartup", "appReady"} {
		if _, err := b.Call(method, nativeArgs("unexpected")); err == nil || !strings.Contains(err.Error(), "no arguments") {
			t.Fatalf("%s must explicitly reject parameters, got %v", method, err)
		}
	}
}

func TestNativeStartupAppCheckUsesShortTimeout(t *testing.T) {
	a := &App{}
	a.setGame("")
	budgets := map[string]time.Duration{}
	var method string
	b := newNativeBridge(a, false, nativeServices{checkApp: func(ctx context.Context, _ string) (AppReleaseInfo, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("app check has no deadline")
		}
		budgets[method] = time.Until(deadline)
		return AppReleaseInfo{}, nil
	}})
	defer b.Close()
	for _, method = range []string{"checkAppStartup", "checkApp"} {
		if _, err := b.Call(method, nil); err != nil {
			t.Fatalf("%s: %v", method, err)
		}
	}
	if d := budgets["checkAppStartup"]; d <= 0 || d > startupAppCheckTimeout {
		t.Fatalf("launch check budget %v, want <= %v", d, startupAppCheckTimeout)
	}
	if d := budgets["checkApp"]; d <= startupAppCheckTimeout || d > appCheckTimeout {
		t.Fatalf("manual check budget %v, want the full %v", d, appCheckTimeout)
	}
}
