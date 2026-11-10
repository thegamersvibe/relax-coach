package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestNativeAppRestartOnlyAfterSuccessfulResponseDispatch(t *testing.T) {
	yes := AppUpdateResult{RestartReady: true}
	for _, tc := range []struct {
		method           string
		value            any
		callErr, postErr error
		want             bool
	}{
		{"applyApp", yes, nil, nil, true}, {"applyApp", AppUpdateResult{}, nil, nil, false}, {"applyApp", yes, errors.New("failed"), nil, false}, {"applyApp", yes, nil, errors.New("post failed"), false}, {"checkApp", yes, nil, nil, false},
	} {
		if got := nativeAppRestartReady(tc.method, tc.value, tc.callErr, tc.postErr); got != tc.want {
			t.Fatalf("restart dispatch: %+v", tc)
		}
	}
	source, err := os.ReadFile("native_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	dispatch := strings.Index(text, "text, _ := windows.UTF16PtrFromString(string(data))")
	if dispatch < 0 || !strings.Contains(text[dispatch:], "nativeAppRestartReady(req.Method") || !strings.Contains(text[dispatch:], "nativeWMClose") {
		t.Fatal("restart is not wired into native UI-thread response dispatch")
	}
}

// This is a source-bound startup-flag check, not a Windows visibility test.
// The Windows companion test exercises the child process's actual STARTUPINFO.
func TestAppOwnedGUIProcessesAreNotForcedHidden(t *testing.T) {
	b, err := os.ReadFile("app_update_process_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "HideWindow: true") {
		t.Fatal("owned GUI launch forces STARTF_USESHOWWINDOW/SW_HIDE; the new or restored app's first ShowWindow cannot display it")
	}
}

func TestAppMainRoutesStrictModesBeforeNormalStartup(t *testing.T) {
	b, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`var Version = "1.5.0"`, "parseAppMode(os.Args[1:])", "appPlatformWorker(", "initAppUpdateRuntime(", "initAppStartup("} {
		if !strings.Contains(s, want) {
			t.Errorf("missing lifecycle integration: %s", want)
		}
	}
}
