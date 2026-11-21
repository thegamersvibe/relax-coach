package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A picker result belongs to the account that started it; a picker failure
// still rechecks admission (review M1/M2).
func TestRaidPagePickerOutcomeBinding(t *testing.T) {
	member := func() string { return strings.Replace(officerSave(), `"officer"`, `"member"`, 1) }
	for _, method := range []string{"raidChooseLog", "raidChooseOfficer"} {
		for _, scene := range []string{"error-demotion", "error-admitted", "switch-cancel", "switch-path", "switch-back-path", "account-field-change",
			"night-demote-cancel", "night-demote-path", "aba-demote-cancel", "aba-demote-path"} {
			t.Run(method+"/"+scene, func(t *testing.T) {
				game, a, b := raidOpeningFixture(t)
				accountSave(t, game, "B#2", "Beta")
				raidRankSave(t, game, "B#2", member())
				log := filepath.Join(t.TempDir(), "invented-log.txt")
				raidOpeningLog(t, log, 99)
				raidCallHTML(t, b, "raidOpen", log)
				b.services.chooseFile = func(context.Context, string) (string, error) {
					switch scene {
					case "error-demotion":
						raidRankSave(t, game, "A#1", member())
						return "", errors.New("invented picker failure")
					case "error-admitted":
						return "", errors.New("invented picker failure")
					case "account-field-change":
						// Account rescans can reassign the account without a revision bump.
						a.mu.Lock()
						a.account = "B#2"
						a.mu.Unlock()
						return log, nil
					}
					if strings.HasPrefix(scene, "night-demote") || strings.HasPrefix(scene, "aba-demote") {
						if strings.HasPrefix(scene, "night-demote") {
							// Another night opened meanwhile: the revision moves, A stays selected.
							raidCallHTML(t, b, "raidOpen", log)
						} else {
							for _, id := range []string{"B#2", "A#1"} {
								if _, err := b.Call("selectAccount", nativeArgs(id)); err != nil {
									t.Fatal(err)
								}
							}
						}
						raidRankSave(t, game, "A#1", member())
						if strings.HasSuffix(scene, "cancel") {
							return "", nil
						}
						return log, nil
					}
					if _, err := b.Call("selectAccount", nativeArgs("B#2")); err != nil {
						t.Fatal(err)
					}
					if scene == "switch-back-path" {
						if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
							t.Fatal(err)
						}
					}
					if scene == "switch-cancel" {
						return "", nil
					}
					return log, nil
				}
				got, err := b.Call(method, nil)
				msg := ""
				if err != nil {
					msg = err.Error()
				}
				if strings.HasPrefix(scene, "night-demote") || strings.HasPrefix(scene, "aba-demote") {
					// The same account is still selected: its real lock wins over staleness.
					if got != nil || !strings.HasPrefix(msg, "Raids locked:") {
						t.Fatalf("demoted starter after a revision change: result=%v err=%q, want refusal", got, msg)
					}
					if a.raidSelection != nil {
						t.Fatal("refused stale picker kept the night")
					}
					return
				}
				switch scene {
				case "error-demotion":
					if got != nil || !strings.HasPrefix(msg, "Raids locked:") {
						t.Fatalf("picker failure after demotion: result=%v err=%q, want refusal", got, msg)
					}
					if a.raidSelection != nil {
						t.Fatal("refused picker failure kept the night")
					}
					return
				case "error-admitted":
					if got != nil || msg != "invented picker failure" {
						t.Fatalf("admitted picker failure: result=%v err=%q", got, msg)
					}
					return
				}
				if got != nil || msg != "raid selection changed while choosing a file" {
					t.Fatalf("cross-account picker result: result=%v err=%q, want stale", got, msg)
				}
				if strings.HasPrefix(msg, "Raids locked") {
					t.Fatal("stale result carried a lock verdict")
				}
				if scene != "switch-back-path" && scene != "account-field-change" {
					if _, err := b.Call("selectAccount", nativeArgs("A#1")); err != nil {
						t.Fatal(err)
					}
				}
				if scene == "account-field-change" {
					a.mu.Lock()
					a.account = "A#1"
					a.mu.Unlock()
				}
				if _, err := b.Call("raidPage", nil); err != nil {
					t.Fatalf("A not admitted after returning: %v", err)
				}
			})
		}
	}
}
