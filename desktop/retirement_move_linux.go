//go:build linux

package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"
)

func retirementPlatformMoveNoReplace(oldAbs, newAbs string) error {
	var number uintptr
	switch runtime.GOARCH {
	case "amd64":
		number = 316 // SYS_RENAMEAT2
	case "arm64":
		number = 276 // SYS_RENAMEAT2
	default:
		return &os.LinkError{Op: "rename-noreplace", Old: oldAbs, New: newAbs, Err: syscall.ENOSYS}
	}
	from, err := syscall.BytePtrFromString(oldAbs)
	if err != nil {
		return err
	}
	to, err := syscall.BytePtrFromString(newAbs)
	if err != nil {
		return err
	}
	const atFDCWD = ^uintptr(99) // -100; paths are absolute.
	const renameNoReplace = 1
	_, _, errno := syscall.Syscall6(number, atFDCWD, uintptr(unsafe.Pointer(from)), atFDCWD, uintptr(unsafe.Pointer(to)), renameNoReplace, 0)
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	if errno != 0 {
		// Unsupported kernel/filesystem and cross-volume moves fail closed too.
		return &os.LinkError{Op: "rename-noreplace", Old: oldAbs, New: newAbs, Err: errno}
	}
	return nil
}
