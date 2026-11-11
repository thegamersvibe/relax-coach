//go:build windows

package main

// COM slots below are from Microsoft.Web.WebView2 1.0.2365.46 WebView2.h,
// cross-checked against go-webview2 commit 56598839c808a2340edee99204db479f410e9bf4.
// Use the exported edge layer, not the convenience WebView wrapper: the latter
// exposes neither NavigationStarting cancellation nor NewWindowRequested blocking.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/jchv/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

var nativeUser32 = windows.NewLazySystemDLL("user32.dll")
var nativeShell32 = windows.NewLazySystemDLL("shell32.dll")
var nativeActive *nativeWindow // accessed only on the locked UI thread

const (
	nativeWMTask    = 0x8001
	nativeWMClose   = 0x0010
	nativeWMDestroy = 0x0002
	nativeIconID    = 1 // RT_GROUP_ICON 1 in goversioninfo's resource_windows_amd64.syso
)

type nativePoint struct{ X, Y int32 }
type nativeMSG struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          nativePoint
	Private        uint32
}
type nativeWindowClass struct {
	Size, Style                        uint32
	Procedure                          uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}
type nativeCOM struct{ VTable *[128]uintptr }

// Pointer-valued uintptr arguments must stay live and off the movable Go stack.
// SyscallN's directive does not propagate through this variadic wrapper.
//
//go:uintptrescapes
func (c *nativeCOM) call(slot int, args ...uintptr) (uintptr, error) {
	values := append([]uintptr{uintptr(unsafe.Pointer(c))}, args...)
	hr, _, _ := syscall.SyscallN(c.VTable[slot], values...)
	if int32(hr) < 0 {
		return hr, fmt.Errorf("COM method %d failed (HRESULT 0x%08x)", slot, uint32(hr))
	}
	return hr, nil
}
func (c *nativeCOM) text(slot int) (string, error) {
	var p *uint16
	_, err := c.call(slot, uintptr(unsafe.Pointer(&p)))
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", nil
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	return windows.UTF16PtrToString(p), nil
}

type nativeWindow struct {
	hwnd             uintptr
	chromium         *edge.Chromium
	core, controller *nativeCOM
	bridge           *nativeBridge
	token            string
	gate             nativeNavigationGate
	handlers         []*nativeCOMHandler
	mu               sync.Mutex
	queue            []func()
	destroyed        bool
	closing          bool // UI thread only
}

func (w *nativeWindow) dispatch(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.destroyed {
		return
	}
	w.queue = append(w.queue, fn)
	nativeUser32.NewProc("PostMessageW").Call(w.hwnd, nativeWMTask, 0, 0)
}
func (w *nativeWindow) drain() {
	w.mu.Lock()
	q := w.queue
	w.queue = nil
	w.mu.Unlock()
	for _, fn := range q {
		fn()
	}
}

func nativeWndProc(hwnd uintptr, msg uint32, wp, lp uintptr) uintptr {
	w := nativeActive
	if w != nil {
		switch msg {
		case 0x0005: // WM_SIZE (including minimizing): resize, never terminate.
			if w.chromium != nil {
				w.chromium.Resize()
			}
			return 0
		case 0x0003, 0x0216:
			if w.chromium != nil {
				_ = w.chromium.NotifyParentWindowPositionChanged()
			}
		case 0x0007: // WM_SETFOCUS
			if w.chromium != nil {
				w.chromium.Focus()
			}
			return 0
		case nativeWMTask:
			w.drain()
			return 0
		case nativeWMClose:
			if !w.closing {
				w.closing = true
				title, _ := windows.UTF16PtrFromString("Relax Coach — closing safely…")
				nativeUser32.NewProc("SetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(title)))
				if w.bridge != nil {
					go func() { w.bridge.Close(); w.dispatch(w.destroy) }()
				} // During Embed, wait for initialization before destroying its window.
			}
			return 0
		case nativeWMDestroy:
			w.mu.Lock()
			w.destroyed = true
			w.queue = nil
			w.mu.Unlock()
			nativeUser32.NewProc("PostQuitMessage").Call(0)
			return 0
		}
	}
	value, _, _ := nativeUser32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), wp, lp)
	return value
}

