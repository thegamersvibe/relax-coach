package main

// Window geometry for the native shell, kept free of Windows calls so it can be
// tested on every platform. All sizes are physical pixels: the manifest is per-monitor
// DPI aware, so the design size is scaled by the DPI (96 = 100%).

const (
	nativeDesignW, nativeDesignH = 1280, 900 // the layout the shell is designed for
	nativeMinW, nativeMinH       = 760, 560  // below this the layout breaks
)

// nativeDPI returns the DPI to scale by; anything outside 96..480 counts as 100%.
func nativeDPI(dpi int) int {
	if dpi < 96 || dpi > 480 {
		return 96
	}
	return dpi
}

// nativeWindowSize is the design size scaled by dpi, each side clamped to the work
// area (the screen minus the taskbar), so no part of the window starts off screen.
// An unknown work area (zero or negative) leaves the scaled size unclamped.
func nativeWindowSize(dpi, workW, workH int) (w, h int) {
	dpi = nativeDPI(dpi)
	return nativeClamp(nativeDesignW*dpi/96, workW), nativeClamp(nativeDesignH*dpi/96, workH)
}

// nativeWindowMinSize is the smallest size a player can drag the window to: the
// minimum layout scaled by dpi, but never larger than the work area.
func nativeWindowMinSize(dpi, workW, workH int) (w, h int) {
	dpi = nativeDPI(dpi)
	return nativeClamp(nativeMinW*dpi/96, workW), nativeClamp(nativeMinH*dpi/96, workH)
}

// nativeWindowPos centres a w x h window in the work area whose top-left corner is
// (left, top).
func nativeWindowPos(left, top, workW, workH, w, h int) (x, y int) {
	if workW <= 0 || workH <= 0 {
		return left, top
	}
	return left + (workW-w)/2, top + (workH-h)/2
}

func nativeClamp(v, limit int) int {
	if limit > 0 && v > limit {
		return limit
	}
	return v
}
