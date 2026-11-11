//go:build !windows

package main

import (
	"errors"
	"syscall"
)

// Test platforms: a busy file or folder stands in for Windows' in-use errors.
func addonErrnoInUse(err error, _ bool) bool {
	return errors.Is(err, syscall.EBUSY) || errors.Is(err, syscall.ETXTBSY)
}
