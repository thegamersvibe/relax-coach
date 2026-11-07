package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestGuildFunModeApplyAndRollback(t *testing.T) {
	entries := funModeEntries("1.4.3")
	payload := funModeZip(t, entries, "1.4.3")
	for _, scenario := range []string{"success", "later move fails", "nested media corrupted"} {
		t.Run(scenario, func(t *testing.T) {
			game := guildGame(t)
			addonPut(t, game, guildFixtureRoots[0]+"/ForeverCoach.toc", addonFixtureEntries("1.4.2")[0].body)
			before := addonTreeHashes(t, game)
			u := guildUpdaterForVersion(t, payload, "1.4.3")
			moves := 0
			u.rename = func(from, to string) error {
				moves++
				if scenario == "later move fails" && moves == 4 {
					return os.ErrPermission
				}
				if err := os.Rename(from, to); err != nil {
					return err
				}
				if scenario == "nested media corrupted" && moves == 2 {
					for _, e := range entries {
						if e.name == guildFixtureRoots[0]+"/Media/Reactions/bags_1.mp3" {
							// Same size: the installed-file SHA256 check must catch this.
							addonPut(t, to, "Media/Reactions/bags_1.mp3", strings.Replace(e.body, "fixture", "FIXTURE", 1))
						}
					}
				}
				return nil
			}
			result, err := u.apply(context.Background(), game, guildInfoForVersion("1.4.3"))
			if scenario != "success" {
				if err == nil || result.Version != "" || !strings.Contains(err.Error(), "restored") {
					t.Fatalf("failed update not rolled back: %+v %v", result, err)
				}
				if scenario == "nested media corrupted" && !strings.Contains(err.Error(), "SHA256 mismatch") {
					t.Fatalf("nested media corruption not hash-checked: %v", err)
				}
				if !reflect.DeepEqual(before, addonTreeHashes(t, game)) {
					t.Fatal("rollback did not restore all runtime and protected bytes")
				}
				return
			}
			if err != nil || result.Version != "1.4.3" || result.BackupPath == "" {
				t.Fatalf("1.4.2 -> 1.4.3 failed: %+v %v", result, err)
			}
			after := addonTreeHashes(t, game)
			count := 0
			for _, e := range entries {
				if !guildRuntime(e.name) {
					if after[e.name] != before[e.name] {
						t.Fatalf("bundled executable/script/extra installed: %s", e.name)
					}
					continue
				}
				count++
				if after[e.name] != fmt.Sprintf("%x", sha256.Sum256([]byte(e.body))) {
					t.Fatalf("installed runtime byte mismatch: %s", e.name)
				}
			}
			if result.UpdatedFiles != count || InstalledCoachVersion(game) != "1.4.3" {
				t.Fatalf("wrong installed inventory/version: %+v", result)
			}
			if got := addonTreeHashes(t, filepath.Join(game, filepath.FromSlash(guildFixtureRoots[0]))); len(got) != 19 {
				t.Fatalf("Coach inventory: got %d want 19", len(got))
			}
			backup := addonTreeHashes(t, result.BackupPath)
			for name, hash := range before {
				got := after[name]
				if guildRuntime(name) {
					got = backup[name]
				}
				if got != hash {
					t.Fatalf("original runtime/protected bytes lost: %s", name)
				}
			}
			if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")); !os.IsNotExist(err) {
				t.Fatalf("update lock retained after success: %v", err)
			}
			if _, err := u.apply(context.Background(), game, guildInfoForVersion("1.4.3")); err == nil {
				t.Fatal("current Fun Mode bundle reinstalled")
			}
			if !reflect.DeepEqual(after, addonTreeHashes(t, game)) {
				t.Fatal("repeat apply changed the installed release or backups")
			}
		})
	}
}
