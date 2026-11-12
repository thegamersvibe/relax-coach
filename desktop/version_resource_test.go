package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"testing"
	"unicode/utf16"
)

func TestAppResourceVersionMatches154(t *testing.T) {
	b, err := os.ReadFile("versioninfo.json")
	if err != nil {
		t.Fatal(err)
	}
	var meta struct {
		FixedFileInfo struct {
			FileVersion, ProductVersion struct{ Major, Minor, Patch, Build int }
		}
		StringFileInfo struct{ FileVersion, ProductVersion string }
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{Version, meta.StringFileInfo.FileVersion, meta.StringFileInfo.ProductVersion} {
		if v != "1.5.4" {
			t.Errorf("stale app/resource version %q", v)
		}
	}
	for _, v := range []struct{ Major, Minor, Patch, Build int }{meta.FixedFileInfo.FileVersion, meta.FixedFileInfo.ProductVersion} {
		if v.Major != 1 || v.Minor != 5 || v.Patch != 4 || v.Build != 0 {
			t.Errorf("stale fixed version %+v", v)
		}
	}
	b, err = os.ReadFile("resource_windows_amd64.syso")
	if err != nil {
		t.Fatal(err)
	}
	signature := []byte{0xbd, 0x04, 0xef, 0xfe}
	at := bytes.Index(b, signature)
	if at < 0 || len(b) < at+24 {
		t.Fatal("compiled resource has no VS_FIXEDFILEINFO")
	}
	for _, offset := range []int{8, 16} {
		if binary.LittleEndian.Uint32(b[at+offset:]) != 0x00010005 || binary.LittleEndian.Uint32(b[at+offset+4:]) != 0x00040000 {
			t.Error("compiled Windows version resource is stale")
		}
	}
	var encoded bytes.Buffer
	for _, c := range utf16.Encode([]rune("1.5.4")) {
		binary.Write(&encoded, binary.LittleEndian, c)
	}
	if !bytes.Contains(b, encoded.Bytes()) {
		t.Error("compiled Windows string version is stale")
	}
}
