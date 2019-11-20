// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"errors"
	"fmt"
	"image"
	"runtime"
	"sync"
	"time"
	"unicode"
	"unsafe"

	syscall "golang.org/x/sys/windows"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
)

var winMap = make(map[syscall.Handle]*window)

type rect struct {
	left, top, right, bottom int32
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cnClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type msg struct {
	hwnd     syscall.Handle
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       point
	lPrivate uint32
}

type point struct {
	x, y int32
}

type window struct {
	hwnd        syscall.Handle
	hdc         syscall.Handle
	w           Callbacks
	width       int
	height      int
	stage       system.Stage
	dead        bool
	pointerBtns pointer.Buttons

	mu        sync.Mutex
	animating bool
}

const (
	_CS_HREDRAW = 0x0002
	_CS_VREDRAW = 0x0001
	_CS_OWNDC   = 0x0020

	_CW_USEDEFAULT = -2147483648

	_IDC_ARROW = 32512

	_INFINITE = 0xFFFFFFFF

	_LOGPIXELSX = 88

	_MDT_EFFECTIVE_DPI = 0

	_MONITOR_DEFAULTTOPRIMARY = 1

	_SIZE_MAXIMIZED = 2
	_SIZE_MINIMIZED = 1
	_SIZE_RESTORED  = 0

	_SW_SHOWDEFAULT = 10

	_USER_TIMER_MINIMUM = 0x0000000A

	_VK_CONTROL = 0x11
	_VK_LWIN    = 0x5B
	_VK_MENU    = 0x12
	_VK_RWIN    = 0x5C
	_VK_SHIFT   = 0x10

	_VK_BACK   = 0x08
	_VK_DELETE = 0x2e
	_VK_DOWN   = 0x28
	_VK_END    = 0x23
	_VK_ESCAPE = 0x1b
	_VK_HOME   = 0x24
	_VK_LEFT   = 0x25
	_VK_NEXT   = 0x22
	_VK_PRIOR  = 0x21
	_VK_RIGHT  = 0x27
	_VK_RETURN = 0x0d
	_VK_SPACE  = 0x20
	_VK_TAB    = 0x09
	_VK_UP     = 0x26

	_VK_F1  = 0x70
	_VK_F2  = 0x71
	_VK_F3  = 0x72
	_VK_F4  = 0x73
	_VK_F5  = 0x74
	_VK_F6  = 0x75
	_VK_F7  = 0x76
	_VK_F8  = 0x77
	_VK_F9  = 0x78
	_VK_F10 = 0x79
	_VK_F11 = 0x7A
	_VK_F12 = 0x7B

	_UNICODE_NOCHAR = 65535

	_WM_CANCELMODE  = 0x001F
	_WM_CHAR        = 0x0102
	_WM_CREATE      = 0x0001
	_WM_DPICHANGED  = 0x02E0
	_WM_DESTROY     = 0x0002
	_WM_ERASEBKGND  = 0x0014
	_WM_KEYDOWN     = 0x0100
	_WM_KEYUP       = 0x0101
	_WM_LBUTTONDOWN = 0x0201
	_WM_LBUTTONUP   = 0x0202
	_WM_MBUTTONDOWN = 0x0207
	_WM_MBUTTONUP   = 0x0208
	_WM_MOUSEMOVE   = 0x0200
	_WM_MOUSEWHEEL  = 0x020A
	_WM_PAINT       = 0x000F
	_WM_QUIT        = 0x0012
	_WM_SETFOCUS    = 0x0007
	_WM_KILLFOCUS   = 0x0008
	_WM_SHOWWINDOW  = 0x0018
	_WM_SIZE        = 0x0005
	_WM_SYSKEYDOWN  = 0x0104
	_WM_RBUTTONDOWN = 0x0204
	_WM_RBUTTONUP   = 0x0205
	_WM_TIMER       = 0x0113
	_WM_UNICHAR     = 0x0109
	_WM_USER        = 0x0400

	_WS_CLIPCHILDREN     = 0x00010000
	_WS_CLIPSIBLINGS     = 0x04000000
	_WS_VISIBLE          = 0x10000000
	_WS_OVERLAPPED       = 0x00000000
	_WS_OVERLAPPEDWINDOW = _WS_OVERLAPPED | _WS_CAPTION | _WS_SYSMENU | _WS_THICKFRAME |
		_WS_MINIMIZEBOX | _WS_MAXIMIZEBOX
	_WS_CAPTION     = 0x00C00000
	_WS_SYSMENU     = 0x00080000
	_WS_THICKFRAME  = 0x00040000
	_WS_MINIMIZEBOX = 0x00020000
	_WS_MAXIMIZEBOX = 0x00010000

	_WS_EX_APPWINDOW  = 0x00040000
	_WS_EX_WINDOWEDGE = 0x00000100

	_QS_ALLINPUT = 0x04FF

	_MWMO_WAITALL        = 0x0001
	_MWMO_INPUTAVAILABLE = 0x0004

	_WAIT_OBJECT_0 = 0

	_PM_REMOVE   = 0x0001
	_PM_NOREMOVE = 0x0000
)

const _WM_REDRAW = _WM_USER + 0

var onceMu sync.Mutex
var mainDone = make(chan struct{})

func Main() {
	<-mainDone
}

func NewWindow(window Callbacks, opts *Options) error {
	onceMu.Lock()
	defer onceMu.Unlock()
	if len(winMap) > 0 {
		return errors.New("multiple windows are not supported")
	}
	cerr := make(chan error)
	go func() {
		// Call win32 API from a single OS thread.
		runtime.LockOSThread()
		w, err := createNativeWindow(opts)
		if err != nil {
			cerr <- err
			return
		}
		defer w.destroy()
		cerr <- nil
		winMap[w.hwnd] = w
		defer delete(winMap, w.hwnd)
		w.w = window
		w.w.SetDriver(w)
		defer w.w.Event(system.DestroyEvent{})
		showWindow(w.hwnd, _SW_SHOWDEFAULT)
		setForegroundWindow(w.hwnd)
		setFocus(w.hwnd)
		if err := w.loop(); err != nil {
			panic(err)
		}
		close(mainDone)
	}()
	return <-cerr
}

func createNativeWindow(opts *Options) (*window, error) {
	setProcessDPIAware()
	cfg := configForDC()
	hInst, err := getModuleHandle()
	if err != nil {
		return nil, err
	}
	curs, err := loadCursor(_IDC_ARROW)
	if err != nil {
		return nil, err
	}
	wcls := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		style:         _CS_HREDRAW | _CS_VREDRAW | _CS_OWNDC,
		lpfnWndProc:   syscall.NewCallback(windowProc),
		hInstance:     hInst,
		hCursor:       curs,
		lpszClassName: syscall.StringToUTF16Ptr("GioWindow"),
	}
	cls, err := registerClassEx(&wcls)
	if err != nil {
		return nil, err
	}
	wr := rect{
		right:  int32(cfg.Px(opts.Width)),
		bottom: int32(cfg.Px(opts.Height)),
	}
	dwStyle := uint32(_WS_OVERLAPPEDWINDOW)
	dwExStyle := uint32(_WS_EX_APPWINDOW | _WS_EX_WINDOWEDGE)
	adjustWindowRectEx(&wr, dwStyle, 0, dwExStyle)
	hwnd, err := createWindowEx(dwExStyle,
		cls,
		opts.Title,
		dwStyle|_WS_CLIPSIBLINGS|_WS_CLIPCHILDREN,
		_CW_USEDEFAULT, _CW_USEDEFAULT,
		wr.right-wr.left,
		wr.bottom-wr.top,
		0,
		0,
		hInst,
		0)
	if err != nil {
		return nil, err
	}
	w := &window{
		hwnd: hwnd,
	}
	w.hdc, err = getDC(hwnd)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func windowProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	w := winMap[hwnd]
	switch msg {
	case _WM_UNICHAR:
		if wParam == _UNICODE_NOCHAR {
			// Tell the system that we accept WM_UNICHAR messages.
			return 1
		}
		fallthrough
	case _WM_CHAR:
		if r := rune(wParam); unicode.IsPrint(r) {
			w.w.Event(key.EditEvent{Text: string(r)})
		}
		// The message is processed.
		return 1
	case _WM_DPICHANGED:
		// Let Windows know we're prepared for runtime DPI changes.
		return 1
	case _WM_ERASEBKGND:
		// Avoid flickering between GPU content and background color.
		return 1
	case _WM_KEYDOWN, _WM_SYSKEYDOWN:
		if n, ok := convertKeyCode(wParam); ok {
			cmd := key.Event{Name: n}
			if getKeyState(_VK_LWIN)&0x1000 != 0 || getKeyState(_VK_RWIN)&0x1000 != 0 {
				cmd.Modifiers |= key.ModSuper
			}
			if getKeyState(_VK_MENU)&0x1000 != 0 {
				cmd.Modifiers |= key.ModAlt
			}
			if getKeyState(_VK_CONTROL)&0x1000 != 0 {
				cmd.Modifiers |= key.ModCtrl
			}
			if getKeyState(_VK_SHIFT)&0x1000 != 0 {
				cmd.Modifiers |= key.ModShift
			}
			w.w.Event(cmd)
		}
	case _WM_LBUTTONDOWN:
		w.pointerButton(pointer.ButtonLeft, true, lParam)
	case _WM_LBUTTONUP:
		w.pointerButton(pointer.ButtonLeft, false, lParam)
	case _WM_RBUTTONDOWN:
		w.pointerButton(pointer.ButtonRight, true, lParam)
	case _WM_RBUTTONUP:
		w.pointerButton(pointer.ButtonRight, false, lParam)
	case _WM_MBUTTONDOWN:
		w.pointerButton(pointer.ButtonMiddle, true, lParam)
	case _WM_MBUTTONUP:
		w.pointerButton(pointer.ButtonMiddle, false, lParam)
	case _WM_CANCELMODE:
		w.w.Event(pointer.Event{
			Type: pointer.Cancel,
		})
	case _WM_SETFOCUS:
		w.w.Event(key.FocusEvent{Focus: true})
	case _WM_KILLFOCUS:
		w.w.Event(key.FocusEvent{Focus: false})
	case _WM_MOUSEMOVE:
		x, y := coordsFromlParam(lParam)
		p := f32.Point{X: float32(x), Y: float32(y)}
		w.w.Event(pointer.Event{
			Type:     pointer.Move,
			Source:   pointer.Mouse,
			Position: p,
			Time:     getMessageTime(),
		})
	case _WM_MOUSEWHEEL:
		w.scrollEvent(wParam, lParam)
	case _WM_DESTROY:
		w.dead = true
	case _WM_PAINT:
		w.draw(true)
	case _WM_SIZE:
		switch wParam {
		case _SIZE_MINIMIZED:
			w.setStage(system.StagePaused)
		case _SIZE_MAXIMIZED, _SIZE_RESTORED:
			w.setStage(system.StageRunning)
		}
	}
	return defWindowProc(hwnd, msg, wParam, lParam)
}

