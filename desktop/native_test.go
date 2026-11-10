package main

import (
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nativeTestApp(t *testing.T) (*App, string) {
	t.Helper()
	game := filepath.Join(t.TempDir(), "_forever_")
	sv := filepath.Join(game, "WTF", "Account", "TEST", "SavedVariables", "ForeverCoach.lua")
	if err := os.MkdirAll(filepath.Dir(sv), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sv, []byte(sample), 0644); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	a.setGame(game)
	a.reload()
	return a, sv
}

func TestAppRefreshRescansAndRetainsLastGood(t *testing.T) {
	a, sv := nativeTestApp(t)
	if err := os.WriteFile(sv, []byte("ForeverCoachDB = {"), 0644); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	v := a.snapshot()
	if len(v.Chars) != 1 || v.Warning == "" {
		t.Fatalf("lost last good report: %+v", v)
	}
	// Selecting another game must not leak the previous character on parse failure.
	other := filepath.Join(t.TempDir(), "_forever_")
	if err := os.MkdirAll(filepath.Join(other, "Interface"), 0755); err != nil {
		t.Fatal(err)
	}
	a.setGame(other)
	a.refresh()
	if len(a.snapshot().Chars) != 0 {
		t.Fatal("previous game's data crossed selection")
	}
}

func TestNativeShellIsSelfContainedAndLockedDown(t *testing.T) {
	a, _ := nativeTestApp(t)
	v := a.snapshot()
	if v.Account != "TEST" {
		t.Fatalf("native startup fixture account = %q, want TEST", v.Account)
	}
	// Exercise the actual Everything else template in the embedded-JS tests.
	v.Chars[0].Findings = nil
	v.Chars[0].Counts = make(map[string]int)
	for _, area := range []string{"Build", "Gear", "Play", "Build", "Gear", "Play", "Next"} {
		v.Chars[0].Findings = append(v.Chars[0].Findings, Finding{Area: area, Text: "Synthetic filter fixture: " + area})
		v.Chars[0].Counts[area]++
	}
	if _, err := renderReportBody(v); err != nil {
		t.Fatal(err)
	}
	html, err := renderNativeShell(v, false, "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(html) > 2*1024*1024 {
		t.Fatal("shell exceeds WebView2 NavigateToString size limit")
	}
	if path := os.Getenv("NATIVE_HTML_OUT"); path != "" {
		if err := os.WriteFile(path, []byte(html), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []string{"Coach Report", "Get started", "setup-install", "Install and Update use the same verified transaction", "Willtank", "data:image/png;base64,", "Content-Security-Policy", "default-src 'none'", "connect-src 'none'", "form-action 'none'", "script-src 'nonce-", "chrome.webview.postMessage", "openExternal", "Journey-Content", "also installs <strong>RelaxQuestPlanner</strong>", "is moved into the backup in RelaxCoachBackups, not deleted"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{"fetch(", "localhost", "127.0.0.1", "onclick=", "src=\"http", "'unsafe-eval'", "action=\"/setgame\""} {
		if strings.Contains(html, bad) {
			t.Errorf("native shell contains %q", bad)
		}
	}
}

func TestNativeShellInitialReportContextEscapesValues(t *testing.T) {
	for _, tc := range []struct {
		name, game, account string
	}{
		{"ordinary", `C:\WoW\_forever_`, "TEST"},
		{"special characters", `D:\WoW "quoted" & <folder>\_forever_`, `A" data-injected="yes' & <é>`},
		{"empty selection", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rendered, err := renderNativeShell(View{Game: tc.game, Account: tc.account}, false, "test-token")
			if err != nil {
				t.Fatal(err)
			}
			want := `<div id="report-body" data-game="` + html.EscapeString(tc.game) + `" data-account="` + html.EscapeString(tc.account) + `">`
			if !strings.Contains(rendered, want) {
				t.Errorf("missing escaped initial report context %q", want)
			}
			if strings.Contains(rendered, ` data-injected="yes`) {
				t.Error("initial context escaped its HTML attribute")
			}
		})
	}
}

func TestNativeExternalURLAllowlist(t *testing.T) {
	for _, u := range []string{"https://github.com/thegamersvibe/relax-coach/releases/tag/v1.5.0", "https://www.wowhead.com/classic/guide/classes/warrior/leveling"} {
		if err := validateExternalURL(u); err != nil {
			t.Errorf("allowed URL %q: %v", u, err)
		}
	}
	for _, u := range []string{"javascript:alert(1)", "file:///C:/Windows/System32/cmd.exe", "http://github.com/x", "https://github.com.evil.test/x", "https://evil@github.com/x", "https://github.com:444/x", "https://evil.test", "https://github.com/elsewhere", "https://github.com/thegamersvibe/relax-coach%2f..", "https://www.wowhead.com/\x00x", "https://github.com\\@evil.test/x"} {
		if validateExternalURL(u) == nil {
			t.Errorf("accepted %q", u)
		}
	}
}

func TestRefreshRetainsValidEmptyReportOnMalformedSave(t *testing.T) {
	a, sv := nativeTestApp(t)
	if err := os.WriteFile(sv, []byte(`ForeverCoachDB = { ["schema"] = 1, ["chars"] = {} }`), 0644); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	if v := a.snapshot(); v.Error != "" || len(v.Chars) != 0 {
		t.Fatalf("valid empty save: %+v", v)
	}
	if err := os.WriteFile(sv, []byte(`ForeverCoachDB = {`), 0644); err != nil {
		t.Fatal(err)
	}
	a.refresh()
	if v := a.snapshot(); v.Error != "" || v.Warning == "" || len(v.Chars) != 0 {
		t.Fatalf("last good empty view lost: %+v", v)
	}
}

func TestNativeReportBodyEscapesSavedData(t *testing.T) {
	a, _ := nativeTestApp(t)
	v := a.snapshot()
	v.Chars[0].Name = `<script>alert('owned')</script>`
	v.Chars[0].Findings[0].Text = `<img src=x onerror=alert(1)>`
	body, err := renderReportBody(v)
	if err != nil {
		t.Fatal(err)
	}
	for _, unsafe := range []string{"<script", "<img src=x", "fetch(", "onclick=", "<html"} {
		if strings.Contains(body, unsafe) {
			t.Fatalf("unsafe or standalone content: %q", unsafe)
		}
	}
	for _, expected := range []string{"&lt;script&gt;", "&lt;img", "Recent deaths", "Gear for level 20"} {
		if !strings.Contains(body, expected) {
			t.Errorf("missing %q", expected)
		}
	}
}
