//go:build linux

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// appMoveNew moves from to to in one step and never replaces an existing to
// (rename with RENAME_NOREPLACE), like MoveFileEx without REPLACE_EXISTING on
// Windows. The file never has two names, so a crash cannot leave a hard-linked
// executable that the updater would then refuse.
func appMoveNew(from, to string) error {
	if err := unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE); err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}
