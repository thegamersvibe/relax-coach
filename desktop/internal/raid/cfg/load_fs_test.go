package cfg_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

func mapConfig(t *testing.T) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	for _, name := range dataFiles {
		b, err := os.ReadFile(filepath.Join(seedDir, name))
		if err != nil {
			t.Fatal(err)
		}
		files[name] = &fstest.MapFile{Data: b}
	}
	return files
}

func TestLoadFSMapAndDir(t *testing.T) {
	want := load(t, seedDir).Snapshot()
	for _, files := range []fs.FS{mapConfig(t), os.DirFS(seedDir)} {
		got, err := cfg.LoadFS(files)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got.Snapshot(), want) {
			t.Fatal("LoadFS differs from Load")
		}
	}
}

func TestLoadFSReadError(t *testing.T) {
	for _, name := range dataFiles {
		t.Run(name, func(t *testing.T) {
			files := mapConfig(t)
			delete(files, name)
			got, err := cfg.LoadFS(files)
			if got != nil || !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), name) {
				t.Fatalf("missing file: config=%v error=%v", got != nil, err)
			}
		})
	}
}

func TestLoadFSDecodeAndValidation(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement string }{
		{"unknown key", `"weights":`, `"unexpected":0,"weights":`},
		{"invalid range", `"tail_ms": 2500`, `"tail_ms": -1`},
		{"decoded metadata", `"version": "`, `"version": "bad-`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := mapConfig(t)
			b := files["scoring.json"].Data
			changed := bytes.Replace(b, []byte(tc.old), []byte(tc.replacement), 1)
			if bytes.Equal(b, changed) {
				t.Fatal("fixture edit missed")
			}
			files["scoring.json"].Data = changed
			got, err := cfg.LoadFS(files)
			if got != nil || err == nil || !strings.Contains(err.Error(), "scoring.json") {
				t.Fatalf("bad config admitted: config=%v error=%v", got != nil, err)
			}
		})
	}
}

func TestLoadFSReloadAndOwnership(t *testing.T) {
	files := mapConfig(t)
	before, err := cfg.LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	files["scoring.json"].Data = bytes.Replace(files["scoring.json"].Data, []byte(`"tail_ms": 2500`), []byte(`"tail_ms": 2501`), 1)
	after, err := cfg.LoadFS(files)
	if err != nil {
		t.Fatal(err)
	}
	if before.Snapshot().Scoring.Fights.TailMs != 2500 || after.Snapshot().Scoring.Fights.TailMs != 2501 {
		t.Fatal("filesystem load cached or aliased")
	}
	after.Realms.Labels[0] = "Edited"
	if after.Snapshot().Realms.Labels[0] != "PvP-EU" {
		t.Fatal("view aliases validated snapshot")
	}
}
