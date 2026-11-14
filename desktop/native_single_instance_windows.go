//go:build windows

package main

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// nativeInstanceLock stays open for the life of the process; Windows closes it
// on exit, which frees the name for the next copy.
var nativeInstanceLock uintptr

// nativeAcquireInstance owns the session-wide name Local\RelaxCoach.Window when
// no other copy holds it, retrying until wait has passed. Only the name's
// existence matters (no mutex ownership, which is per thread).
func nativeAcquireInstance(wait time.Duration) (bool, error) {
	name, err := windows.UTF16PtrFromString(`Local\RelaxCoach.Window`)
	if err != nil {
		return false, err
	}
	create := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreateMutexW")
	deadline := time.Now().Add(wait)
	for {
		h, _, lastErr := create.Call(0, 0, uintptr(unsafe.Pointer(name)))
		if h == 0 {
			return false, lastErr
		}
		if lastErr != windows.ERROR_ALREADY_EXISTS {
			nativeInstanceLock = h
			return true, nil
		}
		windows.CloseHandle(windows.Handle(h))
		if !time.Now().Before(deadline) {
			return false, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// nativeFocusOther restores and brings forward the open Relax Coach window.
func nativeFocusOther() {
	class, _ := windows.UTF16PtrFromString("RelaxCoachNativeWindow")
	hwnd, _, _ := nativeUser32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(class)), 0)
	if hwnd == 0 {
		return
	}
	if iconic, _, _ := nativeUser32.NewProc("IsIconic").Call(hwnd); iconic != 0 {
		nativeUser32.NewProc("ShowWindow").Call(hwnd, 9) // SW_RESTORE
	}
	nativeUser32.NewProc("SetForegroundWindow").Call(hwnd)
}