func (w *window) pointerButton(btn pointer.Buttons, press bool, lParam uintptr) {
	var typ pointer.Type
	if press {
		typ = pointer.Press
		if w.pointerBtns == 0 {
			setCapture(w.hwnd)
		}
		w.pointerBtns |= btn
	} else {
		typ = pointer.Release
		w.pointerBtns &^= btn
		if w.pointerBtns == 0 {
			releaseCapture()
		}
	}
	x, y := coordsFromlParam(lParam)
	p := f32.Point{X: float32(x), Y: float32(y)}
	w.w.Event(pointer.Event{
		Type:     typ,
		Source:   pointer.Mouse,
		Position: p,
		Buttons:  w.pointerBtns,
		Time:     getMessageTime(),
	})
}

func coordsFromlParam(lParam uintptr) (int, int) {
	x := int(int16(lParam & 0xffff))
	y := int(int16((lParam >> 16) & 0xffff))
	return x, y
}

func (w *window) scrollEvent(wParam, lParam uintptr) {
	x, y := coordsFromlParam(lParam)
	// The WM_MOUSEWHEEL coordinates are in screen coordinates, in contrast
	// to other mouse events.
	np := point{x: int32(x), y: int32(y)}
	screenToClient(w.hwnd, &np)
	p := f32.Point{X: float32(np.x), Y: float32(np.y)}
	dist := float32(int16(wParam >> 16))
	w.w.Event(pointer.Event{
		Type:     pointer.Move,
		Source:   pointer.Mouse,
		Position: p,
		Scroll:   f32.Point{Y: -dist},
		Time:     getMessageTime(),
	})
}