func (w *nativeWindow) destroy() {
	if w.controller != nil {
		_, _ = w.controller.call(24)
	} // ICoreWebView2Controller.Close
	if w.core != nil {
		_, _ = w.core.call(2)
		w.core = nil
	} // owned GetCoreWebView2 reference
	nativeUser32.NewProc("DestroyWindow").Call(w.hwnd)
	// Handlers stay pinned until controller.Close has detached all COM callbacks.
	for _, h := range w.handlers {
		h.pin.Unpin()
	}
	w.handlers = nil
}

func runNative(app *App, disabled bool) error {
	// edge's init also initializes COM STA. Keep all WebView and window calls here.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	token, err := nativeToken()
	if err != nil {
		return err
	}
	html, err := renderNativeShell(app.snapshot(), disabled, token)
	if err != nil {
		return err
	}
	data, err := os.UserCacheDir()
	if err != nil {
		return err
	}
	data = filepath.Join(data, "RelaxCoach", "WebView2")
	if err = os.MkdirAll(data, 0700); err != nil {
		return err
	}
	w := &nativeWindow{token: token, chromium: edge.NewChromium(), gate: newNativeNavigationGate(html)}
	nativeActive = w
	defer func() { nativeActive = nil }()
	var instance windows.Handle
	if err = windows.GetModuleHandleEx(0, nil, &instance); err != nil {
		return err
	}
	className, _ := windows.UTF16PtrFromString("RelaxCoachNativeWindow")
	title, _ := windows.UTF16PtrFromString("Relax Coach")
	// Load both sizes from the embedded USER artwork, never the default Go/Edge icon.
	large, _, _ := nativeUser32.NewProc("LoadImageW").Call(uintptr(instance), nativeIconID, 1, 32, 32, 0x8000)
	small, _, _ := nativeUser32.NewProc("LoadImageW").Call(uintptr(instance), nativeIconID, 1, 16, 16, 0x8000)
	if large == 0 || small == 0 {
		return errors.New("Relax Coach artwork icon is missing from the executable resources")
	}
	cursor, _, _ := nativeUser32.NewProc("LoadCursorW").Call(0, 32512)
	wc := nativeWindowClass{Procedure: windows.NewCallback(nativeWndProc), Instance: uintptr(instance), Icon: large, SmallIcon: small, Cursor: cursor, ClassName: className, Background: 6}
	wc.Size = uint32(unsafe.Sizeof(wc))
	atom, _, callErr := nativeUser32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("register app window: %w", callErr)
	}
	hwnd, _, callErr := nativeUser32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(title)), 0x00CF0000, 0x80000000, 0x80000000, 1280, 900, 0, 0, uintptr(instance), 0)
	if hwnd == 0 {
		return fmt.Errorf("create app window: %w", callErr)
	}
	w.hwnd = hwnd
	nativeUser32.NewProc("SendMessageW").Call(hwnd, 0x0080, 1, large) // WM_SETICON / ICON_BIG
	nativeUser32.NewProc("SendMessageW").Call(hwnd, 0x0080, 0, small) // ICON_SMALL
	w.chromium.DataPath = data
	w.chromium.SetGlobalPermission(edge.CoreWebView2PermissionStateDeny)
	w.chromium.MessageCallback = w.message
	w.chromium.AcceleratorKeyCallback = func(key uint) bool { return key == 0x74 || key == 0x7B } // F5/F12
	nativeUser32.NewProc("ShowWindow").Call(hwnd, 5)
	nativeUser32.NewProc("UpdateWindow").Call(hwnd)
	if !w.chromium.Embed(hwnd) || w.chromium.GetController() == nil {
		nativeUser32.NewProc("DestroyWindow").Call(hwnd)
		return errors.New("Microsoft Edge WebView2 Runtime could not start. Install or repair WebView2, then reopen Relax Coach; diagnostic -once remains available")
	}
	w.controller = (*nativeCOM)(unsafe.Pointer(w.chromium.GetController()))
	if w.closing {
		w.destroy()
		return nil
	}
	if _, err = w.controller.call(25, uintptr(unsafe.Pointer(&w.core))); err != nil {
		w.destroy()
		return err
	}
	if err = w.lockDown(); err != nil {
		w.destroy()
		return err
	}
	w.bridge = newNativeBridge(app, disabled, nativeServices{choose: w.chooseFolder, open: w.openExternal, place: w.placementEnv})
	defer w.bridge.Close()
	// The native shell is a local string; no listener, browser process launcher or URL.
	w.chromium.NavigateToString(html)
	w.chromium.Resize()
	w.chromium.Focus()
	var msg nativeMSG
	for {
		result, _, err := nativeUser32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) == -1 {
			w.destroy()
			return fmt.Errorf("window event loop: %w", err)
		}
		if result == 0 {
			break
		}
		nativeUser32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		nativeUser32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
	runtime.KeepAlive(w.chromium)
	return nil
}

