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
// the app copy itself to %USERPROFILE%\RelaxCoach\RelaxCoach.exe and add a
// "Relax Coach" desktop shortcut, the same place Install-RelaxCoach.ps1 uses.
// No PowerShell, shell or second process is involved; nothing is forced, and an
// existing installed copy or shortcut is never overwritten.
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
//   - "unavailable": the paths could not be determined; nothing is offered.
type AppPlacement struct {
	State, Running, Target, Shortcut, Message string
}

type appPlacementEnv struct {
	exe, home, desktop string
	err                error
	// shortcut creates link -> target with the given working folder. nil means
	// shortcuts are not supported on this platform.
	shortcut func(link, target, workdir string) error
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
	if appSamePath(p.Running, target) {
		p.State = "installed"
		return p
	}
	if st, err := os.Lstat(target); err == nil && !st.Mode().IsRegular() {
		p.State = "blocked"
		p.Message = "Relax Coach cannot be copied to " + target + ": something else is at that path (a folder or a link). Nothing was changed; move it away to copy the app there."
		return p
	} else if err == nil {
		p.State = "other-copy"
		p.Message = "Relax Coach is already installed in " + filepath.Dir(target) + ". This copy runs from " + filepath.Dir(p.Running) +
			". Use the installed copy (desktop shortcut) and its Update App; this copy changes nothing there."
		return p
	} else if !os.IsNotExist(err) {
		return AppPlacement{State: "unavailable"}
	}
	p.State = "offer"
	p.Message = "Relax Coach is running from " + filepath.Dir(p.Running) + ". Optional: copy it to " + filepath.Dir(target) +
		" and add a desktop shortcut, so it stays in one place and updates itself there. Nothing else changes."
	return p
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
	note := ""
	switch {
	case p.Shortcut == "":
		note = " No desktop folder was found, so no shortcut was added; open RelaxCoach.exe from " + dir + "."
	case e.shortcut == nil:
		note = " Shortcuts are not supported here; open RelaxCoach.exe from " + dir + "."
	default:
		if _, err := os.Lstat(p.Shortcut); err == nil {
			note = " A \"Relax Coach\" desktop shortcut already exists and was left unchanged."
		} else if err := e.shortcut(p.Shortcut, p.Target, dir); err != nil {
			note = " The desktop shortcut could not be created (" + err.Error() + "); open RelaxCoach.exe from " + dir + "."
		} else {
			note = " A \"Relax Coach\" shortcut was added to the desktop."
		}
	}
	p.Message = "Relax Coach was copied to " + p.Target + "." + note +
		" Close this window and start Relax Coach from there next time; you can delete the downloaded file. Your game folder choice is kept."
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
