package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Independent policy oracle: keep the twelve-file legacy fixture frozen.
var funModeFixtureFiles = []string{
	"Reactions.lua",
	"Media/Reactions/bags_1.mp3",
	"Media/Reactions/bags_2.mp3",
	"Media/Reactions/campfire.mp3",
	"Media/Reactions/death.mp3",
	"Media/Reactions/level_up.mp3",
	"Media/Reactions/level_up.tga",
}

// From 1.5.6 the one campfire song is two songs that take turns.
var campfireSongsFixtureFiles = []string{
	"Reactions.lua",
	"Media/Reactions/bags_1.mp3",
	"Media/Reactions/bags_2.mp3",
	"Media/Reactions/campfire_1.mp3",
	"Media/Reactions/campfire_2.mp3",
	"Media/Reactions/death.mp3",
	"Media/Reactions/level_up.mp3",
	"Media/Reactions/level_up.tga",
}

func funModeFilesFor(version string) []string {
	v, err := parseAddonVersion(version)
	songs, _ := parseAddonVersion("1.5.6")
	if err == nil && !songs.newerThan(v) {
		return campfireSongsFixtureFiles
	}
	return funModeFixtureFiles
}

func funModeEntries(version string) []addonZipEntry {
	entries := guildEntries()
	entries[0].body = addonFixtureEntries(version)[0].body + "Reactions.lua\n"
	for _, name := range funModeFilesFor(version) {
		entries = append(entries, addonZipEntry{guildFixtureRoots[0] + "/" + name, "fixture " + name + "\x00\xff\r\n", 0644})
	}
	if addonBundleRelease(version) {
		entries = append(entries, plannerFixtureEntries()...)
	}
	return entries
}

func funModeZip(t *testing.T, entries []addonZipEntry, version string) []byte {
	t.Helper()
	manifest := guildManifest(entries)
	manifest["version"] = version
	if addonBundleRelease(version) {
		manifest["plannerVersion"] = plannerFixtureVersion
	}
	return guildZip(t, entries, manifest)
}

func TestGuildFunModeExtractsExactCoachRuntime(t *testing.T) {
	for _, version := range []string{"1.4.3", "1.4.3+reviewed", "1.4.4", "1.4.5", "1.5.0", "1.5.1", "1.5.2", "1.5.3", "1.5.4", "1.5.5", "1.5.6", "1.4.10", "2.0.0"} {
		t.Run(version, func(t *testing.T) {
			entries := funModeEntries(version)
			stage := filepath.Join(t.TempDir(), "staged")
			digests, err := newAddonUpdater().extract(context.Background(), funModeZip(t, entries, version), stage, version)
			if err != nil {
				t.Fatal(err)
			}
			count := 0
			for _, e := range entries {
				if !guildRuntime(e.name) {
					if _, err := os.Stat(filepath.Join(stage, e.name)); !os.IsNotExist(err) {
						t.Fatalf("root extra extracted: %s (%v)", e.name, err)
					}
					continue
				}
				count++
				got, err := os.ReadFile(filepath.Join(stage, filepath.FromSlash(e.name)))
				if err != nil || string(got) != e.body {
					t.Fatalf("runtime bytes changed: %s (%v)", e.name, err)
				}
				if digests[e.name] != (addonFileDigest{sha256.Sum256([]byte(e.body)), int64(len(e.body))}) {
					t.Fatalf("wrong digest for %s", e.name)
				}
			}
			if len(digests) != count {
				t.Fatalf("runtime inventory: got %d want %d", len(digests), count)
			}
			coach := addonTreeHashes(t, filepath.Join(stage, filepath.FromSlash(guildFixtureRoots[0])))
			want := 19 + len(funModeFilesFor(version)) - len(funModeFixtureFiles) // 20 from 1.5.6: two campfire songs
			if len(coach) != want {
				t.Fatalf("Coach inventory: got %d want %d", len(coach), want)
			}
			if err := verifyAddonTree(context.Background(), stage, digests); err != nil {
				t.Fatal(fmt.Errorf("staged tree verification: %w", err))
			}
		})
	}
}

func TestGuildFunModeRequiresEveryCoachFile(t *testing.T) {
	for _, name := range append(append([]string{}, addonFixtureFiles...), funModeFixtureFiles...) {
		for _, directory := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/directory=%t", name, directory), func(t *testing.T) {
				entries := funModeEntries("1.4.3")
				for i := range entries {
					if entries[i].name == guildFixtureRoots[0]+"/"+name {
						if directory {
							entries[i].name += "/"
							entries[i].body = ""
							entries[i].mode = os.ModeDir | 0755
						} else {
							entries = append(entries[:i], entries[i+1:]...)
						}
						break
					}
				}
				stage := filepath.Join(t.TempDir(), "staged")
				_, err := newAddonUpdater().extract(context.Background(), funModeZip(t, entries, "1.4.3"), stage, "1.4.3")
				if err == nil || !strings.Contains(err.Error(), "incomplete addon archive: missing "+name) {
					t.Fatalf("missing regular file %s not rejected: %v", name, err)
				}
				if _, err := os.Stat(stage); !os.IsNotExist(err) {
					t.Fatalf("incomplete archive was staged: %v", err)
				}
			})
		}
	}
}
