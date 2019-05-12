// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"time"
	"unicode"
	"unsafe"

	syscall "golang.org/x/sys/windows"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
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
	hwnd   syscall.Handle
	hdc    syscall.Handle
	w      *Window
	width  int
	height int
	stage  Stage

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

	_SIZE_MAXIMIZED = 2
	_SIZE_MINIMIZED = 1
	_SIZE_RESTORED  = 0

	_SW_SHOWDEFAULT = 10

	_USER_TIMER_MINIMUM = 0x0000000A

	_VK_CONTROL = 0x11

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
	_VK_UP     = 0x26

	_UNICODE_NOCHAR = 65535

	_WM_CANCELMODE  = 0x001F
	_WM_CHAR        = 0x0102
	_WM_CREATE      = 0x0001
	_WM_DESTROY     = 0x0002
	_WM_KEYDOWN     = 0x0100
	_WM_KEYUP       = 0x0101
	_WM_LBUTTONDOWN = 0x0201
	_WM_LBUTTONUP   = 0x0202
	_WM_MOUSEMOVE   = 0x0200
	_WM_MOUSEWHEEL  = 0x020A
	_WM_PAINT       = 0x000F
	_WM_QUIT        = 0x0012
	_WM_SHOWWINDOW  = 0x0018
	_WM_SIZE        = 0x0005
	_WM_SYSKEYDOWN  = 0x0104
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

	_PM_REMOVE = 0x0001
)

const _WM_REDRAW = _WM_USER + 0

var onceMu sync.Mutex
var mainDone = make(chan struct{})

func Main() {
	<-mainDone
}

func createWindow(opts *WindowOptions) error {
	onceMu.Lock()
	defer onceMu.Unlock()
	if len(winMap) > 0 {
		panic("multiple windows are not supported")
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
		windows <- w.w
		showWindow(w.hwnd, _SW_SHOWDEFAULT)
		setForegroundWindow(w.hwnd)
		setFocus(w.hwnd)
		if err := w.loop(); err != nil {
			panic(err)
		}
		close(windows)
		close(mainDone)
	}()
	return <-cerr
}

func createNativeWindow(opts *WindowOptions) (*window, error) {
	setProcessDPIAware()
	screenDC, err := getDC(0)
	if err != nil {
		return nil, err
	}
	cfg := configForDC(screenDC)
	releaseDC(screenDC)
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
	defer unregisterClass(cls, hInst)
	wr := rect{
		right:  int32(cfg.Pixels(opts.Width) + .5),
		bottom: int32(cfg.Pixels(opts.Height) + .5),
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
		hwnd:  hwnd,
		stage: StagePaused,
	}
	winMap[hwnd] = w
	w.hdc, err = getDC(hwnd)
	if err != nil {
		return nil, err
	}
	w.w = newWindow(w)
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
			w.w.event(key.Edit{Text: string(r)})
		}
		// The message is processed.
		return 1
	case _WM_KEYDOWN, _WM_SYSKEYDOWN:
		if n, ok := convertKeyCode(wParam); ok {
			cmd := key.Chord{Name: n}
			if getKeyState(_VK_CONTROL)&0x1000 != 0 {
				cmd.Modifiers |= key.ModCommand
			}
			w.w.event(cmd)
		}
	case _WM_LBUTTONDOWN:
		setCapture(w.hwnd)
		x, y := coordsFromlParam(lParam)
		p := f32.Point{X: float32(x), Y: float32(y)}
		w.w.event(pointer.Event{
			Type:     pointer.Press,
			Source:   pointer.Mouse,
			Position: p,
			Time:     getMessageTime(),
		})
	case _WM_CANCELMODE:
		w.w.event(pointer.Event{
			Type: pointer.Cancel,
		})
	case _WM_LBUTTONUP:
		releaseCapture()
		x, y := coordsFromlParam(lParam)
		p := f32.Point{X: float32(x), Y: float32(y)}
		w.w.event(pointer.Event{
			Type:     pointer.Release,
			Source:   pointer.Mouse,
			Position: p,
			Time:     getMessageTime(),
		})
	case _WM_MOUSEMOVE:
		x, y := coordsFromlParam(lParam)
		p := f32.Point{X: float32(x), Y: float32(y)}
		w.w.event(pointer.Event{
			Type:     pointer.Move,
			Source:   pointer.Mouse,
			Position: p,
			Time:     getMessageTime(),
		})
	case _WM_MOUSEWHEEL:
		w.scrollEvent(wParam, lParam)
	case _WM_DESTROY:
		delete(winMap, hwnd)
		w.setStage(StageDead)
	case _WM_REDRAW:
		w.mu.Lock()
		anim := w.animating
		w.mu.Unlock()
		if anim {
			w.draw(false)
			w.postRedraw()
		}
	case _WM_PAINT:
		w.draw(true)
	case _WM_SIZE:
		switch wParam {
		case _SIZE_MINIMIZED:
			w.setStage(StagePaused)
		case _SIZE_MAXIMIZED, _SIZE_RESTORED:
			w.setStage(StageRunning)
			w.draw(true)
		}
	}
	return defWindowProc(hwnd, msg, wParam, lParam)
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
	w.w.event(pointer.Event{
		Type:     pointer.Move,
		Source:   pointer.Mouse,
		Position: p,
		Scroll:   f32.Point{Y: -dist},
		Time:     getMessageTime(),
	})
}

