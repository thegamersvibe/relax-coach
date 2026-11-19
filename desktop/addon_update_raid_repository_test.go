package main

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// R1/R9a/R9b cross-check: never substitute the synthetic raidFixtureEntries for
// the files the builder actually ships. Stage them, then use the app's checks.
func TestRaidRepositoryTOCMatchesAppContract(t *testing.T) {
	source := filepath.Join("..", "addon", "RelaxRaid")
	stage := t.TempDir()
	runtime := map[string]addonFileDigest{}
	err := filepath.WalkDir(source, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(source, name)
		if err != nil {
			return err
		}
		archivePath := addonRaidRoot + "/" + filepath.ToSlash(rel)
		if err := addonSafeZipName(archivePath); err != nil {
			return err
		}
		if !addonGuildPath(archivePath, false, nil, addonGuildRootsFor(addonRaidMinimumVersion)) {
			t.Fatalf("shipped raid file is outside the app allowlist: %s", archivePath)
		}
		if strings.EqualFold(filepath.Ext(name), ".toc") && rel != "RelaxRaid.toc" {
			t.Fatalf("shipped raid has a second TOC: %s", rel)
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		addonPut(t, stage, archivePath, string(data))
		runtime[archivePath] = addonFileDigest{sha256.Sum256(data), int64(len(data))}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	toc, err := os.ReadFile(filepath.Join(source, "RelaxRaid.toc"))
	if err != nil {
		t.Fatal(err)
	}
	meta, _, err := addonTOCMetadata(toc)
	if err != nil || meta["version"] == "" {
		t.Fatalf("real raidVersion missing: %v", err)
	}
	if err := addonCheckRaidTOC(stage, runtime, meta["version"]); err != nil {
		t.Fatalf("shipped RelaxRaid TOC rejected by the app: %v", err)
	}
	// Negative control changes only the staged copy, never addon/RelaxRaid.
	addonPut(t, stage, addonRaidRoot+"/RelaxRaid.toc", strings.Replace(string(toc), "Title: Relax Raid", "Title: |cffffffffRelax Raid|r", 1))
	if err := addonCheckRaidTOC(stage, runtime, meta["version"]); err == nil {
		t.Fatal("app accepted a colour-coded title in the staged real TOC")
	}
}
