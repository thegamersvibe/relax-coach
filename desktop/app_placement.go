package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// First run: a player who downloaded only RelaxCoach.exe can, optionally, have
// the app copy itself to %USERPROFILE%\RelaxCoach\RelaxCoach.exe and add
// "Relax Coach" shortcuts on the Desktop and in the Start menu, the same place
// Install-RelaxCoach.ps1 uses. No PowerShell or shell is involved; nothing is
// forced, and an existing installed copy is never overwritten. An existing
// shortcut is replaced only when the player confirms it by name.
const (
	appInstallFolder = "RelaxCoach"
	appInstallExe    = "RelaxCoach.exe"
	appShortcutName  = "Relax Coach.lnk"
)

// AppPlacement is what the UI shows. State is one of:
//   - "installed": running from the usual folder; nothing to offer.
//   - "offer": running from elsewhere and no installed copy exists yet.
//   - "other-copy": running from elsewhere, but an installed copy exists. It is
//     left alone; the player is pointed at it.
//   - "blocked": something other than a regular file (a folder, a link) is at
//     the installed path. Nothing is copied.
//   - "copied": this run copied the app (the result of installAppCopy).
//   - "opened": the installed copy was started (the result of openInstalledApp).
//   - "unavailable": the paths could not be determined; nothing is offered.
//
// Shortcut is the Desktop link path and StartMenu the Start menu one. Links
// says, for each, whether it points at the installed copy. SameCopy is set in
// "other-copy" when the installed file is byte-identical to the running one.
type AppPlacement struct {
	State, Running, Target, Shortcut, StartMenu, Message string
	Links                                                []AppLink
	SameCopy                                             bool
}

// AppLink is one shortcut. Where is "desktop" or "startmenu". State is "ok"
// (points at the installed copy), "missing", "other" (exists but points
// elsewhere, or cannot be read; Target then says where, when known) or
// "unavailable" (the folder could not be found).
type AppLink struct {
	Where, Path, State, Target string
}

type appPlacementEnv struct {
	exe, home, desktop, programs string
	err                          error
	// shortcut creates link -> target with the given working folder, and
	// readShortcut returns a link's target. nil means shortcuts are not
	// supported on this platform.
	shortcut     func(link, target, workdir string) error
	readShortcut func(link string) (string, error)
	// launch starts target with args (openInstalledApp). args are the GUI
	// arguments this process started with.
	launch func(target string, args []string) error
	args   []string
}

var appLinkWheres = []string{"desktop", "startmenu"}

func appLinkLabel(where string) string {
	if where == "startmenu" {
		return "Start menu"
	}
	return "Desktop"
}

func appSamePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (e appPlacementEnv) placement() AppPlacement {
	if e.err != nil || e.exe == "" || e.home == "" || !filepath.IsAbs(e.exe) || !filepath.IsAbs(e.home) {
		return AppPlacement{State: "unavailable"}
	}
	target := filepath.Join(filepath.Clean(e.home), appInstallFolder, appInstallExe)
	p := AppPlacement{Running: filepath.Clean(e.exe), Target: target}
	if e.desktop != "" && filepath.IsAbs(e.desktop) {
		p.Shortcut = filepath.Join(filepath.Clean(e.desktop), appShortcutName)
	}
	if e.programs != "" && filepath.IsAbs(e.programs) {
		p.StartMenu = filepath.Join(filepath.Clean(e.programs), appShortcutName)
	}
	if appSamePath(p.Running, target) {
		p.State = "installed"
		p.Links = e.links(p)
		return p
	}
	if st, err := os.Lstat(target); err == nil && !st.Mode().IsRegular() {
		p.State = "blocked"
		p.Message = "Relax Coach cannot be copied to " + target + ": something else is at that path (a folder or a link). Nothing was changed; move it away to copy the app there."
		return p
	} else if err == nil {
		p.State = "other-copy"
		p.SameCopy = appSameFile(p.Running, target)
		p.Message = "Relax Coach is already installed in " + filepath.Dir(target) + ". This copy runs from " + filepath.Dir(p.Running) +
			". Open the installed copy and use its updates; this copy changes nothing there."
		p.Links = e.links(p)
		return p
	} else if !os.IsNotExist(err) {
		return AppPlacement{State: "unavailable"}
	}
	p.State = "offer"
	p.Message = "Relax Coach is running from " + filepath.Dir(p.Running) + ". Install it in " + filepath.Dir(target) +
		" with Desktop and Start menu shortcuts, so it stays in one place and updates itself there. Nothing else changes."
	p.Links = e.links(p)
	return p
}