// Adapted from https://blogs.msdn.microsoft.com/oldnewthing/20060126-00/?p=32513/
func (w *window) loop() error {
loop:
	for w.stage > StageDead {
		var msg msg
		// Since posted messages are always returned before system messages,
		// but we want our WM_REDRAW to always come last, just like WM_PAINT.
		// So peek for system messages first, and fall back to processing
		// all messages.
		if !peekMessage(&msg, w.hwnd, 0, _WM_REDRAW-1, _PM_REMOVE) {
			getMessage(&msg, w.hwnd, 0, 0)
		}
		// Clear queue of all other redraws.
		if msg.message == _WM_REDRAW {
			for peekMessage(&msg, w.hwnd, _WM_REDRAW, _WM_REDRAW, _PM_REMOVE) {
			}
		}
		if msg.message == _WM_QUIT {
			postQuitMessage(msg.wParam)
			break loop
		}
		translateMessage(&msg)
		dispatchMessage(&msg)
	}
	return nil
}

func (w *window) setAnimating(anim bool) {
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

func (w *window) setStage(s Stage) {
	w.stage = s
	w.w.event(ChangeStage{s})
}

func (w *window) draw(sync bool) {
	var r rect
	getClientRect(w.hwnd, &r)
	w.width = int(r.right - r.left)
	w.height = int(r.bottom - r.top)
	cfg := configForDC(w.hdc)
	cfg.Now = time.Now()
	w.w.event(Draw{
		Size: image.Point{
			X: w.width,
			Y: w.height,
		},
		Config: cfg,
		sync:   sync,
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

func (w *window) setTextInput(s key.TextInputState) {}

func (w *window) display() uintptr {
	return uintptr(w.hdc)
}

func (w *window) nativeWindow(visID int) (uintptr, int, int) {
	return uintptr(w.hwnd), w.width, w.height
}

func convertKeyCode(code uintptr) (rune, bool) {
	if '0' <= code && code <= '9' || 'A' <= code && code <= 'Z' {
		return rune(code), true
	}
	var r rune
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
	default:
		return 0, false
	}
	return r, true
}

func configForDC(hdc syscall.Handle) *ui.Config {
	dpi := getDeviceCaps(hdc, _LOGPIXELSX)
	ppdp := float32(dpi) * inchPrDp * monitorScale
	// Force a minimum density to keep text legible and to handle bogus output geometry.
	if ppdp < minDensity {
		ppdp = minDensity
	}
	return &ui.Config{
		PxPerDp: ppdp,
		PxPerSp: ppdp,
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
}

func callMsgFilter(m *msg, nCode uintptr) bool {
	r, _, _ := _CallMsgFilter.Call(uintptr(unsafe.Pointer(m)), nCode)
	return r != 0
}

func createWindowEx(dwExStyle uint32, lpClassName uint16, lpWindowName string, dwStyle uint32, x, y, w, h int32, hWndParent, hMenu, hInstance syscall.Handle, lpParam uintptr) (syscall.Handle, error) {
	hwnd, _, err := _CreateWindowEx.Call(
		uintptr(dwExStyle),
		uintptr(lpClassName),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(lpWindowName))),
		uintptr(dwStyle),
		uintptr(x), uintptr(y),
		uintptr(w), uintptr(h),
		uintptr(hWndParent),
		uintptr(hMenu),
		uintptr(hInstance),
		uintptr(lpParam))
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
}

func getClientRect(hwnd syscall.Handle, r *rect) {
	_GetClientRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(r)))
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

func getKeyState(nVirtKey int32) int16 {
	c, _, _ := _GetKeyState.Call(uintptr(nVirtKey))
	return int16(c)
}

func getMessage(m *msg, hwnd syscall.Handle, wMsgFilterMin, wMsgFilterMax uint32) int32 {
	r, _, _ := _GetMessage.Call(uintptr(unsafe.Pointer(m)),
		uintptr(hwnd),
		uintptr(wMsgFilterMin),
		uintptr(wMsgFilterMax))
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
	r, _, _ := _SetCapture.Call(uintptr(unsafe.Pointer(hwnd)))
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
}

func showWindow(hwnd syscall.Handle, nCmdShow int32) {
	_ShowWindow.Call(uintptr(hwnd), uintptr(nCmdShow))
}

func translateMessage(m *msg) {
	_TranslateMessage.Call(uintptr(unsafe.Pointer(m)))
}

func unregisterClass(cls uint16, hInst syscall.Handle) {
	_UnregisterClass.Call(uintptr(cls), uintptr(hInst))
}

func updateWindow(hwnd syscall.Handle) {
	_UpdateWindow.Call(uintptr(hwnd))
}
