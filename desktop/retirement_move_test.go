package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRetirementMoveRequiresOwnedDestinationParent(t *testing.T) {
	game := retirementFixture(t)
	save := "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"
	addonPut(t, game, save, "keep source")
	q, err := newRetirementQuarantine(game)
	if err != nil {
		t.Fatal(err)
	}
	defer q.close()
	result := retirementResult{}
	q.move(filepath.Join(game, save), filepath.FromSlash(save), &result, func() bool {
		parent := filepath.Join(game, q.run, filepath.Dir(save))
		if err := os.Rename(parent, parent+".kept"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(parent, 0700); err != nil {
			t.Fatal(err)
		}
		return true
	})
	if len(result.Moved) != 0 || len(result.Skipped) == 0 {
		t.Errorf("moved into another writer's directory: %+v", result)
	}
	if got, err := os.ReadFile(filepath.Join(game, save)); err != nil || string(got) != "keep source" {
		t.Fatalf("source changed: %q %v", got, err)
	}
}

func TestRetirementRevalidatesSourceAtMove(t *testing.T) {
	for _, kind := range []string{"file", "dir"} {
		t.Run(kind, func(t *testing.T) {
			game := retirementFixture(t)
			source := "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"
			destination := source
			if kind == "dir" {
				source = "Interface/AddOns/ForeverCoach"
				destination = "AddOns/ForeverCoach"
				addonPut(t, game, source+"/Core.lua", "original")
			} else {
				addonPut(t, game, source, "original")
			}
			q, err := newRetirementQuarantine(game)
			if err != nil {
				t.Fatal(err)
			}
			defer q.close()
			outside := t.TempDir()
			addonPut(t, outside, "keep", "foreign bytes")
			before := retirementTestSnapshot(t, outside)
			result := retirementResult{}
			full := filepath.Join(game, source)
			q.move(full, filepath.FromSlash(destination), &result, func() bool {
				if err := os.Rename(full, full+".kept"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, full); err != nil {
					t.Skip(err)
				}
				return true
			})
			if len(result.Moved) != 0 || len(result.Skipped) == 0 {
				t.Errorf("changed source was moved: %+v", result)
			}
			if st, err := os.Lstat(full); err != nil || st.Mode()&os.ModeSymlink == 0 {
				t.Errorf("source link not left in place: %v", err)
			}
			if !reflect.DeepEqual(before, retirementTestSnapshot(t, outside)) {
				t.Fatal("foreign data changed")
			}
		})
	}
}

func TestRetirementDestinationAppearsInsideMove(t *testing.T) {
	for _, kind := range []string{"save", "empty-addon", "nonempty-addon"} {
		t.Run(kind, func(t *testing.T) {
			game := retirementFixture(t)
			source := "Interface/AddOns/ForeverCoach"
			content := source + "/Core.lua"
			if kind == "save" {
				source = "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"
				content = source
			}
			addonPut(t, game, content, "source bytes\x00\xff")
			old := retirementMoveNoReplace
			t.Cleanup(func() { retirementMoveNoReplace = old })
			var destination string
			var existing os.FileInfo
			var snapshot map[string]string
			calls := 0
			retirementMoveNoReplace = func(from, to string) error {
				calls++
				destination = to
				// Past every destination check: the primitive itself must refuse.
				if kind == "save" {
					if err := os.WriteFile(to, []byte("existing bytes\xff\x00"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Mkdir(to, 0700); err != nil {
						t.Fatal(err)
					}
					if kind == "nonempty-addon" {
						addonPut(t, to, "keep.lua", "existing addon bytes")
					}
				}
				var err error
				existing, err = os.Lstat(to)
				if err != nil {
					t.Fatal(err)
				}
				snapshot = retirementTestSnapshot(t, to)
				return old(from, to)
			}
			first := retireGame(context.Background(), game, retirementClosed)
			retirementMoveNoReplace = old
			if calls != 1 || len(first.Moved) != 0 || !strings.Contains(strings.Join(first.Skipped, " "), "left in place; will retry next start") {
				t.Errorf("late conflict not refused/reported: calls=%d %+v", calls, first)
			}
			if data, err := os.ReadFile(filepath.Join(game, content)); err != nil || string(data) != "source bytes\x00\xff" {
				t.Errorf("source changed: %q %v", data, err)
			}
			checkDestination := func() {
				t.Helper()
				now, err := os.Lstat(destination)
				if err != nil || !os.SameFile(existing, now) || !reflect.DeepEqual(snapshot, retirementTestSnapshot(t, destination)) {
					t.Errorf("existing destination replaced/changed: %v", err)
				}
			}
			checkDestination()
			second := retireGame(context.Background(), game, retirementClosed)
			if len(second.Moved) != 1 || len(second.Skipped) != 0 || strings.Contains(second.Moved[0], filepath.ToSlash(strings.TrimPrefix(destination, game+string(filepath.Separator)))) {
				t.Errorf("next start did not retry to a fresh destination: %+v", second)
			}
			checkDestination()
			retirementExists(t, filepath.Join(game, source), false)
		})
	}
}
