package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const extraTestTOC = "## Interface: 16001\n## Title: Forever Maps\n## Version: %s\n## SavedVariablesPerCharacter: ForeverMapsCharDB\n\nData\\World.lua\nCore.lua\n"

func extraZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(body))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func extraGoodZip(t *testing.T, version string) []byte {
	return extraZip(t, map[string]string{
		"ForeverMaps/ForeverMaps.toc": strings.Replace(extraTestTOC, "%s", version, 1),
		"ForeverMaps/Core.lua":        "-- core " + version,
		"ForeverMaps/Data/World.lua":  "-- world",
	})
}

type extraServer struct {
	tag     string
	payload []byte
	sums    string
	assets  []string
}

func newExtraUpdater(t *testing.T, s *extraServer) *addonUpdater {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/repos/buiden/ForeverMaps/releases/latest":
			prefix := "https://github.com/buiden/ForeverMaps/releases/download/" + s.tag + "/"
			var assets []map[string]string
			for _, a := range s.assets {
				assets = append(assets, map[string]string{"name": a, "browser_download_url": prefix + a})
			}
			json.NewEncoder(w).Encode(map[string]any{"tag_name": s.tag, "body": "notes", "assets": assets})
		case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
			w.Write([]byte(s.sums))
		case strings.HasSuffix(r.URL.Path, ".zip"):
			w.Write(s.payload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	server, _ := url.Parse(srv.URL)
	u := newAddonUpdater()
	u.client.Transport = addonTestTransport{server: server, base: srv.Client().Transport}
	u.running = func(context.Context) (bool, error) { return false, nil }
	return u
}

func extraRelease(t *testing.T, version string, payload []byte) *extraServer {
	sum := sha256.Sum256(payload)
	name := "ForeverMaps-" + version + ".zip"
	return &extraServer{tag: "v" + version, payload: payload,
		sums:   hex.EncodeToString(sum[:]) + "  " + name + "\n",
		assets: []string{name, "SHA256SUMS.txt"}}
}

func extraGame(t *testing.T) string {
	game := t.TempDir()
	if err := os.MkdirAll(filepath.Join(game, "Interface", "AddOns", "SomeoneElse"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(game, "Interface", "AddOns", "SomeoneElse", "keep.lua"), []byte("x"), 0644)
	return game
}

func TestExtraAddonMissingReleaseIsStatusNotError(t *testing.T) {
	s := extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3"))
	u := newExtraUpdater(t, s)
	srv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(srv.Close)
	server, _ := url.Parse(srv.URL)
	u.client.Transport = addonTestTransport{server: server, base: srv.Client().Transport}
	info, err := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
	if err != nil {
		t.Fatalf("404 on releases/latest must not be an error, got %v", err)
	}
	if info.Status != "No stable Forever Maps release is published." || info.AssetURL != "" {
		t.Fatalf("unexpected info for missing release: %+v", info)
	}
}

func TestExtraAddonFreshInstallAndUpdate(t *testing.T) {
	game := extraGame(t)
	a, _ := extraAddonByID("forevermaps")
	if v := InstalledExtraVersion(game, a); v != "0.0.0" {
		t.Fatalf("expected not installed, got %s", v)
	}
	u := newExtraUpdater(t, extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3")))
	info, err := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
	if err != nil || !info.Available || !info.Install || info.Version != "0.1.3" {
		t.Fatalf("check: %+v %v", info, err)
	}
	res, err := u.applyExtra(context.Background(), game, info)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if res.BackupPath != "" || res.UpdatedFiles != 3 {
		t.Fatalf("fresh install result %+v", res)
	}
	if v := InstalledExtraVersion(game, a); v != "0.1.3" {
		t.Fatalf("installed version %s", v)
	}
	if _, err := os.Stat(filepath.Join(game, "Interface", "AddOns", "SomeoneElse", "keep.lua")); err != nil {
		t.Fatal("unrelated addon was touched")
	}
	// Same version again: no update offered, apply refuses.
	if info, _ := u.checkExtra(context.Background(), "forevermaps", "0.1.3"); info.Available {
		t.Fatal("offered a reinstall")
	}

	u2 := newExtraUpdater(t, extraRelease(t, "0.2.0", extraGoodZip(t, "0.2.0")))
	info, err = u2.checkExtra(context.Background(), "forevermaps", "0.1.3")
	if err != nil || !info.Available || info.Install {
		t.Fatalf("update check: %+v %v", info, err)
	}
	res, err = u2.applyExtra(context.Background(), game, info)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if v := InstalledExtraVersion(game, a); v != "0.2.0" {
		t.Fatalf("updated version %s", v)
	}
	old, err := os.ReadFile(filepath.Join(res.BackupPath, "Core.lua"))
	if err != nil || string(old) != "-- core 0.1.3" {
		t.Fatalf("backup missing or wrong: %q %v", old, err)
	}
	if _, err := os.Stat(filepath.Join(game, "RelaxCoachBackups", ".extra-addon-update.lock")); !os.IsNotExist(err) {
		t.Fatal("lock left behind")
	}
}

func TestExtraAddonRejectsBadPackages(t *testing.T) {
	cases := map[string][]byte{
		"outside folder": extraZip(t, map[string]string{
			"ForeverMaps/ForeverMaps.toc": strings.Replace(extraTestTOC, "%s", "0.1.3", 1),
			"ForeverMaps/Core.lua":        "", "ForeverMaps/Data/World.lua": "", "RankUp/evil.lua": ""}),
		"wrong title": extraZip(t, map[string]string{
			"ForeverMaps/ForeverMaps.toc": strings.Replace(strings.Replace(extraTestTOC, "%s", "0.1.3", 1), "Forever Maps", "Other", 1),
			"ForeverMaps/Core.lua":        "", "ForeverMaps/Data/World.lua": ""}),
		"version mismatch": extraGoodZip(t, "0.1.2"),
		"missing loaded file": extraZip(t, map[string]string{
			"ForeverMaps/ForeverMaps.toc": strings.Replace(extraTestTOC, "%s", "0.1.3", 1),
			"ForeverMaps/Core.lua":        ""}),
		"traversal": extraZip(t, map[string]string{"ForeverMaps/../x.lua": ""}),
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			game := extraGame(t)
			u := newExtraUpdater(t, extraRelease(t, "0.1.3", payload))
			info, err := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := u.applyExtra(context.Background(), game, info); err == nil {
				t.Fatal("bad package was installed")
			}
			if _, err := os.Stat(filepath.Join(game, "Interface", "AddOns", "ForeverMaps")); !os.IsNotExist(err) {
				t.Fatal("addon folder created from a bad package")
			}
		})
	}
}

func TestExtraAddonChecksumAndTrust(t *testing.T) {
	game := extraGame(t)
	s := extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3"))
	s.sums = strings.Repeat("0", 64) + "  ForeverMaps-0.1.3.zip\n"
	u := newExtraUpdater(t, s)
	info, _ := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
	if _, err := u.applyExtra(context.Background(), game, info); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum mismatch accepted: %v", err)
	}
	// A caller cannot swap in another repository's URL.
	info.AssetURL = strings.Replace(info.AssetURL, "buiden/ForeverMaps", "evil/ForeverMaps", 1)
	if _, err := u.applyExtra(context.Background(), game, info); err == nil {
		t.Fatal("untrusted asset URL accepted")
	}
	if _, err := u.checkExtra(context.Background(), "../coach", "0.0.0"); err == nil {
		t.Fatal("unknown addon id accepted")
	}
	// Missing checksum file: not offered.
	s2 := extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3"))
	s2.assets = s2.assets[:1]
	if info, _ := newExtraUpdater(t, s2).checkExtra(context.Background(), "forevermaps", "0.0.0"); info.Available {
		t.Fatal("offered a release without SHA256SUMS.txt")
	}
}

