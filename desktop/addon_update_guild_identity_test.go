package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type guildRoundTripFunc func(*http.Request) (*http.Response, error)

func (f guildRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGuildRevalidatesBackupAncestorsBeforeAnyMove(t *testing.T) {
	game := guildGame(t)
	outside := t.TempDir()
	addonPut(t, outside, "sentinel.txt", "external")
	u := guildUpdater(t, guildZip(t, guildEntries(), nil))
	checks, moves := 0, 0
	u.running = func(context.Context) (bool, error) {
		checks++
		if checks == 2 {
			txs, err := filepath.Glob(filepath.Join(game, "RelaxCoachBackups", "ForeverCoach-*"))
			if err != nil || len(txs) != 1 {
				t.Fatal("transaction missing", err)
			}
			parent := filepath.Join(txs[0], "previous", "Interface", "AddOns")
			if err := os.Rename(parent, parent+"-original"); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, parent); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
		}
		return false, nil
	}
	u.rename = func(a, b string) error { moves++; return os.Rename(a, b) }
	_, err := u.apply(context.Background(), game, guildInfo())
	if err == nil || moves != 0 {
		t.Fatalf("followed replaced backup ancestor: %d moves, %v", moves, err)
	}
	if got := addonTreeHashes(t, outside); len(got) != 1 {
		t.Fatalf("external directory written: %v", got)
	}
	if InstalledCoachVersion(game) != "1.4.0" {
		t.Fatal("changed Coach on ancestor refusal")
	}
}

func TestGuildSourceAndAncestorChangesDuringDownload(t *testing.T) {
	for _, relative := range []string{"Interface", "Interface/AddOns", "Interface/Content", "Interface/AddOns/ForeverCoach", "Interface/AddOns/Journey", "Interface/Content/Journey-Content"} {
		for _, linked := range []bool{false, true} {
			t.Run(relative+map[bool]string{false: "/replacement", true: "/link"}[linked], func(t *testing.T) {
				game := guildGame(t)
				outside := t.TempDir()
				addonPut(t, outside, "sentinel.txt", "external")
				u := guildUpdater(t, guildZip(t, guildEntries(), nil))
				base := u.client.Transport
				mutated := false
				moves := 0
				u.client.Transport = guildRoundTripFunc(func(r *http.Request) (*http.Response, error) {
					resp, err := base.RoundTrip(r)
					if !mutated {
						mutated = true
						p := filepath.Join(game, filepath.FromSlash(relative))
						if e := os.Rename(p, p+"-original"); e != nil {
							t.Fatal(e)
						}
						if linked {
							if e := os.Symlink(outside, p); e != nil {
								t.Skipf("symlink unavailable: %v", e)
							}
						} else {
							if e := os.Mkdir(p, 0755); e != nil {
								t.Fatal(e)
							}
						}
					}
					return resp, err
				})
				u.rename = func(a, b string) error { moves++; return os.Rename(a, b) }
				if _, err := u.apply(context.Background(), game, guildInfo()); err == nil || moves != 0 {
					t.Fatalf("stale install: moves=%d err=%v", moves, err)
				}
				if len(addonTreeHashes(t, outside)) != 1 {
					t.Fatal("external target changed")
				}
			})
		}
	}
}

func TestGuildGameGuardAndInstalledVersion(t *testing.T) {
	// A running WoW is no longer a guard (1.5.2): see addon_game_open_test.go.
	for _, mode := range []string{"unknown", "current", "newer", "existing lock"} {
		t.Run(mode, func(t *testing.T) {
			game := guildGame(t)
			u := guildUpdater(t, guildZip(t, guildEntries(), nil))
			if mode == "unknown" || mode == "current" || mode == "newer" {
				version := map[string]string{"unknown": "garbage", "current": "1.4.2", "newer": "1.5.0"}[mode]
				addonPut(t, game, "Interface/AddOns/ForeverCoach/ForeverCoach.toc", "## Version: "+version+"\n")
			}
			if mode == "existing lock" {
				addonPut(t, game, "RelaxCoachBackups/.addon-update.lock", "other process")
			}
			before := addonTreeHashes(t, game)
			if _, err := u.apply(context.Background(), game, guildInfo()); err == nil {
				t.Fatal("guard failed")
			}
			if !reflect.DeepEqual(before, addonTreeHashes(t, game)) {
				t.Fatal("guard failure changed files")
			}
		})
	}
}

func TestGuildConcurrentUpdateRefused(t *testing.T) {
	game := guildGame(t)
	u := guildUpdater(t, guildZip(t, guildEntries(), nil))
	base := u.client.Transport
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	u.client.Transport = guildRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt") {
			close(entered)
			<-release
		}
		return base.RoundTrip(r)
	})
	go func() { _, err := u.apply(context.Background(), game, guildInfo()); done <- err }()
	<-entered
	_, secondErr := guildUpdater(t, guildZip(t, guildEntries(), nil)).apply(context.Background(), game, guildInfo())
	close(release)
	firstErr := <-done
	if firstErr != nil {
		t.Fatal(firstErr)
	}
	if secondErr == nil || !strings.Contains(secondErr.Error(), "another addon update") {
		t.Fatal("concurrent install admitted", secondErr)
	}
}
