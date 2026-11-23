package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestRetirementAppUpdateCheckAndApplyOnly(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "enabled", true: "no-update"}[disabled], func(t *testing.T) {
			requests, applied := 0, 0
			u := appTestUpdater(t, func(w http.ResponseWriter, r *http.Request) {
				requests++
				if r.URL.Path != "/repos/thegamersvibe/relax-coach/releases/latest" {
					t.Errorf("unexpected network path: %s", r.URL.Path)
				}
				json.NewEncoder(w).Encode(appTestRelease("v1.9.1"))
			})
			b := newNativeBridge(newRetirementApp(""), disabled, nativeServices{
				ready: func(context.Context) error { return nil },
				checkApp: func(ctx context.Context, current string) (AppReleaseInfo, error) {
					if current != "1.9.0" {
						t.Fatal(current)
					}
					deadline, ok := ctx.Deadline()
					if !ok || time.Until(deadline) > startupAppCheckTimeout {
						t.Fatal("startup timeout lost")
					}
					return u.check(ctx, current)
				},
				applyApp: func(_ context.Context, release AppReleaseInfo) (AppUpdateResult, error) {
					applied++
					if release.Version != "1.9.1" || release.AssetName != "RelaxCoach.exe" {
						t.Fatalf("untrusted offer: %+v", release)
					}
					return AppUpdateResult{RestartReady: true}, nil
				},
				check: func(context.Context, string) (AddonReleaseInfo, error) {
					t.Fatal("addon check reached")
					return AddonReleaseInfo{}, nil
				},
			})
			defer b.Close()
			b.Call("reportRefresh", nil)
			if _, err := b.Call("appReady", nil); err != nil {
				t.Fatal(err)
			}
			value, err := b.Call("checkAppStartup", nil)
			if disabled {
				if err == nil || requests != 0 {
					t.Fatalf("no-update reached network: %v %d", err, requests)
				}
				if _, err = b.Call("applyApp", nativeArgs("1.9.1")); err == nil {
					t.Fatal("disabled apply")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			offer := value.(nativeOffer)
			if !offer.Available || offer.Version != "1.9.1" || requests != 1 {
				t.Fatalf("offer: %+v requests=%d", offer, requests)
			}
			raw, _ := json.Marshal(value)
			if strings.Contains(string(raw), "browser_download_url") || strings.Contains(string(raw), "/download/") {
				t.Fatal("download capability exposed")
			}
			if applied != 0 {
				t.Fatal("automatic apply")
			}
			for _, arg := range []any{nil, map[string]string{"Version": "1.9.1"}, "1.9.1\x00"} {
				if _, err = b.Call("applyApp", nativeArgs(arg)); err == nil {
					t.Fatal("invalid apply arguments")
				}
			}
			value, err = b.Call("applyApp", nativeArgs("1.9.1"))
			if err != nil || applied != 1 || !nativeAppRestartReady("applyApp", value, nil, nil) {
				t.Fatalf("handoff: %v %v %d", value, err, applied)
			}
		})
	}
}
func TestRetirementNoUpdateReachesNativeShell(t *testing.T) {
	// The startup flag must be honored, not accepted and then ignored.
	source := readRetirementSource(t, "main.go")
	if !strings.Contains(source, `noUpdate := flag.Bool("no-update", false`) || !strings.Contains(source, "runNative(app, *noUpdate)") {
		t.Fatal("entrypoint does not pass -no-update")
	}
	for _, disabled := range []bool{false, true} {
		html, err := renderNativeShell(newRetirementApp("").snapshot(), disabled, "token")
		if err != nil {
			t.Fatal(err)
		}
		want := `data-updates-disabled="false"`
		if disabled {
			want = `data-updates-disabled="true"`
		}
		if !strings.Contains(html, want) {
			t.Fatalf("missing %s", want)
		}
	}
}