// links reads both shortcuts. Nothing is written.
func (e appPlacementEnv) links(p AppPlacement) []AppLink {
	out := make([]AppLink, 0, 2)
	for _, where := range appLinkWheres {
		l := AppLink{Where: where, Path: p.Shortcut}
		if where == "startmenu" {
			l.Path = p.StartMenu
		}
		switch st, err := os.Lstat(l.Path); {
		case l.Path == "":
			l.State = "unavailable"
		case os.IsNotExist(err):
			l.State = "missing"
			if _, derr := os.Stat(filepath.Dir(l.Path)); derr != nil {
				l.State = "unavailable"
			}
		case err != nil || !st.Mode().IsRegular() || e.readShortcut == nil:
			l.State = "other"
		default:
			if target, rerr := e.readShortcut(l.Path); rerr != nil {
				l.State = "other"
			} else if appSameLinkTarget(target, p.Target) {
				l.State, l.Target = "ok", target
			} else {
				l.State, l.Target = "other", target
			}
		}
		out = append(out, l)
	}
	return out
}

// appSameFile is true when both files are readable and have the same SHA256.
func appSameFile(a, b string) bool {
	x, err := appReadFile(a, appMaxExecutable)
	if err != nil {
		return false
	}
	y, err := appReadFile(b, appMaxExecutable)
	if err != nil {
		return false
	}
	return sha256.Sum256(x) == sha256.Sum256(y)
}

// install copies the running executable to the usual folder (exclusive create,
// verified by SHA256) and then adds the shortcut if none exists. A shortcut
// failure keeps the verified copy and says so.
func (e appPlacementEnv) install() (AppPlacement, error) {
	p := e.placement()
	if p.State != "offer" {
		return p, errors.New("there is nothing to copy: " + map[string]string{"installed": "Relax Coach already runs from its folder",
			"other-copy": "an installed copy already exists and is not replaced", "unavailable": "the app folder could not be determined",
			"blocked": "something else is at that path"}[p.State])
	}
	dir := filepath.Dir(p.Target)
	if err := appPlainDirectory(filepath.Dir(dir)); err != nil {
		return p, fmt.Errorf("cannot use the user folder: %w", err)
	}
	created := false
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		if err := os.Mkdir(dir, 0755); err != nil {
			return p, fmt.Errorf("could not create %s: %w", dir, err)
		}
		created = true
	}
	fail := func(err error) (AppPlacement, error) {
		if created {
			os.Remove(dir) // only if still empty
		}
		return p, err
	}
	if err := appPlainDirectory(dir); err != nil {
		return fail(fmt.Errorf("refusing to copy into %s: %w", dir, err))
	}
	data, err := appReadFile(p.Running, appMaxExecutable)
	if err != nil {
		return fail(fmt.Errorf("could not read the running app: %w", err))
	}
	if err := appCopyVerified(p.Target, data); err != nil {
		return fail(fmt.Errorf("could not copy Relax Coach to %s; nothing was changed: %w", p.Target, err))
	}
	p.State = "copied"
	note := e.makeLinks(&p, nil)
	p.Message = "Relax Coach was copied to " + p.Target + "." + note +
		" Close this window and start Relax Coach from there next time; you can delete the downloaded file. Your game folder choice is kept."
	return p, nil
}

