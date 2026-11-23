//go:build windows

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

var retirementMoveFileEx = syscall.NewLazyDLL("kernel32.dll").NewProc("MoveFileExW")

func retirementPlatformMoveNoReplace(oldAbs, newAbs string) error {
	from, err := syscall.UTF16PtrFromString(oldAbs)
	if err != nil {
		return err
	}
	to, err := syscall.UTF16PtrFromString(newAbs)
	if err != nil {
		return err
	}
	// Flags 0: never REPLACE_EXISTING or COPY_ALLOWED. Cross-volume moves fail.
	ok, _, callErr := retirementMoveFileEx.Call(uintptr(unsafe.Pointer(from)), uintptr(unsafe.Pointer(to)), 0)
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	if ok == 0 {
		if callErr == syscall.Errno(0) {
			callErr = syscall.EINVAL
		}
		return &os.LinkError{Op: "rename-noreplace", Old: oldAbs, New: newAbs, Err: callErr}
	}
	return nil
}
