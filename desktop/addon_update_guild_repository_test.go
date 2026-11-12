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

// Exercise the repository's real, signed runtime bytes without executing them.
// The ZIP and temporary game stay in memory / t.TempDir, never in the checkout.
// This is not evidence for the parent's separately built release archive/EXE.
func TestGuildRepositoryRuntimeBytes(t *testing.T) {
	const version = "1.5.4"
	entries := []addonZipEntry{}
	sources := []string{"../addon/ForeverCoach", "../addon/Journey", "../content/Journey-Content"}
	roots := append([]string{}, guildFixtureRoots...)
	if addonBundleRelease(version) {
		sources, roots = append(sources, "../addon/RelaxQuestPlanner"), append(roots, addonPlannerRoot)
	}
	for i, source := range sources {
		err := filepath.WalkDir(source, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("unexpected source type: %s", p)
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(source, p)
			if err != nil {
				return err
			}
			entries = append(entries, addonZipEntry{roots[i] + "/" + filepath.ToSlash(relative), string(b), 0644})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	count := len(entries)
	for _, name := range []string{"RelaxCoach.exe", "README.md", "LICENSE-Coach", "Install-RelaxCoach.ps1", "Restore-RelaxCoach.ps1"} {
		entries = append(entries, addonZipEntry{name, "inert fixture extra; never executed", 0644})
	}
	manifest := guildManifest(entries)
	manifest["version"] = version
	if addonBundleRelease(version) {
		toc, err := os.ReadFile("../addon/RelaxQuestPlanner/RelaxQuestPlanner.toc")
		if err != nil {
			t.Fatal(err)
		}
		meta, _, err := addonTOCMetadata(toc)
		if err != nil {
			t.Fatal(err)
		}
		manifest["plannerVersion"] = meta["version"]
	}
	payload := guildZip(t, entries, manifest)
	u := guildUpdaterForVersion(t, payload, version)
	game := guildGame(t)
	result, err := u.apply(context.Background(), game, guildInfoForVersion(version))
	if err != nil {
		t.Fatal(err)
	}
	if result.UpdatedFiles != count {
		t.Fatalf("installed %d, expected %d", result.UpdatedFiles, count)
	}
	actual := addonTreeHashes(t, game)
	for _, e := range entries {
		if !guildRuntime(e.name) {
			continue
		}
		if actual[e.name] != fmt.Sprintf("%x", sha256.Sum256([]byte(e.body))) {
			t.Fatalf("real runtime byte mismatch: %s", e.name)
		}
	}
	for name := range actual {
		if strings.HasSuffix(name, ".ps1") {
			t.Fatal("bundled installer written to game")
		}
	}
	t.Logf("Verified %d actual repository runtime files; fixture archive %d bytes; archive SHA256 %x", count, len(payload), sha256.Sum256(payload))
}