// repairLinks creates every missing shortcut and replaces an existing one that
// points elsewhere only when its Where is listed in replace (the player's
// confirmation). Allowed once an installed copy exists.
func (e appPlacementEnv) repairLinks(replace []string) (AppPlacement, error) {
	for _, r := range replace {
		if r != "desktop" && r != "startmenu" {
			return AppPlacement{}, errors.New("unknown shortcut: " + r)
		}
	}
	p := e.placement()
	if p.State != "installed" && p.State != "other-copy" {
		return p, errors.New("install Relax Coach first; shortcuts point at the installed copy")
	}
	note := e.makeLinks(&p, replace)
	p.Message = strings.TrimSpace(note)
	return p, nil
}

// makeLinks writes the shortcuts p.Links says are missing (and the "other" ones
// listed in replace), then reads the state back into p.Links. It returns a
// sentence per shortcut for the message. A failure never undoes the copy.
func (e appPlacementEnv) makeLinks(p *AppPlacement, replace []string) string {
	dir := filepath.Dir(p.Target)
	if e.shortcut == nil || e.readShortcut == nil {
		p.Links = e.links(*p)
		return " Shortcuts are not supported here; open RelaxCoach.exe from " + dir + "."
	}
	note := ""
	for _, l := range e.links(*p) {
		label := appLinkLabel(l.Where)
		switch {
		case l.State == "ok":
			note += " The " + label + " shortcut was already there."
		case l.State == "unavailable":
			note += " No " + label + " folder was found, so no shortcut was added there."
		case l.State == "other" && !appListed(replace, l.Where):
			note += " A \"Relax Coach\" " + label + " shortcut already exists and points elsewhere; it was left unchanged."
		default:
			if err := e.writeLink(l, p.Target, dir); err != nil {
				note += " " + appLinkProblem(label, err)
			} else if l.State == "other" {
				note += " The " + label + " shortcut was replaced."
			} else {
				note += " A \"Relax Coach\" shortcut was added to the " + label + "."
			}
		}
	}
	p.Links = e.links(*p)
	return note
}

func appListed(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// appLinkProblem says in plain words why a shortcut could not be written.
func appLinkProblem(label string, err error) string {
	if errors.Is(err, os.ErrPermission) {
		return "Windows blocked writing to the " + label + ", so that shortcut was not added."
	}
	return "The " + label + " shortcut could not be created (" + err.Error() + ")."
}

// appLinkRandom names temporary shortcuts (a test seam).
var appLinkRandom = func() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(random[:]), nil
}

// appLongPath expands 8.3 short names (GetLongPathNameW on Windows); it returns
// the path unchanged when it cannot.
var appLongPath = func(path string) string { return path }

// appSameLinkTarget compares a shortcut's target with the installed exe the way
// Windows resolves paths: up to the first NUL (GetPath fills a fixed buffer),
// 8.3 names expanded on both sides, cleaned, case-insensitive (Unicode folding).
// A shortcut written correctly must never read back as "points elsewhere".
func appSameLinkTarget(got, want string) bool {
	if i := strings.IndexByte(got, 0); i >= 0 {
		got = got[:i]
	}
	got, want = strings.TrimSpace(got), strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	norm := func(p string) string { return filepath.Clean(appLongPath(filepath.Clean(p))) }
	return strings.EqualFold(norm(got), norm(want))
}

// appLinkSaved is a test seam: called with the temporary link once it is saved
// and before it is read back.
var appLinkSaved func(temp string)

