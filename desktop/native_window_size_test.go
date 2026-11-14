package main

import "testing"

func TestNativeWindowSizeFitsTheWorkArea(t *testing.T) {
	for _, c := range []struct {
		name              string
		dpi, workW, workH int
		wantW, wantH      int
	}{
		{"100% on 1080p", 96, 1920, 1040, 1280, 900},
		{"125% on 1080p laptop", 120, 1920, 1020, 1600, 1020},
		{"150% on 1080p laptop", 144, 1920, 1020, 1920, 1020},
		{"100% on 1366x768", 96, 1366, 728, 1280, 728},
		{"125% on 1366x768", 120, 1366, 728, 1366, 728},
		{"100% on 4K", 96, 3840, 2120, 1280, 900},
		{"200% on 4K", 192, 3840, 2120, 2560, 1800},
		{"unknown work area keeps the scaled size", 120, 0, 0, 1600, 1125},
		{"DPI out of range counts as 100%", 0, 1920, 1040, 1280, 900},
		{"DPI above 480 counts as 100%", 960, 1920, 1040, 1280, 900},
	} {
		w, h := nativeWindowSize(c.dpi, c.workW, c.workH)
		if w != c.wantW || h != c.wantH {
			t.Errorf("%s: nativeWindowSize(%d, %d, %d) = %dx%d, want %dx%d", c.name, c.dpi, c.workW, c.workH, w, h, c.wantW, c.wantH)
		}
	}
}

func TestNativeWindowMinSizeScalesAndFits(t *testing.T) {
	for _, c := range []struct {
		dpi, workW, workH, wantW, wantH int
	}{
		{96, 1920, 1040, 760, 560},
		{144, 1920, 1020, 1140, 840},
		{192, 1366, 728, 1366, 728}, // never larger than the screen
		{96, 0, 0, 760, 560},
		{1, 1920, 1040, 760, 560},
	} {
		w, h := nativeWindowMinSize(c.dpi, c.workW, c.workH)
		if w != c.wantW || h != c.wantH {
			t.Errorf("nativeWindowMinSize(%d, %d, %d) = %dx%d, want %dx%d", c.dpi, c.workW, c.workH, w, h, c.wantW, c.wantH)
		}
	}
}

func TestNativeWindowIsCentredInTheWorkArea(t *testing.T) {
	for _, c := range []struct {
		left, top, workW, workH, w, h, wantX, wantY int
	}{
		{0, 0, 1920, 1040, 1280, 900, 320, 70},
		{0, 0, 1920, 1020, 1920, 1020, 0, 0},
		{0, 40, 1366, 728, 1280, 728, 43, 40},        // taskbar at the top
		{-1920, 0, 1920, 1040, 1280, 900, -1600, 70}, // primary to the right of another screen
		{100, 50, 0, 0, 1600, 1125, 100, 50},         // unknown work area: its corner
	} {
		x, y := nativeWindowPos(c.left, c.top, c.workW, c.workH, c.w, c.h)
		if x != c.wantX || y != c.wantY {
			t.Errorf("nativeWindowPos(%d, %d, %d, %d, %d, %d) = %d,%d, want %d,%d", c.left, c.top, c.workW, c.workH, c.w, c.h, x, y, c.wantX, c.wantY)
		}
	}
}
