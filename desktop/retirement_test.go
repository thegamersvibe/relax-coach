package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func retirementFixture(t *testing.T) string {
	t.Helper()
	config := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("APPDATA", config)
	game := filepath.Join(t.TempDir(), "_forever_")
	if err := os.MkdirAll(filepath.Join(game, "Interface", "AddOns"), 0700); err != nil {
		t.Fatal(err)
	}
	return game
}
func retirementExists(t *testing.T, path string, want bool) {
	t.Helper()
	_, err := os.Lstat(path)
	if (err == nil) != want || (err != nil && !os.IsNotExist(err)) {
		t.Fatalf("%s exists=%v, want %v: %v", path, err == nil, want, err)
	}
}
func retirementClosed(context.Context) (bool, error) { return false, nil }

func TestRetirementExactAddonScope(t *testing.T) {
	for _, title := range []string{"Relax Journey", "Journey", ""} {
		t.Run(title, func(t *testing.T) {
			game := retirementFixture(t)
			for _, name := range []string{"ForeverCoach", "RelaxRaid", "RelaxQuestPlanner", "AHProfit", "OtherAddon", "ForeverCoachExtra"} {
				addonPut(t, game, "Interface/AddOns/"+name+"/keep.lua", "synthetic")
			}
			addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: "+title+"\n")
			addonPut(t, game, "Interface/Content/Journey-Content/guide.html", "keep")
			addonPut(t, game, "backups/ForeverCoach/keep.lua", "keep")
			result := retireGame(context.Background(), game, retirementClosed)
			for _, name := range []string{"ForeverCoach", "RelaxRaid", "RelaxQuestPlanner", "Journey"} {
				retirementExists(t, filepath.Join(game, "Interface", "AddOns", name), name == "Journey" && title != "Relax Journey")
			}
			for _, path := range []string{"Interface/AddOns/AHProfit/keep.lua", "Interface/AddOns/OtherAddon/keep.lua", "Interface/AddOns/ForeverCoachExtra/keep.lua", "Interface/Content/Journey-Content/guide.html", "backups/ForeverCoach/keep.lua"} {
				retirementExists(t, filepath.Join(game, path), true)
			}
			if len(result.Removed) < 3 {
				t.Fatalf("missing removal list: %+v", result)
			}
			again := retireGame(context.Background(), game, retirementClosed)
			if len(again.Removed) != 0 {
				t.Fatalf("rerun invented removals: %+v", again)
			}
		})
	}
}

func TestRetirementSkipsLinkedAddonAndAncestors(t *testing.T) {
	for _, relative := range []string{"Interface/AddOns/ForeverCoach", "Interface/AddOns", "Interface", "Interface/AddOns/ForeverCoach/nested"} {
		t.Run(relative, func(t *testing.T) {
			game := retirementFixture(t)
			outside := t.TempDir()
			addonPut(t, outside, "sentinel.lua", "keep")
			target := filepath.Join(game, relative)
			if err := os.RemoveAll(target); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, target); err != nil {
				t.Skip(err)
			}
			result := retireGame(context.Background(), game, retirementClosed)
			retirementExists(t, filepath.Join(outside, "sentinel.lua"), true)
			retirementExists(t, target, true)
			if len(result.Skipped) == 0 || !strings.Contains(strings.ToLower(strings.Join(result.Skipped, " ")), "link") {
				t.Fatalf("link not reported: %+v", result)
			}
		})
	}
}

func TestRetirementSavedVariableScopeAndWoWGate(t *testing.T) {
	for _, state := range []string{"closed", "open", "unreadable", "no detector"} {
		for _, ours := range []bool{true, false} {
			t.Run(state+"/journey="+fmt.Sprint(ours), func(t *testing.T) {
				game := retirementFixture(t)
				title := "Journey"
				if ours {
					title = "Relax Journey"
				}
				addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: "+title+"\n")
				addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "synthetic")
				names := []string{"ForeverCoachDB", "RelaxRaidDB", "RelaxRaidCharDB", "RelaxQuestPlannerDB", "Journey_Account", "Journey_Character"}
				keeps := []string{"AHProfit", "OtherAddon", "ForeverCoachDBExtra", "Journey_Unknown"}
				dirs := []string{"WTF/Account/ACCT/SavedVariables", "WTF/Account/ACCT/Realm/Character/SavedVariables", "WTF/Account/SECOND/OtherRealm/OtherCharacter/SavedVariables"}
				for _, dir := range dirs {
					for _, name := range append(append([]string{}, names...), keeps...) {
						for _, suffix := range []string{".lua", ".lua.bak"} {
							addonPut(t, game, dir+"/"+name+suffix, "synthetic save")
						}
					}
				}
				running := retirementClosed
				if state == "open" {
					running = func(context.Context) (bool, error) { return true, nil }
				}
				if state == "unreadable" {
					running = func(context.Context) (bool, error) { return false, errors.New("unreadable processes") }
				}
				if state == "no detector" {
					running = nil
				}
				result := retireGame(context.Background(), game, running)
				retirementExists(t, filepath.Join(game, "Interface/AddOns/ForeverCoach"), false)
				retirementExists(t, filepath.Join(game, "Interface/AddOns/Journey"), !ours)
				if result.SavedSettingsDeferred != (state != "closed") {
					t.Fatalf("WoW gate: %+v", result)
				}
				for _, dir := range dirs {
					for _, name := range names {
						for _, suffix := range []string{".lua", ".lua.bak"} {
							retirementExists(t, filepath.Join(game, dir, name+suffix), state != "closed" || (!ours && strings.HasPrefix(name, "Journey_")))
						}
					}
					for _, name := range keeps {
						for _, suffix := range []string{".lua", ".lua.bak"} {
							retirementExists(t, filepath.Join(game, dir, name+suffix), true)
						}
					}
				}
			})
		}
	}
}

