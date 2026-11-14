package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type placementFixture struct {
	root, exe, home, desktop, programs string
	shortcuts                          []string
	launched                           []string
}

func newPlacementFixture(t *testing.T) *placementFixture {
	t.Helper()
	f := &placementFixture{root: t.TempDir()}
	f.exe = filepath.Join(f.root, "Downloads", "RelaxCoach.exe")
	f.home = filepath.Join(f.root, "Users", "player")
	f.desktop = filepath.Join(f.home, "Desktop")
	f.programs = filepath.Join(f.home, "AppData", "Roaming", "Microsoft", "Windows", "Start Menu", "Programs")
	for _, d := range []string{filepath.Dir(f.exe), f.desktop, f.programs} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(f.exe, append([]byte("MZ"), bytes.Repeat([]byte{7}, 4096)...), 0755); err != nil {
		t.Fatal(err)
	}
	return f
}

// The fake shortcut is a file holding its target; reading it back returns that.
// shortcuts records "<folder>->target@workdir" per write (writes go to a
// temporary name in that folder first).
func (f *placementFixture) env(shortcutErr error) appPlacementEnv {
	return appPlacementEnv{exe: f.exe, home: f.home, desktop: f.desktop, programs: f.programs,
		shortcut: func(link, target, dir string) error {
			f.shortcuts = append(f.shortcuts, filepath.Dir(link)+"->"+target+"@"+dir)
			if shortcutErr != nil {
				return shortcutErr
			}
			return os.WriteFile(link, []byte(target), 0644)
		},
		readShortcut: func(link string) (string, error) {
			b, err := os.ReadFile(link)
			return string(b), err
		},
		launch: func(target string, args []string) error {
			f.launched = append(f.launched, target+" "+strings.Join(args, " "))
			return nil
		}}
}

func (f *placementFixture) link(where string) string {
	if where == "startmenu" {
		return filepath.Join(f.programs, "Relax Coach.lnk")
	}
	return filepath.Join(f.desktop, "Relax Coach.lnk")
}

func linkStates(p AppPlacement) string {
	var out []string
	for _, l := range p.Links {
		out = append(out, l.Where+"="+l.State)
	}
	return strings.Join(out, ",")
}

// noTempLinks fails if a temporary shortcut is left in either folder.
func noTempLinks(t *testing.T, f *placementFixture) {
	t.Helper()
	for _, d := range []string{f.desktop, f.programs} {
		entries, _ := os.ReadDir(d)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".Relax Coach.") {
				t.Fatalf("temporary shortcut left behind: %s", filepath.Join(d, e.Name()))
			}
		}
	}
}

func TestAppPlacementOfferCopyAndShortcut(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	p := e.placement()
	target := filepath.Join(f.home, "RelaxCoach", "RelaxCoach.exe")
	if p.State != "offer" || p.Target != target || p.Shortcut != filepath.Join(f.desktop, "Relax Coach.lnk") {
		t.Fatalf("placement %+v", p)
	}
	got, err := e.install()
	if err != nil {
		t.Fatal(err)
	}
	want, _ := os.ReadFile(f.exe)
	copied, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(copied, want) {
		t.Fatalf("copy differs: %v", err)
	}
	if got.State != "copied" || !strings.Contains(got.Message, "shortcut was added") || len(f.shortcuts) != 2 ||
		f.shortcuts[0] != f.desktop+"->"+target+"@"+filepath.Dir(target) || f.shortcuts[1] != f.programs+"->"+target+"@"+filepath.Dir(target) {
		t.Fatalf("result %+v shortcuts %v", got, f.shortcuts)
	}
	for _, where := range []string{"desktop", "startmenu"} {
		if b, err := os.ReadFile(f.link(where)); err != nil || string(b) != target {
			t.Fatalf("%s link: %q %v", where, b, err)
		}
	}
	if linkStates(got) != "desktop=ok,startmenu=ok" || got.StartMenu != f.link("startmenu") {
		t.Fatalf("links %+v", got)
	}
	noTempLinks(t, f)
	// The copy is now the installed one: running it offers nothing.
	e.exe = target
	if p := e.placement(); p.State != "installed" {
		t.Fatalf("running the installed copy: %+v", p)
	}
	// A second downloaded copy points at the installed one and never replaces it.
	e.exe = f.exe
	if p := e.placement(); p.State != "other-copy" || !strings.Contains(p.Message, "already installed") {
		t.Fatalf("second copy: %+v", p)
	}
	if _, err := e.install(); err == nil {
		t.Fatal("replaced an installed copy")
	}
}

