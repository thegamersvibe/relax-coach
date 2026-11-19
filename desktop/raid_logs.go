package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type raidLogFile struct {
	Path     string
	Modified time.Time
	Bytes    int64
}

// raidLogChoices reads only the selected installation, never a global search.
// The newest file is first; the remaining rotation/archive choices follow by
// modification time. Equal timestamps use the path for deterministic ordering.
func raidLogChoices(installation string) ([]raidLogFile, error) {
	folder := filepath.Join(installation, "Logs")
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var files []raidLogFile
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "WoWCombatLog") || !strings.HasSuffix(name, ".txt") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		files = append(files, raidLogFile{Path: filepath.Join(folder, name), Modified: info.ModTime(), Bytes: info.Size()})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].Modified.Equal(files[j].Modified) {
			return files[i].Path < files[j].Path
		}
		return files[i].Modified.After(files[j].Modified)
	})
	return files, nil
}

// openRaidLog returns a read-only stream; the caller closes it and adapts its
// decoder to fights.Read. A manual path bypasses discovery (including its errors).
// Opening never turns on logging or rotates, moves, truncates or deletes a log.
func openRaidLog(installation, manual string) (*os.File, error) {
	path := manual
	if path == "" {
		files, err := raidLogChoices(installation)
		if err != nil {
			return nil, err
		}
		if len(files) == 0 {
			return nil, errors.New("no WoWCombatLog*.txt in the selected installation's Logs folder")
		}
		path = files[0].Path
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("combat log must be a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err = f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("combat log must be a regular file")
	}
	return f, nil
}