// Adapted from https://blogs.msdn.microsoft.com/oldnewthing/20060126-00/?p=32513/
func (w *window) loop() error {
	msg := new(msg)
	for !w.dead {
		w.mu.Lock()
		anim := w.animating
		w.mu.Unlock()
		if anim && !peekMessage(msg, w.hwnd, 0, 0, _PM_NOREMOVE) {
			w.draw(false)
			continue
		}
		getMessage(msg, w.hwnd, 0, 0)
		if msg.message == _WM_QUIT {
			postQuitMessage(msg.wParam)
			break
		}
		translateMessage(msg)
		dispatchMessage(msg)
	}
	return nil
}

func (w *window) SetAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	w.mu.Unlock()
	if anim {
		w.postRedraw()
	}
}

func (w *window) postRedraw() {
	if err := postMessage(w.hwnd, _WM_REDRAW, 0, 0); err != nil {
		panic(err)
	}
}

func (w *window) setStage(s system.Stage) {
	w.stage = s
	w.w.Event(system.StageEvent{Stage: s})
}

func (w *window) draw(sync bool) {
	var r rect
	getClientRect(w.hwnd, &r)
	w.width = int(r.right - r.left)
	w.height = int(r.bottom - r.top)
	cfg := configForDC()
	cfg.now = time.Now()
	w.w.Event(FrameEvent{
		FrameEvent: system.FrameEvent{
			Size: image.Point{
				X: w.width,
				Y: w.height,
			},
			Config: &cfg,
		},
		Sync: sync,
	})
}