func TestAppPlacementShortcutProblemsKeepTheCopy(t *testing.T) {
	// Shortcut creation fails: the verified copy stays, the message says why.
	f := newPlacementFixture(t)
	got, err := f.env(errors.New("COM unavailable")).install()
	if err != nil || got.State != "copied" || !strings.Contains(got.Message, "Desktop shortcut could not be created (COM unavailable)") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := os.Stat(got.Target); err != nil {
		t.Fatal("copy removed after a shortcut failure")
	}
	// An existing shortcut is left alone.
	f = newPlacementFixture(t)
	os.WriteFile(filepath.Join(f.desktop, "Relax Coach.lnk"), []byte("player's own"), 0644)
	got, err = f.env(nil).install()
	if err != nil || len(f.shortcuts) != 1 || !strings.Contains(got.Message, "Desktop shortcut already exists and points elsewhere; it was left unchanged") {
		t.Fatalf("%+v %v %v", got, err, f.shortcuts)
	}
	if b, _ := os.ReadFile(filepath.Join(f.desktop, "Relax Coach.lnk")); string(b) != "player's own" {
		t.Fatal("existing shortcut overwritten")
	}
	// No shortcut support at all.
	f = newPlacementFixture(t)
	e := f.env(nil)
	e.shortcut = nil
	if got, err := e.install(); err != nil || !strings.Contains(got.Message, "open RelaxCoach.exe from") {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestAppPlacementRefusesLinkedFolderAndUnknownPaths(t *testing.T) {
	f := newPlacementFixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(f.home, "RelaxCoach")); err != nil {
		t.Skip(err)
	}
	e := f.env(nil)
	if p := e.placement(); p.State != "other-copy" && p.State != "offer" {
		t.Fatalf("placement %+v", p)
	}
	if _, err := e.install(); err == nil {
		t.Fatal("copied through a linked RelaxCoach folder")
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 0 {
		t.Fatalf("wrote outside the user folder: %v", entries)
	}
	if p := (appPlacementEnv{err: errors.New("no home")}).placement(); p.State != "unavailable" {
		t.Fatalf("%+v", p)
	}
	if p := (appPlacementEnv{exe: "relative.exe", home: f.home}).placement(); p.State != "unavailable" {
		t.Fatalf("%+v", p)
	}
}

func TestAppPlacementBridge(t *testing.T) {
	f := newPlacementFixture(t)
	b := newNativeBridge(&App{}, false, nativeServices{place: func() appPlacementEnv { return f.env(nil) }})
	defer b.Close()
	p, err := b.Call("appPlacement", nil)
	if err != nil || p.(AppPlacement).State != "offer" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := b.Call("installAppCopy", []json.RawMessage{json.RawMessage(`"x"`)}); err == nil {
		t.Fatal("installAppCopy accepted an argument")
	}
	got, err := b.Call("installAppCopy", nil)
	if err != nil || got.(AppPlacement).State != "copied" {
		t.Fatalf("%+v %v", got, err)
	}
	none := newNativeBridge(&App{}, false, nativeServices{})
	defer none.Close()
	if p, err := none.Call("appPlacement", nil); err != nil || p.(AppPlacement).State != "unavailable" {
		t.Fatalf("%+v %v", p, err)
	}
	if _, err := none.Call("installAppCopy", nil); err == nil {
		t.Fatal("copied without a placement service")
	}
}

func TestAppPlacementDesktopBlockedStartMenuStillAdded(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	write := e.shortcut
	e.shortcut = func(link, target, dir string) error {
		if filepath.Dir(link) == f.desktop {
			return fmt.Errorf("%w (COM method 6 failed (HRESULT 0x80070005))", os.ErrPermission)
		}
		return write(link, target, dir)
	}
	got, err := e.install()
	if err != nil || got.State != "copied" || linkStates(got) != "desktop=missing,startmenu=ok" {
		t.Fatalf("%+v %v", got, err)
	}
	if !strings.Contains(got.Message, "Windows blocked writing to the Desktop") || !strings.Contains(got.Message, "shortcut was added to the Start menu") {
		t.Fatalf("message %q", got.Message)
	}
	noTempLinks(t, f)
}

func TestAppPlacementMissingDesktopFolderIsUnavailable(t *testing.T) {
	f := newPlacementFixture(t)
	os.Remove(f.desktop)
	got, err := f.env(nil).install()
	if err != nil || linkStates(got) != "desktop=unavailable,startmenu=ok" || !strings.Contains(got.Message, "No Desktop folder was found") {
		t.Fatalf("%+v %v", got, err)
	}
	e := f.env(nil)
	e.desktop = ""
	if p := e.placement(); linkStates(p) != "desktop=unavailable,startmenu=ok" {
		t.Fatalf("%+v", p)
	}
}

func TestAppPlacementRepairCreatesMissingAndKeepsOK(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	if _, err := e.install(); err != nil {
		t.Fatal(err)
	}
	e.exe = filepath.Join(f.home, "RelaxCoach", "RelaxCoach.exe") // run the installed copy
	os.Remove(f.link("desktop"))
	before, _ := os.Stat(f.link("startmenu"))
	if p := e.placement(); p.State != "installed" || linkStates(p) != "desktop=missing,startmenu=ok" {
		t.Fatalf("%+v", p)
	}
	f.shortcuts = nil
	got, err := e.repairLinks(nil)
	if err != nil || linkStates(got) != "desktop=ok,startmenu=ok" || len(f.shortcuts) != 1 {
		t.Fatalf("%+v %v %v", got, err, f.shortcuts)
	}
	after, _ := os.Stat(f.link("startmenu"))
	if !os.SameFile(before, after) || !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("an ok Start menu link was rewritten")
	}
	if !strings.Contains(got.Message, "Start menu shortcut was already there") {
		t.Fatalf("message %q", got.Message)
	}
	noTempLinks(t, f)
}

func TestAppPlacementOtherLinkNeedsConfirmation(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	if _, err := e.install(); err != nil {
		t.Fatal(err)
	}
	old := `C:\Old\RelaxCoach.exe`
	os.WriteFile(f.link("desktop"), []byte(old), 0644)
	p := e.placement() // still running from Downloads: other-copy
	if p.State != "other-copy" || linkStates(p) != "desktop=other,startmenu=ok" || p.Links[0].Target != old {
		t.Fatalf("%+v", p)
	}
	got, err := e.repairLinks(nil)
	if err != nil || linkStates(got) != "desktop=other,startmenu=ok" || !strings.Contains(got.Message, "points elsewhere; it was left unchanged") {
		t.Fatalf("%+v %v", got, err)
	}
	if b, _ := os.ReadFile(f.link("desktop")); string(b) != old {
		t.Fatal("replaced without confirmation")
	}
	got, err = e.repairLinks([]string{"desktop"})
	if err != nil || linkStates(got) != "desktop=ok,startmenu=ok" || !strings.Contains(got.Message, "Desktop shortcut was replaced") {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := e.repairLinks([]string{"taskbar"}); err == nil {
		t.Fatal("accepted an unknown shortcut name")
	}
	// A link that is not a regular file (a folder) is "other" and never written through.
	os.Remove(f.link("startmenu"))
	os.Mkdir(f.link("startmenu"), 0755)
	got, err = e.repairLinks([]string{"startmenu"})
	if err != nil || linkStates(got) != "desktop=ok,startmenu=other" || !strings.Contains(got.Message, "changed meanwhile") {
		t.Fatalf("%+v %v", got, err)
	}
	noTempLinks(t, f)
}

func TestAppPlacementReadBackMismatchPublishesNothing(t *testing.T) {
	for _, mode := range []string{"wrong target", "unreadable"} {
		f := newPlacementFixture(t)
		e := f.env(nil)
		e.readShortcut = func(link string) (string, error) {
			if strings.HasPrefix(filepath.Base(link), ".Relax Coach.") {
				if mode == "unreadable" {
					return "", errors.New("bad link")
				}
				return `C:\Elsewhere\x.exe`, nil
			}
			b, err := os.ReadFile(link)
			return string(b), err
		}
		got, err := e.install()
		if err != nil || got.State != "copied" || linkStates(got) != "desktop=missing,startmenu=missing" {
			t.Fatalf("%s: %+v %v", mode, got, err)
		}
		for _, where := range []string{"desktop", "startmenu"} {
			if _, err := os.Lstat(f.link(where)); err == nil {
				t.Fatalf("%s: published %s", mode, where)
			}
		}
		noTempLinks(t, f)
	}
}

func TestAppPlacementLinkAppearingMeanwhileIsNotReplaced(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	appLinkSaved = func(temp string) {
		if filepath.Dir(temp) == f.desktop {
			os.WriteFile(f.link("desktop"), []byte("player's own"), 0644)
		}
	}
	defer func() { appLinkSaved = nil }()
	got, err := e.install()
	if err != nil || !strings.Contains(got.Message, "appeared meanwhile") {
		t.Fatalf("%+v %v", got, err)
	}
	if b, _ := os.ReadFile(f.link("desktop")); string(b) != "player's own" {
		t.Fatal("a link that appeared meanwhile was replaced")
	}
	noTempLinks(t, f)
}

func TestAppPlacementSameCopy(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	if _, err := e.install(); err != nil {
		t.Fatal(err)
	}
	if p := e.placement(); p.State != "other-copy" || !p.SameCopy {
		t.Fatalf("same bytes: %+v", p)
	}
	os.WriteFile(f.exe, []byte("MZ newer build"), 0755)
	if p := e.placement(); p.State != "other-copy" || p.SameCopy {
		t.Fatalf("different bytes: %+v", p)
	}
}

func TestAppPlacementOpenInstalled(t *testing.T) {
	f := newPlacementFixture(t)
	e := f.env(nil)
	target := filepath.Join(f.home, "RelaxCoach", "RelaxCoach.exe")
	if _, err := e.openInstalled(); err == nil || len(f.launched) != 0 {
		t.Fatal("opened without an installed copy")
	}
	if _, err := e.install(); err != nil {
		t.Fatal(err)
	}
	e.args = []string{"-game", `D:\WoW\_forever_`, "-no-update"}
	got, err := e.openInstalled()
	if err != nil || got.State != "opened" || len(f.launched) != 1 || f.launched[0] != target+` -game D:\WoW\_forever_ -no-update` {
		t.Fatalf("%+v %v %v", got, err, f.launched)
	}
	// Anything else on the command line is not passed on.
	f.launched = nil
	e.args = []string{"-headless", "-port", "1"}
	if _, err := e.openInstalled(); err != nil || len(f.launched) != 1 || f.launched[0] != target+" " {
		t.Fatalf("%v %v", err, f.launched)
	}
	// Running the installed copy itself: nothing to open.
	f.launched = nil
	e.exe = target
	if _, err := e.openInstalled(); err == nil || len(f.launched) != 0 {
		t.Fatal("opened itself")
	}
	// A folder or a link at the installed path is refused.
	for _, kind := range []string{"folder", "link"} {
		g := newPlacementFixture(t)
		ge := g.env(nil)
		gt := filepath.Join(g.home, "RelaxCoach", "RelaxCoach.exe")
		os.MkdirAll(filepath.Dir(gt), 0755)
		if kind == "folder" {
			os.Mkdir(gt, 0755)
		} else if err := os.Symlink(g.exe, gt); err != nil {
			t.Skip(err)
		}
		if _, err := ge.openInstalled(); err == nil || len(g.launched) != 0 {
			t.Fatalf("%s: opened", kind)
		}
	}
	// A failed launch is reported and nothing closes.
	g := newPlacementFixture(t)
	ge := g.env(nil)
	ge.install()
	ge.launch = func(string, []string) error { return errors.New("blocked") }
	p, err := ge.openInstalled()
	if err == nil || p.State == "opened" || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("%+v %v", p, err)
	}
	if nativeCloseAfterTest("openInstalledApp", p, err, nil) {
		t.Fatal("closes after a failed open")
	}
}

func TestAppPlacementCloseAfterOpen(t *testing.T) {
	if !nativeCloseAfterTest("openInstalledApp", AppPlacement{State: "opened"}, nil, nil) {
		t.Fatal("window stays open after opening the installed copy")
	}
	for _, c := range []struct {
		method string
		value  any
		call   error
		post   error
	}{
		{"openInstalledApp", AppPlacement{State: "other-copy"}, nil, nil},
		{"openInstalledApp", AppPlacement{State: "opened"}, errors.New("x"), nil},
		{"openInstalledApp", AppPlacement{State: "opened"}, nil, errors.New("post")},
		{"installAppCopy", AppPlacement{State: "opened"}, nil, nil},
		{"repairAppLinks", AppPlacement{State: "opened"}, nil, nil},
	} {
		if nativeCloseAfterTest(c.method, c.value, c.call, c.post) {
			t.Fatalf("closes after %+v", c)
		}
	}
	if !nativeCloseAfterTest("applyApp", AppUpdateResult{RestartReady: true}, nil, nil) {
		t.Fatal("app update restart no longer closes the window")
	}
}

func TestAppPlacementRepairAndOpenBridge(t *testing.T) {
	f := newPlacementFixture(t)
	b := newNativeBridge(&App{}, false, nativeServices{place: func() appPlacementEnv { return f.env(nil) }})
	defer b.Close()
	if _, err := b.Call("repairAppLinks", nil); err == nil {
		t.Fatal("repairAppLinks without its argument")
	}
	if _, err := b.Call("repairAppLinks", []json.RawMessage{json.RawMessage(`""`)}); err == nil {
		t.Fatal("repaired before an installed copy exists")
	}
	if _, err := b.Call("installAppCopy", nil); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(f.link("desktop"), []byte("elsewhere"), 0644)
	got, err := b.Call("repairAppLinks", []json.RawMessage{json.RawMessage(`" desktop ,"`)})
	if err != nil || linkStates(got.(AppPlacement)) != "desktop=ok,startmenu=ok" {
		t.Fatalf("%+v %v", got, err)
	}
	if _, err := b.Call("repairAppLinks", []json.RawMessage{json.RawMessage(`"desktop,../x"`)}); err == nil {
		t.Fatal("accepted an unknown name")
	}
	if _, err := b.Call("openInstalledApp", []json.RawMessage{json.RawMessage(`"x"`)}); err == nil {
		t.Fatal("openInstalledApp accepted an argument")
	}
	got, err = b.Call("openInstalledApp", nil)
	if err != nil || got.(AppPlacement).State != "opened" || len(f.launched) != 1 {
		t.Fatalf("%+v %v %v", got, err, f.launched)
	}
	// Busy: refused while another operation runs.
	b.mu.Lock()
	b.busy = true
	b.mu.Unlock()
	for _, m := range []string{"openInstalledApp", "installAppCopy"} {
		if _, err := b.Call(m, nil); err == nil || !strings.Contains(err.Error(), "already running") {
			t.Fatalf("%s while busy: %v", m, err)
		}
	}
	if _, err := b.Call("repairAppLinks", []json.RawMessage{json.RawMessage(`""`)}); err == nil {
		t.Fatal("repair while busy")
	}
}

func TestNativeLinkErrorMarksAccessDenied(t *testing.T) {
	denied := nativeLinkError(errors.New("COM method 6 failed (HRESULT 0x80070005)"))
	if !errors.Is(denied, os.ErrPermission) || !strings.Contains(appLinkProblem("Desktop", denied), "Windows blocked writing to the Desktop") {
		t.Fatalf("%v", denied)
	}
	stg := nativeLinkError(errors.New("COM method 6 failed (HRESULT 0x80030005)"))
	if !errors.Is(stg, os.ErrPermission) {
		t.Fatalf("STG_E_ACCESSDENIED: %v", stg)
	}
	other := errors.New("COM method 6 failed (HRESULT 0x80004005)")
	if got := nativeLinkError(other); errors.Is(got, os.ErrPermission) || got != other {
		t.Fatalf("%v", got)
	}
	if nativeLinkError(nil) != nil {
		t.Fatal("nil error changed")
	}
}

// nativeCloseAfterTest mirrors the dispatch condition in native_windows.go.
func nativeCloseAfterTest(method string, value any, callErr, postErr error) bool {
	return nativeAppRestartReady(method, value, callErr, postErr) || nativeOpenedInstalled(method, value, callErr, postErr)
}

func TestOpenInstalledIsWiredIntoResponseDispatch(t *testing.T) {
	source, err := os.ReadFile("native_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	dispatch := strings.Index(text, "text, _ := windows.UTF16PtrFromString(string(data))")
	if dispatch < 0 || !strings.Contains(text[dispatch:], "|| nativeOpenedInstalled(req.Method, value, callErr, postErr)") {
		t.Fatal("opening the installed copy does not close this window after the response")
	}
}

func TestAppPlacementTakenTempNameIsNeverRemoved(t *testing.T) {
	f := newPlacementFixture(t)
	old := appLinkRandom
	appLinkRandom = func() (string, error) { return "fixed", nil }
	defer func() { appLinkRandom = old }()
	taken := filepath.Join(f.desktop, ".Relax Coach.fixed.lnk")
	os.WriteFile(taken, []byte("someone else's"), 0644)
	got, err := f.env(nil).install()
	if err != nil || !strings.Contains(got.Message, "temporary shortcut name is taken") {
		t.Fatalf("%+v %v", got, err)
	}
	if b, err := os.ReadFile(taken); err != nil || string(b) != "someone else's" {
		t.Fatalf("a file we did not create was removed or changed: %q %v", b, err)
	}
	// An empty folder at the name is not removed either.
	os.Remove(taken)
	os.Mkdir(taken, 0755)
	f.env(nil).repairLinks(nil)
	if st, err := os.Stat(taken); err != nil || !st.IsDir() {
		t.Fatal("a folder we did not create was removed")
	}
}

func TestAppSameLinkTargetToleratesWindowsSpellings(t *testing.T) {
	old := appLongPath
	defer func() { appLongPath = old }()
	short := map[string]string{
		`C:\Users\PLAYER~1\RelaxCoach\RELAXC~1.EXE`:  `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`,
		`C:\Users\JRGEN~1\RelaxCoach\RelaxCoach.exe`: `C:\Users\Jürgen Ölmann\RelaxCoach\RelaxCoach.exe`,
	}
	appLongPath = func(p string) string {
		if long, ok := short[p]; ok {
			return long
		}
		return p
	}
	for _, c := range []struct {
		name, got, want string
		same            bool
	}{
		{"different-case home", `c:\users\player one\relaxcoach\RELAXCOACH.EXE`, `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`, true},
		{"8.3 short path read back", `C:\Users\PLAYER~1\RelaxCoach\RELAXC~1.EXE`, `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`, true},
		{"8.3 short path on our side", `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`, `C:\Users\PLAYER~1\RelaxCoach\RELAXC~1.EXE`, true},
		{"Unicode user name, other case", `C:\Users\JÜRGEN ÖLMANN\RelaxCoach\RelaxCoach.exe`, `C:\Users\Jürgen Ölmann\RelaxCoach\RelaxCoach.exe`, true},
		{"Unicode user name via 8.3", `C:\Users\JRGEN~1\RelaxCoach\RelaxCoach.exe`, `C:\Users\Jürgen Ölmann\RelaxCoach\RelaxCoach.exe`, true},
		{"GetPath buffer with trailing NULs", "C:\\Users\\Player One\\RelaxCoach\\RelaxCoach.exe\x00\x00\x00garbage", `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`, true},
		{"another folder", `C:\Users\Player One\Downloads\RelaxCoach.exe`, `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`, false},
		{"another user", `C:\Users\Other\RelaxCoach\RelaxCoach.exe`, `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`, false},
		{"empty read-back", "\x00", `C:\Users\Player One\RelaxCoach\RelaxCoach.exe`, false},
	} {
		if got := appSameLinkTarget(c.got, c.want); got != c.same {
			t.Errorf("%s: %v", c.name, got)
		}
	}
	if !appSameLinkTarget("/home/p/RelaxCoach/./RelaxCoach.exe", "/home/p/RelaxCoach/RelaxCoach.exe") {
		t.Error("not cleaned")
	}
}

func TestAppPlacementReadBackSpellingsStillPublish(t *testing.T) {
	// The reader answers in another case with GetPath's NUL padding (a real
	// Windows read-back of a correct link): the link is published and reads ok.
	f := newPlacementFixture(t)
	e := f.env(nil)
	e.readShortcut = func(link string) (string, error) {
		b, err := os.ReadFile(link)
		return strings.ToUpper(string(b)) + "\x00\x00", err
	}
	got, err := e.install()
	if err != nil || linkStates(got) != "desktop=ok,startmenu=ok" {
		t.Fatalf("%+v %v", got, err)
	}
	noTempLinks(t, f)
}

func TestAppPlacementOneDriveDesktop(t *testing.T) {
	// FOLDERID_Desktop redirected into OneDrive: the link lives there and still
	// points at the install folder under the profile.
	f := newPlacementFixture(t)
	f.desktop = filepath.Join(f.home, "OneDrive - Contoso", "Desktop")
	os.MkdirAll(f.desktop, 0755)
	got, err := f.env(nil).install()
	if err != nil || linkStates(got) != "desktop=ok,startmenu=ok" || got.Shortcut != filepath.Join(f.desktop, "Relax Coach.lnk") {
		t.Fatalf("%+v %v", got, err)
	}
}