func TestRetirementSavedVariablesNeverFollowLinks(t *testing.T) {
	for _, relative := range []string{"WTF", "WTF/Account", "WTF/Account/ACCT", "WTF/Account/ACCT/SavedVariables", "WTF/Account/ACCT/Realm", "WTF/Account/ACCT/Realm/Character", "WTF/Account/ACCT/Realm/Character/SavedVariables", "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"} {
		t.Run(relative, func(t *testing.T) {
			game := retirementFixture(t)
			outside := t.TempDir()
			addonPut(t, outside, "ForeverCoachDB.lua", "keep")
			target := filepath.Join(game, relative)
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				t.Fatal(err)
			}
			linkTo := outside
			if strings.HasSuffix(relative, ".lua") {
				linkTo = filepath.Join(outside, "ForeverCoachDB.lua")
			}
			if err := os.Symlink(linkTo, target); err != nil {
				t.Skip(err)
			}
			result := retireGame(context.Background(), game, retirementClosed)
			retirementExists(t, target, true)
			retirementExists(t, filepath.Join(outside, "ForeverCoachDB.lua"), true)
			if len(result.Skipped) == 0 {
				t.Fatalf("unreported linked save: %+v", result)
			}
		})
	}
}

func TestRetirementJourneyDeferredAcrossStarts(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(fmt.Sprint(replacement), func(t *testing.T) {
			game := retirementFixture(t)
			addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Relax Journey\n")
			save := "WTF/Account/ACCT/SavedVariables/Journey_Account.lua"
			addonPut(t, game, save, "keep while open")
			if err := saveGame(game); err != nil {
				t.Fatal(err)
			}
			first := retireGame(context.Background(), game, func(context.Context) (bool, error) { return true, nil })
			if !first.SavedSettingsDeferred {
				t.Fatal("no defer")
			}
			retirementExists(t, filepath.Join(game, "Interface/AddOns/Journey"), false)
			retirementExists(t, filepath.Join(game, save), true)
			if replacement {
				addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Journey\n")
			}
			retireGame(context.Background(), game, retirementClosed)
			retirementExists(t, filepath.Join(game, save), replacement)
			if savedGame() != game {
				t.Fatal("retirement changed the remembered game")
			}
			if replacement {
				if err := os.RemoveAll(filepath.Join(game, "Interface/AddOns/Journey")); err != nil {
					t.Fatal(err)
				}
				retireGame(context.Background(), game, retirementClosed)
				retirementExists(t, filepath.Join(game, save), true)
			}
		})
	}
}

func TestRetirementNoResolvedGameDoesNothing(t *testing.T) {
	result := retireGame(context.Background(), "", retirementClosed)
	if len(result.Removed) != 0 || !result.NoGame {
		t.Fatalf("unresolved game: %+v", result)
	}
}

func TestRetirementReceiptClearedAfterUndeferredCleanup(t *testing.T) {
	game := retirementFixture(t)
	addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Relax Journey\n")
	save := "WTF/Account/ACCT/SavedVariables/Journey_Account.lua"
	addonPut(t, game, save, "ours")
	retireGame(context.Background(), game, retirementClosed)
	retirementExists(t, filepath.Join(game, save), false)
	// A foreign Journey installed and removed later must never inherit the
	// finished cleanup's ownership receipt.
	addonPut(t, game, "Interface/AddOns/Journey/Journey.toc", "## Title: Journey\n")
	addonPut(t, game, save, "foreign")
	if err := os.RemoveAll(filepath.Join(game, "Interface/AddOns/Journey")); err != nil {
		t.Fatal(err)
	}
	if retirementKnownJourney(game, &retirementResult{}) {
		t.Fatal("receipt kept after undeferred cleanup")
	}
	retireGame(context.Background(), game, retirementClosed)
	retirementExists(t, filepath.Join(game, save), true)
}

func TestRetirementRejectsNonForeverGame(t *testing.T) {
	for _, name := range []string{"Documents", "World of Warcraft"} {
		t.Run(name, func(t *testing.T) {
			retirementFixture(t)
			game := filepath.Join(t.TempDir(), name)
			addonPut(t, game, "Interface/AddOns/ForeverCoach/Core.lua", "not a game")
			addonPut(t, game, "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua", "not a game")
			if err := saveGame(game); err != nil {
				t.Fatal(err)
			}
			result := retireGame(context.Background(), game, retirementClosed)
			if !result.NoGame || len(result.Removed) != 0 {
				t.Fatalf("non-Forever folder accepted: %+v", result)
			}
			retirementExists(t, filepath.Join(game, "Interface/AddOns/ForeverCoach"), true)
			retirementExists(t, filepath.Join(game, "WTF/Account/ACCT/SavedVariables/ForeverCoachDB.lua"), true)
		})
	}
}