func (w *window) destroy() {
	if w.hdc != 0 {
		releaseDC(w.hdc)
		w.hdc = 0
	}
	if w.hwnd != 0 {
		destroyWindow(w.hwnd)
		w.hwnd = 0
	}
}

func (w *window) ShowTextInput(show bool) {}

func (w *window) HDC() syscall.Handle {
	return w.hdc
}

func (w *window) HWND() (syscall.Handle, int, int) {
	return w.hwnd, w.width, w.height
}

func convertKeyCode(code uintptr) (string, bool) {
	if '0' <= code && code <= '9' || 'A' <= code && code <= 'Z' {
		return string(code), true
	}
	var r string
	switch code {
	case _VK_ESCAPE:
		r = key.NameEscape
	case _VK_LEFT:
		r = key.NameLeftArrow
	case _VK_RIGHT:
		r = key.NameRightArrow
	case _VK_RETURN:
		r = key.NameReturn
	case _VK_UP:
		r = key.NameUpArrow
	case _VK_DOWN:
		r = key.NameDownArrow
	case _VK_HOME:
		r = key.NameHome
	case _VK_END:
		r = key.NameEnd
	case _VK_BACK:
		r = key.NameDeleteBackward
	case _VK_DELETE:
		r = key.NameDeleteForward
	case _VK_PRIOR:
		r = key.NamePageUp
	case _VK_NEXT:
		r = key.NamePageDown
	case _VK_F1:
		r = "F1"
	case _VK_F2:
		r = "F2"
	case _VK_F3:
		r = "F3"
	case _VK_F4:
		r = "F4"
	case _VK_F5:
		r = "F5"
	case _VK_F6:
		r = "F6"
	case _VK_F7:
		r = "F7"
	case _VK_F8:
		r = "F8"
	case _VK_F9:
		r = "F9"
	case _VK_F10:
		r = "F10"
	case _VK_F11:
		r = "F11"
	case _VK_F12:
		r = "F12"
	case _VK_TAB:
		r = key.NameTab
	case _VK_SPACE:
		r = "Space"
	default:
		return "", false
	}
	return r, true
}

