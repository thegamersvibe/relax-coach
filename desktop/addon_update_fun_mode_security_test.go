package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func assertFunModeRejected(t *testing.T, entries []addonZipEntry, manifest map[string]any, contains string) {
	t.Helper()
	if manifest == nil {
		manifest = guildManifest(entries)
		manifest["version"] = "1.4.3"
	}
	game := guildGame(t)
	before := addonTreeHashes(t, game)
	u := guildUpdaterForVersion(t, guildZip(t, entries, manifest), "1.4.3")
	result, err := u.apply(context.Background(), game, guildInfoForVersion("1.4.3"))
	if err == nil || result.Version != "" || !strings.Contains(err.Error(), contains) {
		t.Fatalf("expected %q refusal, got %+v %v", contains, result, err)
	}
	if !reflect.DeepEqual(before, addonTreeHashes(t, game)) {
		t.Fatal("rejected Fun Mode bundle changed runtime, helper, settings or saves")
	}
}

func TestGuildFunModeRejectsAlteredMediaPaths(t *testing.T) {
	for _, name := range funModeFixtureFiles {
		t.Run("case/"+name, func(t *testing.T) {
			entries := funModeEntries("1.4.3")
			for i := range entries {
				if entries[i].name == guildFixtureRoots[0]+"/"+name {
					entries[i].name = guildFixtureRoots[0] + "/" + strings.TrimSuffix(name, filepath.Base(name)) + strings.ToUpper(filepath.Base(name))
				}
			}
			assertFunModeRejected(t, entries, nil, "allowlist")
		})
	}
	for _, name := range []string{
		"Media/Reactions/extra.mp3", "Media/Reactions/extra.tga", "Media/Reactions/extra.lua",
		"Media/Reactions/level_up.wav", "Media/Reactions/nested/bags_1.mp3",
		"Media/bags_1.mp3", "bags_1.mp3", "Media/Reactions/../evil.lua",
		"Media/Reactions/bags_1.mp3:evil", "Media/Reactions/bags_1.mp3.",
	} {
		t.Run("extra/"+name, func(t *testing.T) {
			entries := append(funModeEntries("1.4.3"), addonZipEntry{guildFixtureRoots[0] + "/" + name, "unapproved", 0644})
			assertFunModeRejected(t, entries, nil, "")
		})
	}
	for _, mode := range []os.FileMode{os.ModeSymlink | 0777, os.ModeNamedPipe | 0600} {
		entries := funModeEntries("1.4.3")
		entries[len(entries)-1].mode = mode
		assertFunModeRejected(t, entries, nil, "special")
	}
}

func TestGuildFunModeRejectsInvalidTOC(t *testing.T) {
	for _, tc := range []struct{ name, old, replacement, errorText string }{
		{"missing reaction load", "Reactions.lua\n", "", "missing module Reactions.lua"},
		{"duplicate reaction load", "Reactions.lua\n", "Reactions.lua\nReactions.lua\n", "duplicate TOC load"},
		{"case changed load", "Reactions.lua", "reactions.lua", "TOC load path"},
		{"media load", "Reactions.lua", "Media/Reactions/level_up.tga", "TOC load path"},
		{"escaped load", "Reactions.lua", "../Reactions.lua", "TOC load path"},
		{"old declared version", "## Version: 1.4.3", "## Version: 1.4.2", "Version"},
		{"newer declared version", "## Version: 1.4.3", "## Version: 1.4.4", "Version"},
		{"case changed Version", "## Version: 1.4.3", "## version: 1.4.3", "Version"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries := funModeEntries("1.4.3")
			entries[0].body = strings.Replace(entries[0].body, tc.old, tc.replacement, 1)
			assertFunModeRejected(t, entries, nil, tc.errorText)
		})
	}
}

func TestGuildFunModeManifestStillBindsMedia(t *testing.T) {
	for _, kind := range []string{"media hash", "media size", "missing row", "extra row", "wrong version", "extra field"} {
		t.Run(kind, func(t *testing.T) {
			entries := funModeEntries("1.4.3")
			manifest := guildManifest(entries)
			manifest["version"] = "1.4.3"
			rows := manifest["files"].([]map[string]any)
			switch kind {
			case "media hash":
				rows[len(rows)-1]["sha256"] = strings.Repeat("0", 64)
			case "media size":
				rows[len(rows)-1]["bytes"] = 1
			case "missing row":
				manifest["files"] = rows[:len(rows)-1]
			case "extra row":
				manifest["files"] = append(rows, map[string]any{"path": guildFixtureRoots[0] + "/Media/Reactions/extra.mp3", "sha256": strings.Repeat("0", 64), "bytes": 1})
			case "wrong version":
				manifest["version"] = "1.4.2"
			case "extra field":
				manifest["allowExtraMedia"] = true
			}
			assertFunModeRejected(t, entries, manifest, "manifest")
		})
	}
}

func TestAddonFunModePreservesLegacyPackages(t *testing.T) {
	// Exercise legacy after a new extraction to catch accidental global map mutation.
	stage := filepath.Join(t.TempDir(), "new")
	if _, err := newAddonUpdater().extract(context.Background(), funModeZip(t, funModeEntries("1.4.3"), "1.4.3"), stage, "1.4.3"); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, name := range addonFixtureFiles {
		want[name] = true
	}
	if !reflect.DeepEqual(addonPublicFiles, want) {
		t.Fatal("legacy twelve-file policy changed")
	}
	for _, version := range []string{"1.4.0", "1.4.1", "1.4.2", "1.4.2+legacy"} {
		t.Run(version, func(t *testing.T) {
			entries := addonFixtureEntries(version)
			coachRoot := ""
			payload := addonZip(t, entries)
			if strings.HasPrefix(version, "1.4.2") {
				entries = guildEntries()
				entries[0].body = addonFixtureEntries(version)[0].body
				payload = funModeZip(t, entries, version)
				coachRoot = guildFixtureRoots[0]
			}
			stage := filepath.Join(t.TempDir(), "staged")
			if _, err := newAddonUpdater().extract(context.Background(), payload, stage, version); err != nil {
				t.Fatal(err)
			}
			if got := addonTreeHashes(t, filepath.Join(stage, filepath.FromSlash(coachRoot))); len(got) != 12 {
				t.Fatalf("legacy Coach inventory: got %d want 12", len(got))
			}
			if err := validateAddonTOC([]byte(addonFixtureEntries(version)[0].body+"Reactions.lua\n"), version); err == nil {
				t.Fatal("legacy TOC admitted Reactions.lua")
			}
			for _, name := range funModeFixtureFiles {
				t.Run("reject/"+name, func(t *testing.T) {
					root := "ForeverCoach"
					if coachRoot != "" {
						root = coachRoot
					}
					bad := append(append([]addonZipEntry{}, entries...), addonZipEntry{root + "/" + name, "unapproved for this release", 0644})
					payload := addonZip(t, bad)
					if coachRoot != "" {
						payload = funModeZip(t, bad, version)
					}
					stage := filepath.Join(t.TempDir(), "staged")
					if _, err := newAddonUpdater().extract(context.Background(), payload, stage, version); err == nil || !strings.Contains(err.Error(), "allowlist") {
						t.Fatalf("old release admitted %s: %v", name, err)
					}
					if _, err := os.Stat(stage); !os.IsNotExist(err) {
						t.Fatalf("old release with new files was staged: %v", err)
					}
				})
			}
		})
	}
}
