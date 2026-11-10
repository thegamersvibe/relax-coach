//go:build windows

package main

import (
	"errors"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Shell link COM identifiers (ShObjIdl.h / ObjIdl.h).
var (
	nativeShellLinkCLSID   = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	nativeShellLinkWIID    = windows.GUID{Data1: 0x000214f9, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	nativePersistFileIID   = windows.GUID{Data1: 0x0000010b, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	nativeCoCreateInstance = windows.NewLazySystemDLL("ole32.dll").NewProc("CoCreateInstance")
)

// IShellLinkW vtable slots: 7 SetDescription, 9 SetWorkingDirectory,
// 17 SetIconLocation, 20 SetPath. IPersistFile: 6 Save. IUnknown: 0
// QueryInterface, 2 Release.
func nativeWriteShortcut(link, target, workdir string) error {
	var shell *nativeCOM
	hr, _, _ := nativeCoCreateInstance.Call(uintptr(unsafe.Pointer(&nativeShellLinkCLSID)), 0, 1, /* CLSCTX_INPROC_SERVER */
		uintptr(unsafe.Pointer(&nativeShellLinkWIID)), uintptr(unsafe.Pointer(&shell)))
	if int32(hr) < 0 || shell == nil {
		return errors.New("the Windows shortcut service is unavailable")
	}
	defer shell.call(2)
	text := func(s string) (*uint16, error) { return windows.UTF16PtrFromString(s) }
	path, err := text(target)
	if err != nil {
		return err
	}
	dir, err := text(workdir)
	if err != nil {
		return err
	}
	desc, _ := text("Relax Coach - WoW Forever companion")
	file, err := text(link)
	if err != nil {
		return err
	}
	if _, err = shell.call(20, uintptr(unsafe.Pointer(path))); err != nil {
		return err
	}
	if _, err = shell.call(9, uintptr(unsafe.Pointer(dir))); err != nil {
		return err
	}
	shell.call(7, uintptr(unsafe.Pointer(desc)))
	shell.call(17, uintptr(unsafe.Pointer(path)), 0)
	var persist *nativeCOM
	if _, err = shell.call(0, uintptr(unsafe.Pointer(&nativePersistFileIID)), uintptr(unsafe.Pointer(&persist))); err != nil || persist == nil {
		return errors.New("the Windows shortcut service cannot save shortcuts")
	}
	defer persist.call(2)
	if _, err = persist.call(6, uintptr(unsafe.Pointer(file)), 1); err != nil {
		return err
	}
	runtime.KeepAlive(path)
	runtime.KeepAlive(dir)
	runtime.KeepAlive(desc)
	runtime.KeepAlive(file)
	return nil
}

// Shell COM objects run on the window's STA UI thread.
func (w *nativeWindow) createShortcut(link, target, workdir string) error {
	done := make(chan error, 1)
	w.dispatch(func() { done <- nativeWriteShortcut(link, target, workdir) })
	select {
	case err := <-done:
		return err
	case <-w.bridge.ctx.Done():
		return w.bridge.ctx.Err()
	}
}

func (w *nativeWindow) placementEnv() appPlacementEnv {
	exe, err := os.Executable()
	if err != nil {
		return appPlacementEnv{err: err}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return appPlacementEnv{err: err}
	}
	desktop, _ := windows.KnownFolderPath(windows.FOLDERID_Desktop, 0)
	return appPlacementEnv{exe: exe, home: home, desktop: desktop, shortcut: w.createShortcut}
}
