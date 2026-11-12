package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// AccountChoice identifies a local WTF/Account folder, not an online account.
// Each save belongs to exactly one identity within the selected installation.
type AccountChoice struct {
	ID, SVPath string
	SaveTime   time.Time
}

func validAccountID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.EqualFold(id, "SavedVariables") &&
		!strings.ContainsAny(id, "/\\:\x00") && !strings.HasSuffix(id, ".") && !strings.HasSuffix(id, " ")
}

func openPlainDirectory(root *os.Root, name string) (*os.Root, error) {
	st, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("not a local directory: %s", name)
	}
	dir, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	// OpenRoot may follow an in-root link inserted after Lstat. Check the
	// opened directory itself, not the path (which may have changed again).
	opened, err := dir.Stat(".")
	if err != nil {
		dir.Close()
		return nil, err
	}
	if !opened.IsDir() || opened.Mode()&os.ModeSymlink != 0 || !os.SameFile(st, opened) {
		dir.Close()
		return nil, fmt.Errorf("local directory changed while opening: %s", name)
	}
	return dir, nil
}

// Root operations enforce containment; openPlainDirectory also binds each
// checked directory's identity to its opened handle at every layer.
func openAccountsRoot(game string) (*os.Root, error) {
	root, err := os.OpenRoot(game)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	wtf, err := openPlainDirectory(root, "WTF")
	if err != nil {
		return nil, err
	}
	defer wtf.Close()
	return openPlainDirectory(wtf, "Account")
}

func openAccountOutput(root *os.Root, id string) (*os.File, error) {
	return openAccountFile(root, id, "ForeverCoach.lua")
}

// openAccountFile opens one SavedVariables file of one local account, with
// the same containment and identity checks for every file the app reads.
func openAccountFile(root *os.Root, id, name string) (*os.File, error) {
	if !validAccountID(id) {
		return nil, fmt.Errorf("invalid local account identity")
	}
	account, err := openPlainDirectory(root, id)
	if err != nil {
		return nil, err
	}
	defer account.Close()
	saves, err := openPlainDirectory(account, "SavedVariables")
	if err != nil {
		return nil, err
	}
	defer saves.Close()
	st, err := saves.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("account output is not a regular local save")
	}
	f, err := saves.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	// A save replacement during this open is transiently unavailable. Compare
	// only this read's identity so the next refresh accepts a normal new save.
	if !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		f.Close()
		return nil, fmt.Errorf("account output changed while opening")
	}
	return f, nil
}

func enumerateAccounts(game string) []AccountChoice {
	var accounts []AccountChoice
	if game == "" {
		return accounts
	}
	root, err := openAccountsRoot(game)
	if err != nil {
		return accounts
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return accounts
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return accounts
	}
	for _, entry := range entries {
		if !entry.IsDir() || !validAccountID(entry.Name()) {
			continue
		}
		f, err := openAccountOutput(root, entry.Name())
		if err != nil {
			continue
		}
		st, err := f.Stat()
		f.Close()
		if err == nil && st.Mode().IsRegular() {
			accounts = append(accounts, AccountChoice{ID: entry.Name(), SVPath: filepath.Join(game, "WTF", "Account", entry.Name(), "SavedVariables", "ForeverCoach.lua"), SaveTime: st.ModTime()})
		}
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })
	return accounts
}

func (a *App) readSelectedSave() ([]byte, os.FileInfo, error) {
	var f *os.File
	var err error
	if a.account == "" {
		// Internal legacy diagnostic callers can supply a save directly; native
		// requests never accept a save path or populate this field themselves.
		f, err = os.Open(a.svPath)
	} else {
		// Even a remembered name must match the current enumeration exactly.
		// In particular, Windows case-insensitive path lookup is not authority
		// to reinterpret a missing identity as a differently named account.
		available := false
		for _, account := range a.accounts {
			if account.ID == a.account {
				available = true
				break
			}
		}
		if !available {
			return nil, nil, fmt.Errorf("selected account %q output is unavailable; log in and /reload, or choose another account", a.account)
		}
		root, e := openAccountsRoot(a.game)
		if e != nil {
			return nil, nil, e
		}
		defer root.Close()
		f, err = openAccountOutput(root, a.account)
	}
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !st.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("account output is not a regular local save")
	}
	data, err := io.ReadAll(f)
	return data, st, err
}

func (a *App) rescanAccountsLocked() {
	a.accounts = enumerateAccounts(a.game)
	if len(a.accounts) > 1 {
		a.canAutoSelect = false
	}
	if a.account == "" && a.canAutoSelect && len(a.accounts) == 1 {
		a.account = a.accounts[0].ID
		if err := saveAccount(a.game, a.account); err != nil {
			a.configWarning = err.Error()
		}
	}
	a.svPath = ""
	if validAccountID(a.account) {
		// Preserve the selected identity even if its save is missing.
		a.svPath = filepath.Join(a.game, "WTF", "Account", a.account, "SavedVariables", "ForeverCoach.lua")
	}
}

func (a *App) selectAccount(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	accounts := enumerateAccounts(a.game)
	for _, account := range accounts {
		if account.ID != id {
			continue
		}
		if err := saveAccount(a.game, id); err != nil {
			return err
		}
		a.configWarning = ""
		a.accounts = accounts
		if a.account != id {
			a.account, a.svPath = id, account.SVPath
			a.modTime = time.Time{}
			// Never retain another account's report, even if this save is malformed.
			a.view = View{AppVersion: Version, Game: a.game, NoGame: a.game == ""}
		}
		a.reloadLocked()
		return nil
	}
	return fmt.Errorf("choose an available account from this game folder")
}
