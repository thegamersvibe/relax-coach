package main

// BROWSEINFOW flags (shlobj_core.h). Keep the existing folder-picker behavior;
// file picks include files and do not restrict selection to directories. Both
// paths run the same native modal picker on the existing STA UI thread.
func nativePickerSpec(kind string) (string, uint32) {
	if kind == "folder" {
		return "Choose the WoW Forever game folder", 0x0041
	}
	title := "Choose a combat log file"
	if kind == "officer" {
		title = "Choose the officer ranking JSON file"
	}
	return title, 0x0040 | 0x0010 | 0x0200 | 0x4000 // new UI, edit box, no new-folder button, include files
}