func (w *nativeWindow) lockDown() error {
	// Use HRESULT checks directly; some upstream convenience setters check last-error instead.
	var settings *nativeCOM
	if _, err := w.core.call(3, uintptr(unsafe.Pointer(&settings))); err != nil {
		return err
	}
	defer settings.call(2)
	for _, slot := range []int{8, 10, 12, 14, 16} { // dialogs, status bar, devtools, context menus, host objects
		if _, err := settings.call(slot, 0); err != nil {
			return err
		}
	}
	// Fail closed if any native navigation guard cannot be attached.
	for _, cfg := range []struct {
		slot         int
		frame, popup bool
		iid          windows.GUID
	}{
		{7, false, false, windows.GUID{Data1: 0x9adbe429, Data2: 0xf36d, Data3: 0x432b, Data4: [8]byte{0x9d, 0xdc, 0xf8, 0x88, 0x1f, 0xbd, 0x76, 0xe3}}},
		{17, true, false, windows.GUID{Data1: 0x9adbe429, Data2: 0xf36d, Data3: 0x432b, Data4: [8]byte{0x9d, 0xdc, 0xf8, 0x88, 0x1f, 0xbd, 0x76, 0xe3}}},
		{44, false, true, windows.GUID{Data1: 0xd4c185fe, Data2: 0xc81c, Data3: 0x4989, Data4: [8]byte{0x97, 0xaf, 0x2d, 0x3f, 0xa7, 0xab, 0x56, 0x51}}},
	} {
		h := newNativeCOMHandler(w, cfg.frame, cfg.popup, cfg.iid)
		w.handlers = append(w.handlers, h)
		var registration int64
		if _, err := w.core.call(cfg.slot, uintptr(unsafe.Pointer(h)), uintptr(unsafe.Pointer(&registration))); err != nil {
			return err
		}
	}
	return nil
}

func (w *nativeWindow) message(raw string) {
	if w.closing || w.bridge == nil || w.core == nil {
		return
	}
	source, err := w.core.text(4) // GetSource: privileged calls only from the embedded doc.
	if err != nil || source != "about:blank" {
		return
	}
	req, err := decodeNativeRequest(raw, w.token)
	if err != nil {
		return
	}
	go func() {
		value, err := w.bridge.Call(req.Method, req.Params)
		callErr := err
		response := nativeResponse{Kind: "native-response", ID: req.ID, Result: value}
		if err != nil {
			response.Result = nil
			response.Error = err.Error()
		}
		data, err := json.Marshal(response)
		if err != nil {
			return
		}
		w.dispatch(func() {
			if w.closing || w.core == nil {
				return
			}
			text, _ := windows.UTF16PtrFromString(string(data))
			_, postErr := w.core.call(32, uintptr(unsafe.Pointer(text))) // PostWebMessageAsJson
			if nativeAppRestartReady(req.Method, value, callErr, postErr) {
				// This closure runs on the locked UI thread, after response dispatch.
				nativeUser32.NewProc("PostMessageW").Call(w.hwnd, nativeWMClose, 0, 0)
			}
		})
	}()
}

