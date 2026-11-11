//go:build windows

package main

import (
	"errors"
	"syscall"
)

// Windows refuses to move a folder while a file in it is open: a sharing or
// lock violation, or "access denied" for a directory rename. "Access denied"
// is also a permission problem, so it only counts as in use while WoW runs.
func addonErrnoInUse(err error, running bool) bool {
	const sharingViolation, lockViolation, accessDenied = syscall.Errno(32), syscall.Errno(33), syscall.Errno(5)
	return errors.Is(err, sharingViolation) || errors.Is(err, lockViolation) || (running && errors.Is(err, accessDenied))
}