func configForDC() config {
	dpi := getSystemDPI()
	const inchPrDp = 1.0 / 96.0
	ppdp := float32(dpi) * inchPrDp
	return config{
		pxPerDp: ppdp,
		pxPerSp: ppdp,
	}
}

var (
	kernel32          = syscall.NewLazySystemDLL("kernel32.dll")
	_GetModuleHandleW = kernel32.NewProc("GetModuleHandleW")

	user32                       = syscall.NewLazySystemDLL("user32.dll")
	_AdjustWindowRectEx          = user32.NewProc("AdjustWindowRectEx")
	_CallMsgFilter               = user32.NewProc("CallMsgFilterW")
	_CreateWindowEx              = user32.NewProc("CreateWindowExW")
	_DefWindowProc               = user32.NewProc("DefWindowProcW")
	_DestroyWindow               = user32.NewProc("DestroyWindow")
	_DispatchMessage             = user32.NewProc("DispatchMessageW")
	_GetClientRect               = user32.NewProc("GetClientRect")
	_GetDC                       = user32.NewProc("GetDC")
	_GetKeyState                 = user32.NewProc("GetKeyState")
	_GetMessage                  = user32.NewProc("GetMessageW")
	_GetMessageTime              = user32.NewProc("GetMessageTime")
	_KillTimer                   = user32.NewProc("KillTimer")
	_LoadCursor                  = user32.NewProc("LoadCursorW")
	_MonitorFromPoint            = user32.NewProc("MonitorFromPoint")
	_MsgWaitForMultipleObjectsEx = user32.NewProc("MsgWaitForMultipleObjectsEx")
	_PeekMessage                 = user32.NewProc("PeekMessageW")
	_PostMessage                 = user32.NewProc("PostMessageW")
	_PostQuitMessage             = user32.NewProc("PostQuitMessage")
	_ReleaseCapture              = user32.NewProc("ReleaseCapture")
	_RegisterClassExW            = user32.NewProc("RegisterClassExW")
	_ReleaseDC                   = user32.NewProc("ReleaseDC")
	_ScreenToClient              = user32.NewProc("ScreenToClient")
	_ShowWindow                  = user32.NewProc("ShowWindow")
	_SetCapture                  = user32.NewProc("SetCapture")
	_SetForegroundWindow         = user32.NewProc("SetForegroundWindow")
	_SetFocus                    = user32.NewProc("SetFocus")
	_SetProcessDPIAware          = user32.NewProc("SetProcessDPIAware")
	_SetTimer                    = user32.NewProc("SetTimer")
	_TranslateMessage            = user32.NewProc("TranslateMessage")
	_UnregisterClass             = user32.NewProc("UnregisterClassW")
	_UpdateWindow                = user32.NewProc("UpdateWindow")

	shcore            = syscall.NewLazySystemDLL("shcore")
	_GetDpiForMonitor = shcore.NewProc("GetDpiForMonitor")

	gdi32          = syscall.NewLazySystemDLL("gdi32")
	_GetDeviceCaps = gdi32.NewProc("GetDeviceCaps")
)

func getModuleHandle() (syscall.Handle, error) {
	h, _, err := _GetModuleHandleW.Call(uintptr(0))
	if h == 0 {
		return 0, fmt.Errorf("GetModuleHandleW failed: %v", err)
	}
	return syscall.Handle(h), nil
}

func adjustWindowRectEx(r *rect, dwStyle uint32, bMenu int, dwExStyle uint32) {
	_AdjustWindowRectEx.Call(uintptr(unsafe.Pointer(r)), uintptr(dwStyle), uintptr(bMenu), uintptr(dwExStyle))
	issue34474KeepAlive(r)
}

func callMsgFilter(m *msg, nCode uintptr) bool {
	r, _, _ := _CallMsgFilter.Call(uintptr(unsafe.Pointer(m)), nCode)
	issue34474KeepAlive(m)
	return r != 0
}