// writeLink saves the shortcut under a new temporary name in the same folder,
// reads it back, and publishes it only if it points at target: a new link with
// a move that never replaces anything; a confirmed "other" link by replacing
// that regular file. Any failure removes the temporary file.
func (e appPlacementEnv) writeLink(l AppLink, target, workdir string) (err error) {
	suffix, err := appLinkRandom()
	if err != nil {
		return err
	}
	temp := filepath.Join(filepath.Dir(l.Path), ".Relax Coach."+suffix+".lnk")
	// Checked before the cleanup below exists: a name that is already taken
	// belongs to someone else and is never removed.
	if _, err := os.Lstat(temp); err == nil || !os.IsNotExist(err) {
		return errors.New("a temporary shortcut name is taken")
	}
	defer func() {
		if err != nil {
			os.Remove(temp)
		}
	}()
	if err := e.shortcut(temp, target, workdir); err != nil {
		return err
	}
	if appLinkSaved != nil {
		appLinkSaved(temp)
	}
	got, err := e.readShortcut(temp)
	if err != nil {
		return fmt.Errorf("the new shortcut could not be read back: %w", err)
	}
	if !appSameLinkTarget(got, target) {
		return errors.New("the new shortcut does not point at " + target)
	}
	if l.State == "other" {
		st, err := os.Lstat(l.Path)
		if err != nil || !st.Mode().IsRegular() {
			return errors.New("the existing shortcut changed meanwhile; nothing was replaced")
		}
		return os.Rename(temp, l.Path) // replaces this regular file only
	}
	if err := appMoveNew(temp, l.Path); err != nil {
		if os.IsExist(err) {
			return errors.New("a shortcut appeared meanwhile; it was not replaced")
		}
		return err
	}
	return nil
}

// appOpenArgs keeps only the GUI arguments a restart may carry (-game, -no-update).
func appOpenArgs(args []string) []string {
	out, err := appGUIArgs(args)
	if err != nil {
		return nil
	}
	return out
}

// openInstalled starts the installed copy with this run's GUI arguments. It
// refuses unless that copy is a regular file other than the running one.
func (e appPlacementEnv) openInstalled() (AppPlacement, error) {
	p := e.placement()
	if p.State == "unavailable" || p.Target == "" {
		return p, errors.New("the installed copy could not be found")
	}
	if appSamePath(p.Running, p.Target) {
		return p, errors.New("this is already the installed copy")
	}
	st, err := os.Lstat(p.Target)
	if err != nil || !st.Mode().IsRegular() {
		return p, errors.New("there is no installed copy at " + p.Target + " to open")
	}
	if e.launch == nil {
		return p, errors.New("opening the installed copy is not supported here")
	}
	if err := e.launch(p.Target, appOpenArgs(e.args)); err != nil {
		return p, fmt.Errorf("could not open %s: %w", p.Target, err)
	}
	p.State = "opened"
	p.Message = "Opening Relax Coach from " + filepath.Dir(p.Target) + "…"
	return p, nil
}

// appCopyWritten is a test seam: called with the path being written once all
// bytes are on disk and before they are verified and published.
var appCopyWritten func(path string)

// appCopyPublished is a test seam: called right after the copy is published
// under its final name, the last step of appCopyVerified.
var appCopyPublished func(target string)

// appCopyVerified writes data to a new temp file next to target (exclusive
// create, random name), verifies it by SHA256 and only then publishes it under
// the target name with one move that never replaces anything (appMoveNew:
// MoveFileEx without REPLACE_EXISTING on Windows, rename(RENAME_NOREPLACE) on
// Linux). Publishing is the last step, so the file never has a second name
// and any error means nothing was published. A crash leaves at most a temp
// file, never a half-written RelaxCoach.exe; temp files from earlier runs are
// left alone and never block a later copy.
func appCopyVerified(target string, data []byte) error {
	dir := filepath.Dir(target)
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	temp := filepath.Join(dir, "."+filepath.Base(target)+"."+hex.EncodeToString(random[:])+".tmp")
	want := sha256.Sum256(data)
	return appWriteOwned(temp, data, func() error {
		if appCopyWritten != nil {
			appCopyWritten(temp)
		}
		got, err := appReadFile(temp, appMaxExecutable)
		if err != nil {
			return err
		}
		if sum := sha256.Sum256(got); !bytes.Equal(sum[:], want[:]) {
			return errors.New("the copied app does not match the running app")
		}
		if err := appPlainDirectory(dir); err != nil {
			return err
		}
		if err := appMoveNew(temp, target); err != nil {
			if os.IsExist(err) {
				return errors.New("something appeared at " + target + " meanwhile; it was not replaced")
			}
			return err
		}
		if appCopyPublished != nil {
			appCopyPublished(target)
		}
		return nil
	})
}
