package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"os"
	"testing"
)

// Executed explicitly by the real app-only release gate, never replaced by a
// synthetic PE. validateAppPE is byte-identical to the shipped updater.
func TestRetirementReleaseExecutable(t *testing.T) {
	path := os.Getenv("RETIREMENT_EXE")
	if path == "" {
		t.Skip("real release gate supplies RETIREMENT_EXE")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = validateAppPE(data); err != nil {
		t.Fatal(err)
	}
	image, err := pe.NewFile(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer image.Close()
	resource := image.Section(".rsrc")
	if resource == nil {
		t.Fatal("missing PE resource section")
	}
	versionData, err := resource.Data()
	if err != nil {
		t.Fatal(err)
	}
	signature := []byte{0xbd, 0x04, 0xef, 0xfe, 0x00, 0x00, 0x01, 0x00}
	i := bytes.Index(versionData, signature)
	if i < 0 || i+24 > len(versionData) {
		t.Fatal("missing Windows version resource")
	}
	for _, offset := range []int{8, 16} {
		if binary.LittleEndian.Uint32(versionData[i+offset:]) != uint32(1<<16|9) || binary.LittleEndian.Uint32(versionData[i+offset+4:]) != 0 {
			t.Fatal("Windows version resource is not 1.9.0.0")
		}
	}
	t.Logf("unchanged updater accepted retirement executable sha256=%s", appDigest(data))
}
