package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LayerEnemies in More addons: buiden's repository, tagged alpha. The entry is
// fixed like the others; only the Alpha flag is new, and it changes display only.
func TestExtraAddonLayerEnemiesEntry(t *testing.T) {
	a, ok := extraAddonByID("layerenemies")
	if !ok || a.Name != "LayerEnemies" || a.Repo != "buiden/LayerEnemies" || a.Folder != "LayerEnemies" || a.Title != "LayerEnemies" {
		t.Fatalf("unexpected LayerEnemies entry: %+v", a)
	}
	if a.Blurb != "Enemy players seen by you and by War Effort, HeadHunter and Spy users, plus one-click layer hops." {
		t.Fatalf("blurb %q", a.Blurb)
	}
	if !a.Alpha {
		t.Fatal("LayerEnemies is not tagged alpha")
	}
	for _, other := range extraAddons {
		if other.ID != "layerenemies" && other.Alpha {
			t.Fatalf("%s is tagged alpha", other.ID)
		}
	}
	if !extraLatestURL("https://api.github.com/repos/buiden/LayerEnemies/releases/latest") {
		t.Fatal("LayerEnemies release URL is not trusted")
	}
	if a.assetName("1.0.0") != "LayerEnemies-1.0.0.zip" || a.releasePrefix() != "https://github.com/buiden/LayerEnemies/releases/" {
		t.Fatalf("asset %q prefix %q", a.assetName("1.0.0"), a.releasePrefix())
	}
}

// The alpha flag reaches the UI through the check result and both bridge
// shapes (the offline list and a check), and nothing else about the offer moves.
func TestExtraAddonAlphaReachesTheUI(t *testing.T) {
	s := extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3"))
	u := newExtraUpdater(t, s)
	info, _ := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
	if info.Alpha {
		t.Fatal("Forever Maps reported as alpha")
	}
	if j, _ := json.Marshal(ExtraAddonInfo{Alpha: true}); !strings.Contains(string(j), `"alpha":true`) {
		t.Fatalf("ExtraAddonInfo JSON has no alpha: %s", j)
	}
	game := extraGame(t)
	app := &App{}
	app.setGame(game)
	b := newNativeBridge(app, false, nativeServices{
		checkExtra: func(_ context.Context, id, current string) (ExtraAddonInfo, error) {
			return ExtraAddonInfo{ID: id, Version: "1.0.0", InstalledVersion: current, Available: true, Install: current == "0.0.0"}, nil
		},
		applyExtra: func(_ context.Context, g string, info ExtraAddonInfo) (AddonUpdateResult, error) {
			return AddonUpdateResult{Version: info.Version}, nil
		},
	})
	defer b.Close()
	list, err := b.Call("extraList", nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, o := range list.([]nativeExtraOffer) {
		if o.Alpha != (o.ID == "layerenemies") {
			t.Fatalf("list alpha wrong for %s: %v", o.ID, o.Alpha)
		}
		seen = seen || o.ID == "layerenemies"
	}
	if !seen {
		t.Fatal("LayerEnemies missing from the list")
	}
	raw := func(s string) []json.RawMessage { j, _ := json.Marshal(s); return []json.RawMessage{j} }
	offer, err := b.Call("checkExtra", raw("layerenemies"))
	if err != nil || !offer.(nativeExtraOffer).Alpha || offer.(nativeExtraOffer).Name != "LayerEnemies" {
		t.Fatalf("check %+v %v", offer, err)
	}
	offer, err = b.Call("checkExtra", raw("rankup"))
	if err != nil || offer.(nativeExtraOffer).Alpha {
		t.Fatalf("rankup check %+v %v", offer, err)
	}
}

// The real v1.0.0 asset (set LAYERENEMIES_ZIP to its path): 30 entries under
// LayerEnemies/, TOC title and version, and a load list extractExtra accepts.
func TestExtraAddonLayerEnemiesRealZip(t *testing.T) {
	path := os.Getenv("LAYERENEMIES_ZIP")
	if path == "" {
		t.Skip("LAYERENEMIES_ZIP not set")
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	if want := os.Getenv("LAYERENEMIES_SHA256"); want != "" && hex.EncodeToString(sum[:]) != want {
		t.Fatalf("zip sha256 %x, want %s", sum, want)
	}
	r, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil || len(r.File) != 30 {
		t.Fatalf("zip entries %v %v", len(r.File), err)
	}
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "LayerEnemies/") {
			t.Fatalf("entry outside LayerEnemies/: %s", f.Name)
		}
	}
	a, _ := extraAddonByID("layerenemies")
	stage := t.TempDir()
	n, err := newAddonUpdater().extractExtra(context.Background(), payload, stage, a, "1.0.0")
	if err != nil || n != 30 {
		t.Fatalf("extractExtra: %d %v", n, err)
	}
	toc, err := os.ReadFile(filepath.Join(stage, "LayerEnemies", "LayerEnemies.toc"))
	if err != nil || !strings.Contains(string(toc), "## Title: LayerEnemies") || !strings.Contains(string(toc), "## Version: 1.0.0") {
		t.Fatalf("toc %v", err)
	}
	if _, err := newAddonUpdater().extractExtra(context.Background(), payload, t.TempDir(), a, "1.0.1"); err == nil {
		t.Fatal("accepted the zip for another version")
	}
}
