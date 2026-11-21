package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/fixture"
)

func raidOpeningFixture(t *testing.T) (string, *App, *nativeBridge) {
	t.Helper()
	accountConfigEnv(t)
	game := filepath.Join(t.TempDir(), "_forever_")
	accountSave(t, game, "A#1", "Alpha")
	raidRankSave(t, game, "A#1", officerSave())
	a, b := raidLockApp(t, game, raidTestClock)
	return game, a, b
}

func raidOpeningLog(t *testing.T, path string, instances ...int64) {
	t.Helper()
	var raw bytes.Buffer
	at := time.Date(2026, 12, 9, 20, 0, 0, 0, time.UTC)
	for _, instance := range instances {
		fixture.Write(&raw, at, 3, "ZONE_CHANGE", fixture.Number(instance), fixture.Text("Training Instance"), fixture.Number(1))
		fixture.Write(&raw, at, 3, "ENCOUNTER_START", fixture.Number(42), fixture.Text("Training Colossus"), fixture.Number(1), fixture.Number(10), fixture.Number(instance))
		at = at.Add(time.Minute)
		fixture.Write(&raw, at, 3, "ENCOUNTER_END", fixture.Number(42), fixture.Text("Training Colossus"), fixture.Number(1), fixture.Number(10), fixture.Number(1), fixture.Number(60000))
		at = at.Add(time.Minute)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

func raidCallHTML(t *testing.T, b *nativeBridge, method string, args ...string) string {
	t.Helper()
	var result any
	var err error
	if len(args) == 0 {
		result, err = b.Call(method, nil)
	} else {
		result, err = b.Call(method, nativeArgs(args[0]))
	}
	if err != nil {
		t.Fatal(err)
	}
	text, ok := result.(string)
	if !ok {
		t.Fatalf("not rendered HTML: %T", result)
	}
	return text
}

func TestRaidPageOpenEmbedded(t *testing.T) {
	game, _, b := raidOpeningFixture(t)
	log := filepath.Join(game, "Logs", "WoWCombatLog-fixture.txt")
	raidOpeningLog(t, log, 99, 88)
	before, _ := os.ReadFile(log)
	html := raidCallHTML(t, b, "raidOpen", "")
	c, err := raidConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Unlisted instance 99", "Scoring " + c.Scoring.Version, `id="raid-night-select"`, `data-raid-officer`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if out := os.Getenv("NATIVE_HTML_OUT"); out != "" {
		if err := os.WriteFile(out+".raid-opened", []byte(html), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(html, "not available yet") {
		t.Fatal("unavailable notice remains")
	}
	html = raidCallHTML(t, b, "raidNight", "1")
	if !strings.Contains(html, "Raids &gt; <bdi>Unlisted instance 88") {
		t.Fatal("night selection lost")
	}
	html = raidCallHTML(t, b, "raidNight", "-1")
	if !strings.Contains(html, "invalid night selection") {
		t.Fatal("invalid index accepted")
	}
	// Manual path bypasses discovery and opening is read-only.
	manual := filepath.Join(t.TempDir(), "selected.txt")
	raidOpeningLog(t, manual, 77)
	if !strings.Contains(raidCallHTML(t, b, "raidOpen", manual), "Unlisted instance 77") {
		t.Fatal("manual path ignored")
	}
	after, _ := os.ReadFile(log)
	if !bytes.Equal(before, after) {
		t.Fatal("log changed")
	}
}

func TestRaidPageConfigFailure(t *testing.T) {
	_, a, old := raidOpeningFixture(t)
	old.Close()
	loads := 0
	picks := 0
	b := newNativeBridge(a, true, nativeServices{raidConfig: func() (cfg.Config, error) { loads++; return cfg.Config{}, errors.New("scoring.json: <invalid>") }, chooseFile: func(context.Context, string) (string, error) { picks++; return "", nil }})
	t.Cleanup(b.Close)
	if loads != 1 {
		t.Fatal("config not loaded at bridge startup")
	}
	for _, method := range []string{"raidPage", "raidChooseLog", "raidChooseOfficer"} {
		html := raidCallHTML(t, b, method)
		if !strings.Contains(html, "scoring.json: &lt;invalid&gt;") || strings.Contains(html, `id="raid-ranked"`) || strings.Contains(html, `id="raid-open"`) {
			t.Fatal("config error hidden or night/actions rendered")
		}
	}
	if loads != 1 || picks != 0 {
		t.Fatal("config retried or picker opened after failure")
	}
}

func TestRaidPageOfficerPicker(t *testing.T) {
	game, _, b := raidOpeningFixture(t)
	log := filepath.Join(t.TempDir(), "selected.txt")
	raidOpeningLog(t, log, 99)
	prediction := filepath.Join(t.TempDir(), "officer.json")
	picks := 0
	b.services.chooseFile = func(_ context.Context, kind string) (string, error) {
		picks++
		if kind == "log" {
			return log, nil
		}
		return prediction, nil
	}
	raidCallHTML(t, b, "raidChooseLog")
	for _, name := range []string{"First Officer", "Second Officer"} {
		body := []byte("\xef\xbb\xbf" + `{"version":1,"officer":"` + name + `","top":[" Alia-PvP-EU "],"bottom":["Bram-PvP-EU"]}`)
		if err := os.WriteFile(prediction, body, 0600); err != nil {
			t.Fatal(err)
		}
		html := raidCallHTML(t, b, "raidChooseOfficer")
		if !strings.Contains(html, name) {
			t.Fatal("chosen file not parsed/rendered anew")
		}
		after, _ := os.ReadFile(prediction)
		if !bytes.Equal(body, after) {
			t.Fatal("prediction changed")
		}
	}
	os.WriteFile(prediction, []byte(`{"version":2}`), 0600)
	html := raidCallHTML(t, b, "raidOfficerFile", prediction)
	if !strings.Contains(html, "Officer file:") || strings.Contains(html, "Second Officer") || !strings.Contains(html, `id="raid-ranked"`) {
		t.Fatal("parser refusal hidden or old comparison retained")
	}
	if strings.Contains(raidCallHTML(t, b, "raidPage"), "Second Officer") {
		t.Fatal("comparison persisted")
	}
	b.services.chooseFile = func(context.Context, string) (string, error) { return "", nil }
	if text, err := b.Call("raidChooseOfficer", nil); err != nil || text != nil {
		t.Fatal("cancel changed the page")
	}
	if picks != 3 {
		t.Fatal("picker path bypassed")
	}
	data, _ := os.ReadFile(filepath.Join(game, "WTF", "Account", "A#1", "SavedVariables", "RelaxRaid.lua"))
	if string(data) != officerSave() {
		t.Fatal("save changed")
	}
}

func TestRaidPageOpeningAdmission(t *testing.T) {
	game, _, b := raidOpeningFixture(t)
	picks := 0
	b.services.chooseFile = func(context.Context, string) (string, error) { picks++; return "", nil }
	raidRankSave(t, game, "A#1", strings.Replace(officerSave(), `"officer"`, `"member"`, 1))
	for _, method := range []string{"raidOpen", "raidNight", "raidOfficerFile", "raidChooseLog", "raidChooseOfficer"} {
		args := nativeArgs("missing")
		if strings.Contains(method, "Choose") {
			args = nil
		}
		got, err := b.Call(method, args)
		if err == nil || got != nil || !strings.Contains(err.Error(), "locked") {
			t.Fatalf("%s bypassed admission: %v", method, err)
		}
	}
	if picks != 0 {
		t.Fatal("locked chooser invoked")
	}
}

func TestRaidPagePickerContextChange(t *testing.T) {
	game, _, b := raidOpeningFixture(t)
	accountSave(t, game, "B#2", "Beta")
	raidRankSave(t, game, "B#2", officerSave())
	log := filepath.Join(t.TempDir(), "selected.txt")
	raidOpeningLog(t, log, 99)
	started, release := make(chan struct{}), make(chan struct{})
	b.services.chooseFile = func(context.Context, string) (string, error) { close(started); <-release; return log, nil }
	done := make(chan error, 1)
	go func() {
		got, err := b.Call("raidChooseLog", nil)
		if got != nil {
			err = errors.New("stale picker returned content")
		}
		done <- err
	}()
	<-started
	if _, err := b.Call("selectAccount", nativeArgs("B#2")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err == nil || !strings.Contains(err.Error(), "selection changed") {
		t.Fatalf("stale chooser accepted: %v", err)
	}
}