func createWindowEx(dwExStyle uint32, lpClassName uint16, lpWindowName string, dwStyle uint32, x, y, w, h int32, hWndParent, hMenu, hInstance syscall.Handle, lpParam uintptr) (syscall.Handle, error) {
	wname := syscall.StringToUTF16Ptr(lpWindowName)
	hwnd, _, err := _CreateWindowEx.Call(
		uintptr(dwExStyle),
		uintptr(lpClassName),
		uintptr(unsafe.Pointer(wname)),
		uintptr(dwStyle),
		uintptr(x), uintptr(y),
		uintptr(w), uintptr(h),
		uintptr(hWndParent),
		uintptr(hMenu),
		uintptr(hInstance),
		uintptr(lpParam))
	issue34474KeepAlive(wname)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowEx failed: %v", err)
	}
	return syscall.Handle(hwnd), nil
}

func defWindowProc(hwnd syscall.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	r, _, _ := _DefWindowProc.Call(uintptr(hwnd), uintptr(msg), wparam, lparam)
	return r
}

func destroyWindow(hwnd syscall.Handle) {
	_DestroyWindow.Call(uintptr(hwnd))
}

func dispatchMessage(m *msg) {
	_DispatchMessage.Call(uintptr(unsafe.Pointer(m)))
	issue34474KeepAlive(m)
}

func getClientRect(hwnd syscall.Handle, r *rect) {
	_GetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(r)))
	issue34474KeepAlive(r)
}

func getDC(hwnd syscall.Handle) (syscall.Handle, error) {
	hdc, _, err := _GetDC.Call(uintptr(hwnd))
	if hdc == 0 {
		return 0, fmt.Errorf("GetDC failed: %v", err)
	}
	return syscall.Handle(hdc), nil
}

func getDeviceCaps(hdc syscall.Handle, index int32) int {
	c, _, _ := _GetDeviceCaps.Call(uintptr(hdc), uintptr(index))
	return int(c)
}

