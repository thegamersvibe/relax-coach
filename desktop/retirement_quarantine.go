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
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Type        string `json:"type"`
	SHA256      string `json:"sha256,omitempty"`
}
type retirementManifest struct {
	Schema  int               `json:"schema"`
	Entries []retirementEntry `json:"entries"`
}
type retirementQuarantine struct {
	game     string
	root     *os.Root
	run      string
	entries  []retirementEntry
	created  []string
	serial   int
	inFlight bool
	blocked  bool
}

// Only renames within the already-open game root; never copy and delete.
var retirementRename = func(root *os.Root, from, to string) error {
	// The run directory is private and uniquely created for this pass. Check
	// again at the mutation boundary: os.Rename would replace an existing file.
	if _, err := root.Lstat(to); !os.IsNotExist(err) {
		return fmt.Errorf("destination already exists or is unreadable")
	}
	return root.Rename(from, to)
}
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
func (q *retirementQuarantine) mkdir(path string) error {
	if err := q.root.Mkdir(path, 0700); err != nil {
		if !os.IsExist(err) {
			return err
		}
	} else {
		q.created = append(q.created, path)
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
			q.run = run
			q.created = append(q.created, run)
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
	return q.atomicJSON("manifest.json", retirementManifest{1, q.entries})
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
	q.created = append(q.created, temp)
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
	return q.root.Rename(temp, filepath.Join(q.run, name))
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
	if err = q.atomicJSON("pending.json", retirementManifest{1, entries}); err != nil {
		result.skip(source, err)
		return
	}
	if len(before) > 0 && !before[0]() {
		_ = q.clearPending()
		return
	}
	q.inFlight = true
	if err = retirementRename(q.root, relative, target); err != nil {
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
			_ = q.root.Remove(q.created[i])
		}
	}
	_ = q.root.Close()
}
