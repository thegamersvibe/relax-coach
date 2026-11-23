package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const retirementStorage = "RelaxCoachRetired"

type retirementEntry struct {
	Source      string                     `json:"source"`
	Destination string                     `json:"destination"`
	Type        string                     `json:"type"`
	SHA256      string                     `json:"sha256,omitempty"`
	Extra       map[string]json.RawMessage `json:"-"`
}
type retirementManifest struct {
	Schema  int                        `json:"schema"`
	Entries []retirementEntry          `json:"entries"`
	Extra   map[string]json.RawMessage `json:"-"`
}
type retirementQuarantine struct {
	game     string
	root     *os.Root
	run      string
	entries  []retirementEntry
	created  []string
	owned    map[string]os.FileInfo
	serial   int
	inFlight bool
	blocked  bool
}

// Keep the root-relative seam for failure/crash probes, but use an atomic
// no-replace primitive for both addon directories and saved-variable files.
var retirementRename = func(root *os.Root, from, to string) error {
	if !filepath.IsAbs(root.Name()) || !filepath.IsLocal(from) || !filepath.IsLocal(to) {
		return fmt.Errorf("move is not inside the opened game root")
	}
	oldAbs, newAbs := filepath.Join(root.Name(), from), filepath.Join(root.Name(), to)
	// Revalidate both plain parents and their identity against the opened root.
	// Absolute paths passed to the OS must still name those validated entries.
	for _, path := range []string{from, to} {
		parent := filepath.Dir(path)
		abs := filepath.Join(root.Name(), parent)
		if err := appPlainDirectory(abs); err != nil {
			return err
		}
		bound, err := root.Lstat(parent)
		if err != nil {
			return err
		}
		current, err := os.Lstat(abs)
		if err != nil || !os.SameFile(bound, current) {
			return fmt.Errorf("move parent changed")
		}
	}
	bound, err := root.Lstat(from)
	if err != nil {
		return err
	}
	current, err := os.Lstat(oldAbs)
	if err != nil || !os.SameFile(bound, current) || (!current.IsDir() && !current.Mode().IsRegular()) {
		return fmt.Errorf("linked, special or changed move source")
	}
	if err := appRejectReparse(oldAbs); err != nil {
		return err
	}
	if _, err := root.Lstat(to); !os.IsNotExist(err) {
		return fmt.Errorf("destination already exists or is unreadable")
	}
	// This check is only diagnostic; the primitive MUST reject a destination
	// created after it, with no copy/delete or replacing-rename fallback.
	return retirementMoveNoReplace(oldAbs, newAbs)
}
var retirementMoveNoReplace = retirementPlatformMoveNoReplace
var retirementNow = time.Now