func getDpiForMonitor(hmonitor syscall.Handle, dpiType uint32) int {
	var dpiX, dpiY uintptr
	_GetDpiForMonitor.Call(uintptr(hmonitor), uintptr(dpiType), uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	return int(dpiX)
}

// getSystemDPI returns the effective DPI of the system.
func getSystemDPI() int {
	// Check for GetDpiForMonitor, introduced in Windows 8.1.
	if _GetDpiForMonitor.Find() == nil {
		hmon := monitorFromPoint(point{}, _MONITOR_DEFAULTTOPRIMARY)
		return getDpiForMonitor(hmon, _MDT_EFFECTIVE_DPI)
	} else {
		// Fall back to the physical device DPI.
		screenDC, err := getDC(0)
		if err != nil {
			return 96
		}
		defer releaseDC(screenDC)
		return getDeviceCaps(screenDC, _LOGPIXELSX)
	}
}

func getKeyState(nVirtKey int32) int16 {
	c, _, _ := _GetKeyState.Call(uintptr(nVirtKey))
	return int16(c)
}

func getMessage(m *msg, hwnd syscall.Handle, wMsgFilterMin, wMsgFilterMax uint32) int32 {
	r, _, _ := _GetMessage.Call(uintptr(unsafe.Pointer(m)),
		uintptr(hwnd),
		uintptr(wMsgFilterMin),
		uintptr(wMsgFilterMax))
	issue34474KeepAlive(m)
	return int32(r)
}

func getMessageTime() time.Duration {
	r, _, _ := _GetMessageTime.Call()
	return time.Duration(r) * time.Millisecond
}

func killTimer(hwnd syscall.Handle, nIDEvent uintptr) error {
	r, _, err := _SetTimer.Call(uintptr(hwnd), uintptr(nIDEvent), 0, 0)
	if r == 0 {
		return fmt.Errorf("KillTimer failed: %v", err)
	}
	return nil
}

func loadCursor(curID uint16) (syscall.Handle, error) {
	h, _, err := _LoadCursor.Call(0, uintptr(curID))
	if h == 0 {
		return 0, fmt.Errorf("LoadCursorW failed: %v", err)
	}
	return syscall.Handle(h), nil
}

func monitorFromPoint(pt point, flags uint32) syscall.Handle {
	r, _, _ := _MonitorFromPoint.Call(uintptr(pt.x), uintptr(pt.y), uintptr(flags))
	return syscall.Handle(r)
}

func msgWaitForMultipleObjectsEx(nCount uint32, pHandles uintptr, millis, mask, flags uint32) (uint32, error) {
	r, _, err := _MsgWaitForMultipleObjectsEx.Call(uintptr(nCount), pHandles, uintptr(millis), uintptr(mask), uintptr(flags))
	res := uint32(r)
	if res == 0xFFFFFFFF {
		return 0, fmt.Errorf("MsgWaitForMultipleObjectsEx failed: %v", err)
	}
	return res, nil
}

func peekMessage(m *msg, hwnd syscall.Handle, wMsgFilterMin, wMsgFilterMax, wRemoveMsg uint32) bool {
	r, _, _ := _PeekMessage.Call(uintptr(unsafe.Pointer(m)), uintptr(hwnd), uintptr(wMsgFilterMin), uintptr(wMsgFilterMax), uintptr(wRemoveMsg))
	issue34474KeepAlive(m)
	return r != 0
}

func postQuitMessage(exitCode uintptr) {
	_PostQuitMessage.Call(exitCode)
}

func postMessage(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) error {
	r, _, err := _PostMessage.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	if r == 0 {
		return fmt.Errorf("PostMessage failed: %v", err)
	}
	return nil
}

func releaseCapture() bool {
	r, _, _ := _ReleaseCapture.Call()
	return r != 0
}

func registerClassEx(cls *wndClassEx) (uint16, error) {
	a, _, err := _RegisterClassExW.Call(uintptr(unsafe.Pointer(cls)))
	issue34474KeepAlive(cls)
	if a == 0 {
		return 0, fmt.Errorf("RegisterClassExW failed: %v", err)
	}
	return uint16(a), nil
}

func releaseDC(hdc syscall.Handle) {
	_ReleaseDC.Call(uintptr(hdc))
}

func setForegroundWindow(hwnd syscall.Handle) {
	_SetForegroundWindow.Call(uintptr(hwnd))
}

func setFocus(hwnd syscall.Handle) {
	_SetFocus.Call(uintptr(hwnd))
}

func setProcessDPIAware() {
	_SetProcessDPIAware.Call()
}

func setCapture(hwnd syscall.Handle) syscall.Handle {
	r, _, _ := _SetCapture.Call(uintptr(hwnd))
	return syscall.Handle(r)
}

func setTimer(hwnd syscall.Handle, nIDEvent uintptr, uElapse uint32, timerProc uintptr) error {
	r, _, err := _SetTimer.Call(uintptr(hwnd), uintptr(nIDEvent), uintptr(uElapse), timerProc)
	if r == 0 {
		return fmt.Errorf("SetTimer failed: %v", err)
	}
	return nil
}

func screenToClient(hwnd syscall.Handle, p *point) {
	_ScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(p)))
	issue34474KeepAlive(p)
}

func showWindow(hwnd syscall.Handle, nCmdShow int32) {
	_ShowWindow.Call(uintptr(hwnd), uintptr(nCmdShow))
}

func translateMessage(m *msg) {
	_TranslateMessage.Call(uintptr(unsafe.Pointer(m)))
	issue34474KeepAlive(m)
}

func unregisterClass(cls uint16, hInst syscall.Handle) {
	_UnregisterClass.Call(uintptr(cls), uintptr(hInst))
}

func updateWindow(hwnd syscall.Handle) {
	_UpdateWindow.Call(uintptr(hwnd))
}

// issue34474KeepAlive calls runtime.KeepAlive as a
// workaround for golang.org/issue/34474.
func issue34474KeepAlive(v interface{}) {
	runtime.KeepAlive(v)
}