func (w *nativeWindow) openExternal(url string) error {
	// Defence in depth at the OS boundary. ShellExecute receives only a validated
	// HTTPS URL, never command text or parameters. There is no arbitrary shell binding.
	if err := validateExternalURL(url); err != nil {
		return err
	}
	done := make(chan error, 1)
	w.dispatch(func() {
		verb, _ := windows.UTF16PtrFromString("open")
		target, _ := windows.UTF16PtrFromString(url)
		result, _, _ := nativeShell32.NewProc("ShellExecuteW").Call(w.hwnd, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(target)), 0, 0, 1)
		if result <= 32 {
			done <- fmt.Errorf("could not open external link (%d)", result)
		} else {
			done <- nil
		}
	})
	select {
	case err := <-done:
		return err
	case <-w.bridge.ctx.Done():
		return w.bridge.ctx.Err()
	}
}

func (w *nativeWindow) chooseFolder(ctx context.Context) (string, error) {
	type result struct {
		path string
		err  error
	}
	done := make(chan result, 1)
	w.dispatch(func() {
		if ctx.Err() != nil {
			done <- result{err: ctx.Err()}
			return
		}
		// SHBrowseForFolderW runs a native modal picker on the existing STA UI thread.
		title, _ := windows.UTF16PtrFromString("Choose the WoW Forever game folder")
		var display [260]uint16
		type browseInfo struct {
			Owner, Root     uintptr
			Display, Title  *uint16
			Flags           uint32
			Callback, Param uintptr
			Image           int32
		}
		info := browseInfo{Owner: w.hwnd, Display: &display[0], Title: title, Flags: 0x0041} // BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE
		pidl, _, _ := nativeShell32.NewProc("SHBrowseForFolderW").Call(uintptr(unsafe.Pointer(&info)))
		if pidl == 0 {
			done <- result{}
			return
		}
		defer windows.NewLazySystemDLL("ole32.dll").NewProc("CoTaskMemFree").Call(pidl)
		var path [32768]uint16
		ok, _, _ := nativeShell32.NewProc("SHGetPathFromIDListEx").Call(pidl, uintptr(unsafe.Pointer(&path[0])), uintptr(len(path)), 0)
		if ok == 0 {
			done <- result{err: errors.New("folder picker could not read this path; paste it instead")}
			return
		}
		done <- result{path: windows.UTF16ToString(path[:])}
	})
	select {
	case r := <-done:
		return r.path, r.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func showNativeError(err error) {
	text, _ := windows.UTF16PtrFromString(err.Error())
	title, _ := windows.UTF16PtrFromString("Relax Coach")
	nativeUser32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}

// Callback storage is pinned and owned until the controller closes. The Go
// bridge is not exported as a COM host object; only the explicit RPC methods exist.
type nativeCOMHandler struct {
	vtable       *[4]uintptr
	window       *nativeWindow
	iid          windows.GUID
	frame, popup bool
	pin          runtime.Pinner
}

var nativeUnknownIID = windows.GUID{Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
var nativeHandlerVTable = [4]uintptr{
	windows.NewCallback(func(h *nativeCOMHandler, iid *windows.GUID, out *unsafe.Pointer) uintptr {
		if out == nil {
			return 0x80004003
		}
		*out = nil
		if iid != nil && (*iid == nativeUnknownIID || *iid == h.iid) {
			*out = unsafe.Pointer(h)
			return 0
		}
		return 0x80004002
	}),
	windows.NewCallback(func(_ *nativeCOMHandler) uintptr { return 1 }),
	windows.NewCallback(func(_ *nativeCOMHandler) uintptr { return 1 }),
	windows.NewCallback(func(h *nativeCOMHandler, _ *nativeCOM, args *nativeCOM) uintptr {
		if h.popup {
			_, _ = args.call(6, 1)
			return 0
		} // NewWindowRequested.put_Handled
		uri, err := args.text(3)
		if err != nil || !h.window.gate.Allow(uri, h.frame) {
			_, _ = args.call(8, 1)
		} // NavigationStarting.put_Cancel
		return 0
	}),
}

func newNativeCOMHandler(w *nativeWindow, frame, popup bool, iid windows.GUID) *nativeCOMHandler {
	h := &nativeCOMHandler{vtable: &nativeHandlerVTable, window: w, frame: frame, popup: popup, iid: iid}
	h.pin.Pin(h)
	return h
}