func TestExtraAddonRefusesWhileGameRunningAndForeignFolder(t *testing.T) {
	game := extraGame(t)
	u := newExtraUpdater(t, extraRelease(t, "0.1.3", extraGoodZip(t, "0.1.3")))
	info, _ := u.checkExtra(context.Background(), "forevermaps", "0.0.0")
	u.running = func(context.Context) (bool, error) { return true, nil }
	if _, err := u.applyExtra(context.Background(), game, info); err == nil {
		t.Fatal("installed while WoW was running")
	}
	u.running = func(context.Context) (bool, error) { return false, nil }
	foreign := filepath.Join(game, "Interface", "AddOns", "ForeverMaps")
	os.MkdirAll(foreign, 0755)
	os.WriteFile(filepath.Join(foreign, "ForeverMaps.toc"), []byte("## Title: Something Else\n## Version: 0.0.1\n"), 0644)
	if _, err := u.applyExtra(context.Background(), game, info); err == nil {
		t.Fatal("replaced a folder that is not Forever Maps")
	}
	if b, _ := os.ReadFile(filepath.Join(foreign, "ForeverMaps.toc")); !strings.Contains(string(b), "Something Else") {
		t.Fatal("foreign folder was modified")
	}
}

func TestExtraAddonBridge(t *testing.T) {
	game := extraGame(t)
	a := &App{}
	a.setGame(game)
	var applied ExtraAddonInfo
	b := newNativeBridge(a, false, nativeServices{
		checkExtra: func(_ context.Context, id, current string) (ExtraAddonInfo, error) {
			return ExtraAddonInfo{ID: id, Version: "1.0.1", InstalledVersion: current, Available: true, Install: current == "0.0.0",
				AssetURL: "https://github.com/buiden/RankUp/releases/download/v1.0.1/RankUp-1.0.1.zip"}, nil
		},
		applyExtra: func(_ context.Context, g string, info ExtraAddonInfo) (AddonUpdateResult, error) {
			applied = info
			return AddonUpdateResult{Version: info.Version}, nil
		},
	})
	defer b.Close()
	raw := func(s string) []json.RawMessage { j, _ := json.Marshal(s); return []json.RawMessage{j} }
	list, err := b.Call("extraList", nil)
	if err != nil || len(list.([]nativeExtraOffer)) != 2 {
		t.Fatalf("list %v %v", list, err)
	}
	if _, err := b.Call("applyExtra", raw("rankup@1.0.1")); err == nil {
		t.Fatal("applied without a check")
	}
	offer, err := b.Call("checkExtra", raw("rankup"))
	if err != nil || !offer.(nativeExtraOffer).Available || !offer.(nativeExtraOffer).Install {
		t.Fatalf("check %+v %v", offer, err)
	}
	if j, _ := json.Marshal(offer); strings.Contains(string(j), "releases/download") {
		t.Fatal("download URL leaked to the UI")
	}
	if _, err := b.Call("applyExtra", raw("rankup@9.9.9")); err == nil {
		t.Fatal("applied a version the user did not see")
	}
	b.Call("checkExtra", raw("rankup"))
	if _, err := b.Call("applyExtra", raw("rankup@1.0.1")); err != nil || applied.Version != "1.0.1" {
		t.Fatalf("apply %v %+v", err, applied)
	}
	if _, err := b.Call("checkExtra", raw("journey")); err == nil {
		t.Fatal("unknown addon accepted")
	}
}
