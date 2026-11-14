package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func nativeInstalled(t *testing.T, a *App, version string) {
	t.Helper()
	target := filepath.Join(a.snapshot().Game, "Interface", "AddOns", "ForeverCoach", "ForeverCoach.toc")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("## Version: "+version+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
}
func nativeArgs(v ...any) []json.RawMessage {
	args := make([]json.RawMessage, len(v))
	for i, x := range v {
		args[i], _ = json.Marshal(x)
	}
	return args
}
func TestNativeBridgeChecksInstalledAddonAndOwnsRelease(t *testing.T) {
	a, _ := nativeTestApp(t)
	nativeInstalled(t, a, "1.3.1")
	release := AddonReleaseInfo{Version: "1.4.0", Name: "Release", AssetURL: "trusted-asset", ChecksumURL: "trusted-checksum", Available: true}
	var applied AddonReleaseInfo
	b := newNativeBridge(a, false, nativeServices{
		check: func(ctx context.Context, current string) (AddonReleaseInfo, error) {
			if current != "1.3.1" {
				t.Errorf("used helper version %q", current)
			}
			return release, nil
		},
		apply: func(ctx context.Context, game string, r AddonReleaseInfo) (AddonUpdateResult, error) {
			applied = r
			return AddonUpdateResult{Version: r.Version}, nil
		},
	})
	defer b.Close()
	if _, err := b.Call("applyAddon", nativeArgs("1.4.0")); err == nil {
		t.Fatal("apply without check")
	}
	value, err := b.Call("checkAddon", nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(value)
	if strings.Contains(string(encoded), "trusted-") {
		t.Fatal("download capabilities leaked to JS")
	}
	if _, err = b.Call("applyAddon", nativeArgs("1.4.1")); err == nil {
		t.Fatal("wrong expected version")
	}
	if _, err = b.Call("applyAddon", nativeArgs(map[string]any{"Version": "1.4.0", "AssetURL": "evil"})); err == nil {
		t.Fatal("object accepted instead of version confirmation")
	}
	if _, err = b.Call("applyAddon", nativeArgs("1.4.0")); err != nil {
		t.Fatal(err)
	}
	if applied != release {
		t.Fatalf("stored release was not applied: %+v", applied)
	}
	if _, err = b.Call("applyAddon", nativeArgs("1.4.0")); err == nil {
		t.Fatal("release replay accepted")
	}
}
func TestNativeBridgeFolderChangeAndFailedCheckInvalidateOffer(t *testing.T) {
	a, _ := nativeTestApp(t)
	failures := false
	for _, name := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME"} {
		t.Setenv(name, t.TempDir())
	}
	b := newNativeBridge(a, false, nativeServices{check: func(context.Context, string) (AddonReleaseInfo, error) {
		if failures {
			return AddonReleaseInfo{}, errors.New("offline")
		}
		return AddonReleaseInfo{Version: "1.4.0", Available: true}, nil
	}})
	defer b.Close()
	if _, err := b.Call("checkAddon", nil); err != nil {
		t.Fatal(err)
	}
	other, _ := nativeTestApp(t)
	if _, err := b.Call("setGame", nativeArgs(other.snapshot().Game)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call("applyAddon", nativeArgs("1.4.0")); err == nil {
		t.Fatal("offer survived folder change")
	}
	if _, err := b.Call("checkAddon", nil); err != nil {
		t.Fatal(err)
	}
	failures = true
	if _, err := b.Call("checkAddon", nil); err == nil {
		t.Fatal("offline check should fail")
	}
	if _, err := b.Call("applyAddon", nativeArgs("1.4.0")); err == nil {
		t.Fatal("stale offer survived failed check")
	}
}
func TestNativeBridgeCloseCancelsAndJoinsWork(t *testing.T) {
	a, _ := nativeTestApp(t)
	entered, exited := make(chan struct{}), make(chan struct{})
	b := newNativeBridge(a, false, nativeServices{check: func(ctx context.Context, _ string) (AddonReleaseInfo, error) {
		close(entered)
		<-ctx.Done()
		defer close(exited)
		return AddonReleaseInfo{}, ctx.Err()
	}})
	done := make(chan struct{})
	go func() { defer close(done); _, _ = b.Call("checkAddon", nil) }()
	<-entered
	if _, err := b.Call("setGame", nativeArgs(a.snapshot().Game)); err == nil {
		t.Fatal("folder changed during update work")
	}
	if _, err := b.Call("checkAddon", nil); err == nil {
		t.Fatal("duplicate update work admitted")
	}
	b.Close()
	select {
	case <-exited:
	default:
		t.Fatal("Close returned before worker exit")
	}
	<-done
	if _, err := b.Call("reportRefresh", nil); err == nil {
		t.Fatal("closed bridge accepted call")
	}
	b.Close() // idempotent
}
func TestNativeBridgeConcurrentReportsAndNoUpdate(t *testing.T) {
	a, _ := nativeTestApp(t)
	b := newNativeBridge(a, true, nativeServices{check: func(context.Context, string) (AddonReleaseInfo, error) {
		t.Error("no-update invoked network")
		return AddonReleaseInfo{}, nil
	}})
	defer b.Close()
	if _, err := b.Call("checkAddon", nil); err == nil {
		t.Fatal("-no-update ignored")
	}
	if _, err := b.Call("applyAddon", nativeArgs("1.4.0")); err == nil {
		t.Fatal("-no-update ignored on apply")
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if _, err := b.Call("reportRefresh", nil); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
	// Passage of time/minimization has no lifecycle API or heartbeat to expire.
	if b.ctx.Err() != nil {
		t.Fatal("idle bridge is not live")
	}
}
func TestNativeRPCRejectsUntrustedCalls(t *testing.T) {
	for _, raw := range []string{`{}`, `{"id":1,"method":"reportRefresh","params":[],"token":"wrong"}`, `{"id":-1,"method":"reportRefresh","token":"secret"}`, `{"id":1,"method":"exec","params":[],"token":"secret","extra":1}`, strings.Repeat("x", 16385)} {
		if _, err := decodeNativeRequest(raw, "secret"); err == nil {
			t.Errorf("accepted %q", raw[:min(100, len(raw))])
		}
	}
	request, err := decodeNativeRequest(`{"id":1,"method":"reportRefresh","params":[],"token":"secret"}`, "secret")
	if err != nil || request.Method != "reportRefresh" {
		t.Fatalf("valid request: %+v %v", request, err)
	}
	a, _ := nativeTestApp(t)
	opened := false
	b := newNativeBridge(a, false, nativeServices{open: func(string) error { opened = true; return nil }})
	defer b.Close()
	if _, err := b.Call("exec", nativeArgs("anything")); err == nil {
		t.Fatal("arbitrary method admitted")
	}
	if _, err := b.Call("reportRefresh", nativeArgs("unexpected")); err == nil {
		t.Fatal("excess parameters admitted")
	}
	if _, err := b.Call("openExternal", nativeArgs("file:///C:/Windows/cmd.exe")); err == nil || opened {
		t.Fatal("unsafe external opened")
	}
	if _, err := b.Call("openExternal", nativeArgs("https://github.com/"+Repo+"/releases")); err != nil || !opened {
		t.Fatalf("allowed external: %v", err)
	}
}
func TestNativeLifecycleDoesNotExpire(t *testing.T) {
	a, _ := nativeTestApp(t)
	b := newNativeBridge(a, true, nativeServices{})
	defer b.Close()
	// Window-owned lifetime, not a save timestamp or last-ping deadline.
	a.mu.Lock()
	a.view.Loaded = time.Now().Add(-24 * time.Hour)
	a.mu.Unlock()
	if _, err := b.Call("reportRefresh", nil); err != nil {
		t.Fatal(err)
	}
}

func TestNativeBridgeTextArgumentsMustBeJSONStrings(t *testing.T) {
	a, _ := nativeTestApp(t)
	b := newNativeBridge(a, false, nativeServices{})
	defer b.Close()
	for _, method := range []string{"setGame", "selectAccount", "applyAddon", "applyApp", "openExternal", "checkExtra", "applyExtra", "repairAppLinks"} {
		for _, raw := range []string{"null", " null ", "0", "true", "[]", "{}"} {
			if _, err := b.Call(method, []json.RawMessage{json.RawMessage(raw)}); err == nil || err.Error() != "expected one text argument" {
				t.Fatalf("%s(%s): %v", method, raw, err)
			}
		}
	}
	// A real (empty) string still reaches the method.
	if _, err := b.Call("repairAppLinks", []json.RawMessage{json.RawMessage(` ""`)}); err != nil && err.Error() == "expected one text argument" {
		t.Fatalf("empty string refused: %v", err)
	}
}
