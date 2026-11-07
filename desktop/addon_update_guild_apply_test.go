package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var guildFixtureRoots = []string{"Interface/AddOns/ForeverCoach", "Interface/AddOns/Journey", "Interface/Content/Journey-Content"}

// Historical guild fixtures stay at 1.4.2, without Fun Mode files or TOC loads.
func guildEntries() []addonZipEntry {
	entries := addonFixtureEntries("1.4.2")
	for i := range entries {
		entries[i].name = "Interface/AddOns/" + entries[i].name
	}
	entries = append(entries,
		addonZipEntry{"Interface/AddOns/Journey/Journey.toc", "## Title: Relax Journey\n## Version: 4.00.04\n## SavedVariables: Journey_Account\n## SavedVariablesPerCharacter: Journey_Character\nMain.xml\n", 0644},
		addonZipEntry{"Interface/AddOns/Journey/Main.xml", "<Ui/>\n", 0644},
		addonZipEntry{"Interface/Content/Journey-Content/guides.html", "Synthetic guide bytes\x00\xff\r\n", 0644},
	)
	for i := 0; i < 300; i++ {
		entries = append(entries, addonZipEntry{fmt.Sprintf("Interface/AddOns/Journey/lib/sub/module%d.lua", i), "-- synthetic library\r\n", 0644})
	}
	for _, name := range []string{"RelaxCoach.exe", "README.md", "LICENSE-Coach", "Install-RelaxCoach.ps1", "Restore-RelaxCoach.ps1"} {
		entries = append(entries, addonZipEntry{name, "never install or execute this root extra", 0644})
	}
	return entries
}

func guildManifest(entries []addonZipEntry) map[string]any {
	rows := []map[string]any{}
	for _, e := range entries {
		if e.name != "manifest.json" && !e.mode.IsDir() {
			rows = append(rows, map[string]any{"path": e.name, "sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(e.body))), "bytes": len(e.body)})
		}
	}
	return map[string]any{"project": "Relax Coach", "kind": "guild-combined", "version": "1.4.2", "journeyVersion": "4.00.04", "contentVersion": "4.00.06", "files": rows}
}
func guildZip(t *testing.T, entries []addonZipEntry, manifest map[string]any) []byte {
	t.Helper()
	if manifest == nil {
		manifest = guildManifest(entries)
	}
	b, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return addonZip(t, append(entries, addonZipEntry{"manifest.json", string(b), 0644}))
}
func guildInfo() AddonReleaseInfo {
	return guildInfoForVersion("1.4.2")
}
func guildInfoForVersion(version string) AddonReleaseInfo {
	name := "RelaxCoach-Guild-" + version + ".zip"
	prefix := addonReleasePrefix + "download/v" + version + "/"
	return AddonReleaseInfo{Version: version, Available: true, AssetName: name, AssetURL: prefix + name, ChecksumURL: prefix + "SHA256SUMS.txt"}
}
func guildUpdater(t *testing.T, payload []byte) *addonUpdater {
	t.Helper()
	return guildUpdaterForVersion(t, payload, "1.4.2")
}
func guildUpdaterForVersion(t *testing.T, payload []byte, version string) *addonUpdater {
	t.Helper()
	info := guildInfoForVersion(version)
	return addonTestUpdater(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt") {
			fmt.Fprintf(w, "%x  %s\n", sha256.Sum256(payload), info.AssetName)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/"+info.AssetName) {
			w.Write(payload)
			return
		}
		t.Errorf("unexpected guild request %s", r.URL.Path)
		http.NotFound(w, r)
	})
}
func guildGame(t *testing.T) string {
	game := addonTestGame(t)
	addonPut(t, game, guildFixtureRoots[2]+"/old.html", "old signed guide bytes\x00\xff\r\n")
	addonPut(t, game, "Interface/AddOns/Unrelated/keep.lua", "unrelated")
	addonPut(t, game, "config.json", "unrelated app settings")
	return game
}
func guildRuntime(name string) bool {
	for _, root := range guildFixtureRoots {
		if strings.HasPrefix(name, root+"/") {
			return true
		}
	}
	return false
}

func TestGuildApplyReplacesOnlyThreeRuntimeTrees(t *testing.T) {
	entries := guildEntries()
	game := guildGame(t)
	before := addonTreeHashes(t, game)
	u := guildUpdater(t, guildZip(t, entries, nil))
	result, err := u.apply(context.Background(), game, guildInfo())
	if err != nil {
		t.Fatal(err)
	}
	if result.Version != "1.4.2" || result.BackupPath == "" || !strings.Contains(result.Message, "Journey") {
		t.Fatalf("bad result: %+v", result)
	}
	installedCount := 0
	for _, e := range entries {
		if !guildRuntime(e.name) {
			if _, err := os.Stat(filepath.Join(game, e.name)); e.name != "RelaxCoach.exe" && e.name != "README.md" && !os.IsNotExist(err) {
				t.Fatalf("installed root extra %s", e.name)
			}
			continue
		}
		installedCount++
		b, err := os.ReadFile(filepath.Join(game, filepath.FromSlash(e.name)))
		if err != nil || string(b) != e.body {
			t.Fatalf("runtime bytes %s: %v", e.name, err)
		}
	}
	if result.UpdatedFiles != installedCount {
		t.Fatalf("file count: %+v want %d", result, installedCount)
	}
	for name, hash := range before {
		root := game
		if guildRuntime(name) {
			root = result.BackupPath
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(b)) != hash {
			t.Fatalf("original not preserved: %s (%v)", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".addon-update.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock left after success: %v", err)
	}
	if got := InstalledCoachVersion(game); got != "1.4.2" {
		t.Fatal(got)
	}
	// A current bundle must not reinstall or touch any retained backup.
	after := addonTreeHashes(t, game)
	if _, err := u.apply(context.Background(), game, guildInfo()); err == nil {
		t.Fatal("reinstalled current bundle")
	}
	if !reflect.DeepEqual(after, addonTreeHashes(t, game)) {
		t.Fatal("current bundle changed files")
	}
}
