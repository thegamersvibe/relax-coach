package main

import (
	"os"
	"strings"
	"testing"
)

func TestRaidPageNativePickerSpec(t *testing.T) {
	for _, kind := range []string{"log", "officer"} {
		title, flags := nativePickerSpec(kind)
		if flags&0x4000 == 0 || flags&0x40 == 0 || flags&0x1 != 0 || !strings.Contains(strings.ToLower(title), kind) {
			t.Fatal("file picker hides files or has wrong purpose")
		}
	}
	title, flags := nativePickerSpec("folder")
	if flags != 0x41 || !strings.Contains(title, "game folder") {
		t.Fatal("folder picker changed")
	}
	source, err := os.ReadFile("native_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"chooseFile: w.chooseFile", "nativePickerSpec(kind)", "w.dispatch(func()"} {
		if !strings.Contains(string(source), want) {
			t.Fatalf("native picker not wired: %s", want)
		}
	}
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	// Retirement has no Raids page; retain the picker contract above, but never
	// prime the historical raid configuration in the running app's startup.
	if strings.Contains(string(main), "raidConfig()") {
		t.Fatal("retired app must not prime raid config at startup")
	}
}
