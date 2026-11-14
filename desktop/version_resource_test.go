package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"unicode/utf16"
)

// winres/winres.json is the single source for the exe's Windows resources (icon, manifest,
// version). resource_windows_amd64.syso is generated from it with go-winres; see CONTRIBUTING.md.
func readWinres(t *testing.T) (meta struct {
	Manifest map[string]map[string]struct {
		ExecutionLevel string `json:"execution-level"`
		UIAccess       bool   `json:"ui-access"`
		AutoElevate    bool   `json:"auto-elevate"`
		DPIAwareness   string `json:"dpi-awareness"`
		MinimumOS      string `json:"minimum-os"`
	} `json:"RT_MANIFEST"`
	Icon    map[string]map[string]string `json:"RT_GROUP_ICON"`
	Version map[string]map[string]struct {
		Fixed struct {
			FileVersion    string `json:"file_version"`
			ProductVersion string `json:"product_version"`
		} `json:"fixed"`
		Info map[string]map[string]string `json:"info"`
	} `json:"RT_VERSION"`
}) {
	b, err := os.ReadFile("winres/winres.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	return meta
}

func TestAppResourceVersionMatches160(t *testing.T) {
	meta := readWinres(t)
	v := meta.Version["#1"]["0000"]
	info := v.Info["0409"]
	for _, s := range []string{Version, info["FileVersion"], info["ProductVersion"]} {
		if s != "1.6.0" {
			t.Errorf("stale app/resource version %q", s)
		}
	}
	for _, s := range []string{v.Fixed.FileVersion, v.Fixed.ProductVersion} {
		if s != "1.6.0.0" {
			t.Errorf("stale fixed version %q", s)
		}
	}
	for k, want := range map[string]string{"CompanyName": "TheGamersVibe", "ProductName": "Relax Coach", "OriginalFilename": "RelaxCoach.exe"} {
		if info[k] != want {
			t.Errorf("version info %s = %q, want %q", k, info[k], want)
		}
	}
	b, err := os.ReadFile("resource_windows_amd64.syso")
	if err != nil {
		t.Fatal(err)
	}
	signature := []byte{0xbd, 0x04, 0xef, 0xfe}
	at := bytes.Index(b, signature)
	if at < 0 || len(b) < at+24 {
		t.Fatal("compiled resource has no VS_FIXEDFILEINFO")
	}
	for _, offset := range []int{8, 16} {
		if binary.LittleEndian.Uint32(b[at+offset:]) != 0x00010006 || binary.LittleEndian.Uint32(b[at+offset+4:]) != 0x00000000 {
			t.Error("compiled Windows version resource is stale")
		}
	}
	var encoded bytes.Buffer
	for _, c := range utf16.Encode([]rune("1.6.0")) {
		binary.Write(&encoded, binary.LittleEndian, c)
	}
	if !bytes.Contains(b, encoded.Bytes()) {
		t.Error("compiled Windows string version is stale")
	}
}

// An exe with no manifest makes Windows guess (installer detection, virtualization, legacy DPI)
// and is a common antivirus heuristic signal. Keep an explicit asInvoker, never-elevating manifest.
func TestAppResourceDeclaresSafeManifest(t *testing.T) {
	meta := readWinres(t)
	m, ok := meta.Manifest["#1"]["0409"]
	if !ok {
		t.Fatal("winres.json has no RT_MANIFEST #1")
	}
	if m.ExecutionLevel != "as invoker" || m.UIAccess || m.AutoElevate {
		t.Errorf("manifest must run as invoker without uiAccess/autoElevate: %+v", m)
	}
	if m.DPIAwareness != "per monitor v2" || m.MinimumOS != "win10" {
		t.Errorf("manifest dpi/os = %q/%q", m.DPIAwareness, m.MinimumOS)
	}
	if meta.Icon["#1"]["0000"] != "../icon.ico" {
		t.Error("RT_GROUP_ICON #1 must be icon.ico (native window loads icon ID 1)")
	}
	b, err := os.ReadFile("resource_windows_amd64.syso")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`requestedExecutionLevel level="asInvoker" uiAccess="false"`, "permonitorv2", `name="TheGamersVibe.RelaxCoach" version="1.6.0.0"`, "{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"} {
		if !strings.Contains(s, want) {
			t.Errorf("compiled manifest is stale: missing %s (regenerate with go-winres)", want)
		}
	}
	if strings.Contains(s, "requireAdministrator") || strings.Contains(s, "highestAvailable") {
		t.Error("compiled manifest must never request elevation")
	}
}