func newRetirementQuarantine(game string) (*retirementQuarantine, error) {
	if err := appPlainDirectory(filepath.Join(game, retirementStorage)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	root, err := os.OpenRoot(game)
	if err != nil {
		return nil, err
	}
	return &retirementQuarantine{game: game, root: root}, nil
}
func (q *retirementQuarantine) rememberCreated(path string, info os.FileInfo) {
	if q.owned == nil {
		q.owned = make(map[string]os.FileInfo)
	}
	q.created = append(q.created, path)
	q.owned[path] = info
}
func (q *retirementQuarantine) forgetCreated(path string) {
	delete(q.owned, path)
	for i, name := range q.created {
		if name == path {
			q.created = append(q.created[:i], q.created[i+1:]...)
			return
		}
	}
}
func (q *retirementQuarantine) mkdir(path string) error {
	if err := q.root.Mkdir(path, 0700); err != nil {
		if !os.IsExist(err) {
			return err
		}
	} else {
		info, err := q.root.Lstat(path)
		if err != nil {
			return err
		}
		q.rememberCreated(path, info)
	}
	return appPlainDirectory(filepath.Join(q.game, path))
}
func (q *retirementQuarantine) prepare(relative string) error {
	if err := q.mkdir(retirementStorage); err != nil {
		return err
	}
	if q.run == "" {
		stamp := retirementNow().UTC().Format("20060102T150405Z")
		for n := 0; ; n++ {
			name := stamp
			if n != 0 {
				name = fmt.Sprintf("%s-%d", stamp, n)
			}
			run := filepath.Join(retirementStorage, name)
			err := q.root.Mkdir(run, 0700)
			if os.IsExist(err) {
				continue
			}
			if err != nil {
				return err
			}
			info, err := q.root.Lstat(run)
			if err != nil {
				return err
			}
			q.run = run
			q.rememberCreated(run, info)
			break
		}
	}
	parent := q.run
	parts := strings.Split(filepath.Dir(relative), string(filepath.Separator))
	for _, part := range parts {
		if part == "." {
			continue
		}
		parent = filepath.Join(parent, part)
		if err := q.mkdir(parent); err != nil {
			return err
		}
	}
	return nil
}
func (q *retirementQuarantine) inventory(source, destination string) ([]retirementEntry, error) {
	var entries []retirementEntry
	err := filepath.WalkDir(filepath.Join(q.game, source), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink != 0 || (!d.IsDir() && !d.Type().IsRegular()) {
			return fmt.Errorf("linked or special entry: %s", path)
		}
		if err = appRejectReparse(path); err != nil {
			return err
		}
		sub, err := filepath.Rel(filepath.Join(q.game, source), path)
		if err != nil {
			return err
		}
		e := retirementEntry{Source: filepath.ToSlash(filepath.Join(source, sub)), Destination: filepath.ToSlash(filepath.Join(destination, sub)), Type: "dir"}
		if !d.IsDir() {
			e.Type = "file"
			e.SHA256, err = q.digest(filepath.FromSlash(e.Source))
			if err != nil {
				return err
			}
		}
		entries = append(entries, e)
		return nil
	})
	return entries, err
}
func (q *retirementQuarantine) digest(path string) (string, error) {
	f, err := q.root.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func (q *retirementQuarantine) writeManifest() error {
	return q.atomicJSON("manifest.json", retirementManifest{Schema: 1, Entries: q.entries})
}
func (q *retirementQuarantine) atomicJSON(name string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	var f *os.File
	var temp string
	for {
		q.serial++
		temp = filepath.Join(q.run, fmt.Sprintf("metadata-%d.tmp", q.serial))
		f, err = q.root.OpenFile(temp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		break
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	q.rememberCreated(temp, info)
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	// Only internal metadata is replaced here, inside our run directory;
	// user data always uses the separate no-replace primitive.
	if err = q.root.Rename(temp, filepath.Join(q.run, name)); err != nil {
		return err
	}
	// The temp path no longer belongs to us. Never remove a later occupant,
	// and never put the published pending/manifest name in the cleanup list.
	q.forgetCreated(temp)
	return nil
}
func (q *retirementQuarantine) rename(from, to string) error {
	parent := filepath.Dir(to)
	owned, ok := q.owned[parent]
	current, err := q.root.Lstat(parent)
	if !ok || err != nil || !current.IsDir() || !os.SameFile(owned, current) {
		return fmt.Errorf("destination parent is no longer our created directory")
	}
	return retirementRename(q.root, from, to)
}
func (q *retirementQuarantine) move(source, destination string, result *retirementResult, before ...func() bool) {
	if q.blocked {
		result.skip(source, fmt.Errorf("quarantine journal needs recovery"))
		return
	}
	relative, err := filepath.Rel(q.game, source)
	if err != nil || !filepath.IsLocal(relative) {
		result.skip(source, fmt.Errorf("not inside game"))
		return
	}
	if err = q.prepare(destination); err != nil {
		result.skip(source, err)
		return
	}
	target := filepath.Join(q.run, destination)
	if _, err = q.root.Lstat(target); !os.IsNotExist(err) {
		result.skip(source, fmt.Errorf("destination already exists or is unreadable"))
		return
	}
	entries, err := q.inventory(relative, target)
	if err != nil {
		result.skip(source, err)
		return
	}
	// Intent is durable BEFORE the rename, but is not a claim of completion.
	if err = q.atomicJSON("pending.json", retirementManifest{Schema: 1, Entries: entries}); err != nil {
		result.skip(source, err)
		return
	}
	if len(before) > 0 && !before[0]() {
		_ = q.clearPending()
		return
	}
	q.inFlight = true
	if err = q.rename(relative, target); err != nil {
		q.inFlight = false
		if clearErr := q.clearPending(); clearErr != nil {
			q.blocked = true
		}
		result.skip(source, err)
		return
	}
	q.inFlight = false
	q.entries = append(q.entries, entries...)
	result.Moved = append(result.Moved, filepath.ToSlash(relative)+" -> "+filepath.ToSlash(target))
	if err = q.writeManifest(); err != nil {
		q.blocked = true
		result.Skipped = append(result.Skipped, "moved "+relative+"; journal kept for manifest recovery next start: "+err.Error())
	}
	if err = q.verify(entries); err != nil {
		q.blocked = true
		result.Skipped = append(result.Skipped, "moved "+relative+"; hash verification failed (kept in quarantine): "+err.Error())
	}
	if !q.blocked {
		if err = q.clearPending(); err != nil {
			q.blocked = true
			result.Skipped = append(result.Skipped, "moved "+relative+"; journal finalization will retry next start: "+err.Error())
		}
	}
}
func (q *retirementQuarantine) close() {
	// Only our own empty scaffolding/temporary metadata is removed, never a
	// source or a quarantined item. An empty/failed pass leaves no run directory.
	if len(q.entries) == 0 && !q.inFlight && !q.blocked {
		for i := len(q.created) - 1; i >= 0; i-- {
			path := q.created[i]
			owned, ok := q.owned[path]
			current, err := q.root.Lstat(path)
			if ok && err == nil && os.SameFile(owned, current) && appRejectReparse(filepath.Join(q.game, path)) == nil {
				_ = q.root.Remove(q.created[i])
			}
		}
	}
	_ = q.root.Close()
}
