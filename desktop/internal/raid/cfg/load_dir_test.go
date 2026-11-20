package cfg_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/thegamersvibe/relax-coach/desktop/internal/raid/cfg"
)

func TestLoadDirCleaning(t *testing.T) {
	for _, kind := range []string{"symlink", "missing", "empty"} {
		t.Run(kind, func(t *testing.T) {
			dir := copyData(t)
			want := load(t, dir).Snapshot()
			path := dir + string(os.PathSeparator) + "missing" + string(os.PathSeparator) + ".."
			switch kind {
			case "symlink":
				target := filepath.Join(t.TempDir(), "child")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				link := filepath.Join(dir, "link")
				if err := os.Symlink(target, link); err != nil {
					t.Fatal(err)
				}
				// Do not pre-clean: the old filepath.Join removed link/..
				// lexically, without following link or requiring missing.
				path = link + string(os.PathSeparator) + ".."
			case "empty":
				t.Chdir(dir)
				path = ""
			}
			got, err := cfg.Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Snapshot(), want) {
				t.Fatal("directory load differs from pre-LoadFS path semantics")
			}
		})
	}
}

func TestLoadDirReadError(t *testing.T) {
	for _, name := range []string{"scoring.json", "instances.json"} {
		t.Run(name, func(t *testing.T) {
			dir := copyData(t)
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
			path := dir + string(os.PathSeparator) + "missing" + string(os.PathSeparator) + ".."
			_, readErr := os.ReadFile(filepath.Join(path, name))
			want := fmt.Errorf("%s: %w", name, readErr)
			got, err := cfg.Load(path)
			var pathErr *fs.PathError
			if got != nil || !errors.Is(err, fs.ErrNotExist) || !errors.As(err, &pathErr) {
				t.Fatalf("read failure lost error identity: config=%v error=%v", got != nil, err)
			}
			if err.Error() != want.Error() || pathErr.Path != filepath.Join(dir, name) {
				t.Fatalf("read error = %q, want %q; path=%q", err, want, pathErr.Path)
			}
		})
	}
}
