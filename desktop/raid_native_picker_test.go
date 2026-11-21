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
	load := strings.Index(string(main), "_, _ = raidConfig()")
	app := strings.Index(string(main), "app := &App{}")
	if load < 0 || load >= app {
		t.Fatal("embedded config not primed at startup")
	}
}
