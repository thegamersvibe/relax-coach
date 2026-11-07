package main

import (
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHelperVersionAndLegacyStorage(t *testing.T) {
	if Version != "1.4.4" {
		t.Errorf("helper version = %q, want 1.4.4", Version)
	}
	game := t.TempDir()
	sv := filepath.Join(game, "WTF", "Account", "TEST", "SavedVariables", "ForeverCoach.lua")
	addon := filepath.Join(game, "Interface", "AddOns", "ForeverCoach")
	for _, dir := range []string{filepath.Dir(sv), addon} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(sv, []byte(sample), 0o644); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	a.setGame(game)
	a.reload()
	view := a.snapshot()
	if a.addonDir != addon || a.svPath != sv || view.Error != "" || len(view.Chars) != 1 || view.Chars[0].Name != "Willtank" {
		t.Fatalf("legacy install not loaded: addon=%q sv=%q view=%+v", a.addonDir, a.svPath, view)
	}
	unchanged, err := os.ReadFile(sv)
	if err != nil || string(unchanged) != sample {
		t.Fatalf("helper changed the save: %v", err)
	}
}

func TestRememberedFolderKeepsLegacyConfigPath(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"APPDATA", "XDG_CONFIG_HOME", "HOME"} {
		t.Setenv(name, root)
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "ForeverCoach", "config.json")
	if configPath() != want {
		t.Fatalf("config migrated: %q, want %q", configPath(), want)
	}
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte(`{"game":"legacy-folder"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if savedGame() != "legacy-folder" {
		t.Fatal("previously remembered folder was lost")
	}
	saveGame("new-folder")
	if savedGame() != "new-folder" {
		t.Fatal("remembered folder no longer updates")
	}
}

func TestHelperErrorsUseRelaxBranding(t *testing.T) {
	dir := t.TempDir()
	a := &App{game: dir, addonDir: dir}
	a.reload()
	if !strings.Contains(a.snapshot().Error, "No Relax Coach data yet") {
		t.Errorf("missing-save error = %q", a.snapshot().Error)
	}
	sv := filepath.Join(dir, "ForeverCoach.lua")
	if err := os.WriteFile(sv, []byte(strings.Replace(sample, `["schema"] = 1`, `["schema"] = 9`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	a.svPath = sv
	a.reload()
	if !strings.Contains(a.snapshot().Error, "newer Relax Coach addon") {
		t.Errorf("newer-save error = %q", a.snapshot().Error)
	}
}

func TestReportRelaxBranding(t *testing.T) {
	tpl := template.Must(template.New("report.html").Funcs(funcs).ParseFS(assets, "report.html"))
	vars, err := ParseSavedVariables(sample)
	if err != nil {
		t.Fatal(err)
	}
	chars := buildChars(M(vars["ForeverCoachDB"]))
	chars[0].Guide = "Redridge route"
	chars[0].GuideStep = "42"
	for _, tc := range []struct {
		name string
		view View
	}{
		{"no game", View{NoGame: true, AppVersion: Version}},
		{"legacy save", View{Chars: chars, AppVersion: Version, RulesVersion: "1.3.1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			if err := tpl.Execute(&out, tc.view); err != nil {
				t.Fatal(err)
			}
			html := out.String()
			for _, want := range []string{
				"<title>Relax Coach</title>", "<h1>Relax Coach</h1>",
				"Diagnostic report", "Relax Coach · private bundle",
				"Update Addon delivers ForeverCoach, Journey and guide content together", "Settings and saves are preserved",
				"TheGamersVibe", "ForeverCoach.lua", "WoW Forever leveling report",
				`action="/setgame"`, `fetch('/refresh',{method:'POST'})`,
				`src="/logo.png"`, "app " + Version,
			} {
				if !strings.Contains(html, want) {
					t.Errorf("rendered report missing %q", want)
				}
			}
			for _, unwanted := range []string{"Forever Coach", "https://github.com/", "releases/latest"} {
				if strings.Contains(html, unwanted) {
					t.Errorf("report advertises upstream branding/download: %q", unwanted)
				}
			}
			if len(tc.view.Chars) > 0 {
				for _, want := range []string{`title="Relax Journey step 42"`, "Relax Journey <b>Redridge route</b>", "2 unspent talent points", "Recent deaths", "Gear for level 20"} {
					if !strings.Contains(html, want) {
						t.Errorf("legacy-save report missing %q", want)
					}
				}
			} else if strings.Contains(html, "Relax Journey step") {
				t.Error("guide chip rendered without guide context")
			}
		})
	}
}
