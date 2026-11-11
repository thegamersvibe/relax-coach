package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A crash in the middle of a bundle install (the app killed, power loss)
// leaves RelaxCoachBackups/.addon-update.lock behind. Before this fix every
// later Install/Update refused with "holds the backup lock" forever.
func crashedBundleTransaction(t *testing.T, game string, age time.Duration, movedAside bool) (lock, transaction string) {
	t.Helper()
	backups := filepath.Join(game, "RelaxCoachBackups")
	transaction = filepath.Join(backups, "ForeverCoach-20261101T000000.000000000Z-123456")
	if err := os.MkdirAll(filepath.Join(transaction, "staged"), 0700); err != nil {
		t.Fatal(err)
	}
	if movedAside {
		// Killed right after Journey was moved aside, before the new one landed.
		prev := filepath.Join(transaction, "previous", "Interface", "AddOns", "Journey")
		os.MkdirAll(filepath.Dir(prev), 0700)
		if err := os.Rename(filepath.Join(game, "Interface", "AddOns", "Journey"), prev); err != nil {
			t.Fatal(err)
		}
	}
	lock = filepath.Join(backups, ".addon-update.lock")
	if err := os.WriteFile(lock, []byte(transaction+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	os.Chtimes(lock, old, old)
	return lock, transaction
}

func TestFICrashBundleBlocksLaterInstalls(t *testing.T) {
	const version = "1.5.0"
	t.Run("update interrupted after moving Journey aside", func(t *testing.T) {
		game := guildGame(t)
		_, transaction := crashedBundleTransaction(t, game, 20*time.Minute, true)
		u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
		result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
		if err != nil {
			t.Fatalf("an install interrupted 20 minutes ago still blocks later installs: %v", err)
		}
		// The interrupted update's Journey was put back first, so the new
		// install backed it up like any other existing folder.
		b, err := os.ReadFile(filepath.Join(result.BackupPath, "Interface/AddOns/Journey/private.lua"))
		if err != nil || !strings.Contains(string(b), "private Journey content") {
			t.Fatalf("the moved-aside Journey was not recovered into the new backup: %v", err)
		}
		if _, err := os.Stat(transaction); err != nil {
			t.Fatalf("recovery deleted the interrupted transaction: %v", err)
		}
	})
	t.Run("first install interrupted before any swap", func(t *testing.T) {
		game := freshClient(t)
		os.MkdirAll(filepath.Join(game, "Interface", "AddOns"), 0755)
		_, transaction := crashedBundleTransaction(t, game, 20*time.Minute, false)
		u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
		if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err != nil {
			t.Fatalf("an interrupted first install still blocks: %v", err)
		}
		if _, err := os.Stat(filepath.Join(transaction, "staged")); err != nil {
			t.Fatalf("recovery deleted files: %v", err)
		}
	})
	t.Run("a fresh lock still blocks", func(t *testing.T) {
		game := guildGame(t)
		lock, transaction := crashedBundleTransaction(t, game, time.Minute, true)
		u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
		if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err == nil || !strings.Contains(err.Error(), "backup lock") {
			t.Fatalf("a lock from a possibly running install was ignored: %v", err)
		}
		if _, err := os.Stat(lock); err != nil {
			t.Fatal("fresh lock removed")
		}
		if _, err := os.Stat(filepath.Join(transaction, "previous", "Interface", "AddOns", "Journey", "private.lua")); err != nil {
			t.Fatal("a fresh lock's transaction was touched")
		}
	})
	t.Run("a lock naming a folder outside RelaxCoachBackups is kept", func(t *testing.T) {
		game := guildGame(t)
		lock, _ := crashedBundleTransaction(t, game, 20*time.Minute, false)
		os.WriteFile(lock, []byte(t.TempDir()+"\n"), 0600)
		old := time.Now().Add(-20 * time.Minute)
		os.Chtimes(lock, old, old)
		u := guildUpdaterForVersion(t, bundleZip(t, version, nil), version)
		if _, err := u.apply(context.Background(), game, guildInfoForVersion(version)); err == nil {
			t.Fatal("cleared a lock that names an unknown transaction")
		}
		if _, err := os.Stat(lock); err != nil {
			t.Fatal("unknown lock removed")
		}
	})
}
