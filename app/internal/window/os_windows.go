// SPDX-License-Identifier: Unlicense OR MIT

package window

import (
	"errors"
	"image"
	"runtime"
	"sync"
	"time"
	"unicode"
	"unsafe"

	syscall "golang.org/x/sys/windows"

	"gioui.org/app/internal/windows"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
)

var winMap = make(map[syscall.Handle]*window)

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

const _WM_REDRAW = windows.WM_USER + 0

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
		windows.ShowWindow(w.hwnd, windows.SW_SHOWDEFAULT)
		windows.SetForegroundWindow(w.hwnd)
		windows.SetFocus(w.hwnd)
		if err := w.loop(); err != nil {
			panic(err)
		}
		close(mainDone)
	}()
	return <-cerr
}

func createNativeWindow(opts *Options) (*window, error) {
	windows.SetProcessDPIAware()
	cfg := configForDC()
	hInst, err := windows.GetModuleHandle()
	if err != nil {
		return nil, err
	}
	curs, err := windows.LoadCursor(windows.IDC_ARROW)
	if err != nil {
		return nil, err
	}
	wcls := windows.WndClassEx{
		CbSize:        uint32(unsafe.Sizeof(windows.WndClassEx{})),
		Style:         windows.CS_HREDRAW | windows.CS_VREDRAW | windows.CS_OWNDC,
		LpfnWndProc:   syscall.NewCallback(windowProc),
		HInstance:     hInst,
		HCursor:       curs,
		LpszClassName: syscall.StringToUTF16Ptr("GioWindow"),
	}
	cls, err := windows.RegisterClassEx(&wcls)
	if err != nil {
		return nil, err
	}
	wr := windows.Rect{
		Right:  int32(cfg.Px(opts.Width)),
		Bottom: int32(cfg.Px(opts.Height)),
	}
	dwStyle := uint32(windows.WS_OVERLAPPEDWINDOW)
	dwExStyle := uint32(windows.WS_EX_APPWINDOW | windows.WS_EX_WINDOWEDGE)
	windows.AdjustWindowRectEx(&wr, dwStyle, 0, dwExStyle)
	hwnd, err := windows.CreateWindowEx(dwExStyle,
		cls,
		opts.Title,
		dwStyle|windows.WS_CLIPSIBLINGS|windows.WS_CLIPCHILDREN,
		windows.CW_USEDEFAULT, windows.CW_USEDEFAULT,
		wr.Right-wr.Left,
		wr.Bottom-wr.Top,
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
	w.hdc, err = windows.GetDC(hwnd)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func windowProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	w := winMap[hwnd]
	switch msg {
	case windows.WM_UNICHAR:
		if wParam == windows.UNICODE_NOCHAR {
			// Tell the system that we accept WM_UNICHAR messages.
			return 1
		}
		fallthrough
	case windows.WM_CHAR:
		if r := rune(wParam); unicode.IsPrint(r) {
			w.w.Event(key.EditEvent{Text: string(r)})
		}
		// The message is processed.
		return 1
	case windows.WM_DPICHANGED:
		// Let Windows know we're prepared for runtime DPI changes.
		return 1
	case windows.WM_ERASEBKGND:
		// Avoid flickering between GPU content and background color.
		return 1
	case windows.WM_KEYDOWN, windows.WM_SYSKEYDOWN:
		if n, ok := convertKeyCode(wParam); ok {
			w.w.Event(key.Event{Name: n, Modifiers: getModifiers()})
		}
	case windows.WM_LBUTTONDOWN:
		w.pointerButton(pointer.ButtonLeft, true, lParam, getModifiers())
	case windows.WM_LBUTTONUP:
		w.pointerButton(pointer.ButtonLeft, false, lParam, getModifiers())
	case windows.WM_RBUTTONDOWN:
		w.pointerButton(pointer.ButtonRight, true, lParam, getModifiers())
	case windows.WM_RBUTTONUP:
		w.pointerButton(pointer.ButtonRight, false, lParam, getModifiers())
	case windows.WM_MBUTTONDOWN:
		w.pointerButton(pointer.ButtonMiddle, true, lParam, getModifiers())
	case windows.WM_MBUTTONUP:
		w.pointerButton(pointer.ButtonMiddle, false, lParam, getModifiers())
	case windows.WM_CANCELMODE:
		w.w.Event(pointer.Event{
			Type: pointer.Cancel,
		})
	case windows.WM_SETFOCUS:
		w.w.Event(key.FocusEvent{Focus: true})
	case windows.WM_KILLFOCUS:
		w.w.Event(key.FocusEvent{Focus: false})
	case windows.WM_MOUSEMOVE:
		x, y := coordsFromlParam(lParam)
		p := f32.Point{X: float32(x), Y: float32(y)}
		w.w.Event(pointer.Event{
			Type:     pointer.Move,
			Source:   pointer.Mouse,
			Position: p,
			Time:     windows.GetMessageTime(),
		})
	case windows.WM_MOUSEWHEEL:
		w.scrollEvent(wParam, lParam)
	case windows.WM_DESTROY:
		w.dead = true
	case windows.WM_PAINT:
		w.draw(true)
	case windows.WM_SIZE:
		switch wParam {
		case windows.SIZE_MINIMIZED:
			w.setStage(system.StagePaused)
		case windows.SIZE_MAXIMIZED, windows.SIZE_RESTORED:
			w.setStage(system.StageRunning)
		}
	}
	return windows.DefWindowProc(hwnd, msg, wParam, lParam)
}

func getModifiers() key.Modifiers {
	var kmods key.Modifiers
	if windows.GetKeyState(windows.VK_LWIN)&0x1000 != 0 || windows.GetKeyState(windows.VK_RWIN)&0x1000 != 0 {
		kmods |= key.ModSuper
	}
	if windows.GetKeyState(windows.VK_MENU)&0x1000 != 0 {
		kmods |= key.ModAlt
	}
	if windows.GetKeyState(windows.VK_CONTROL)&0x1000 != 0 {
		kmods |= key.ModCtrl
	}
	if windows.GetKeyState(windows.VK_SHIFT)&0x1000 != 0 {
		kmods |= key.ModShift
	}
	return kmods
}

func (w *window) pointerButton(btn pointer.Buttons, press bool, lParam uintptr, kmods key.Modifiers) {
	var typ pointer.Type
	if press {
		typ = pointer.Press
		if w.pointerBtns == 0 {
			windows.SetCapture(w.hwnd)
		}
		w.pointerBtns |= btn
	} else {
		typ = pointer.Release
		w.pointerBtns &^= btn
		if w.pointerBtns == 0 {
			windows.ReleaseCapture()
		}
	}
	x, y := coordsFromlParam(lParam)
	p := f32.Point{X: float32(x), Y: float32(y)}
	w.w.Event(pointer.Event{
		Type:      typ,
		Source:    pointer.Mouse,
		Position:  p,
		Buttons:   w.pointerBtns,
		Time:      windows.GetMessageTime(),
		Modifiers: kmods,
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
	np := windows.Point{X: int32(x), Y: int32(y)}
	windows.ScreenToClient(w.hwnd, &np)
	p := f32.Point{X: float32(np.X), Y: float32(np.Y)}
	dist := float32(int16(wParam >> 16))
	w.w.Event(pointer.Event{
		Type:     pointer.Move,
		Source:   pointer.Mouse,
		Position: p,
		Scroll:   f32.Point{Y: -dist},
		Time:     windows.GetMessageTime(),
	})
}

// Adapted from https://blogs.msdn.microsoft.com/oldnewthing/20060126-00/?p=32513/
func (w *window) loop() error {
	msg := new(windows.Msg)
	for !w.dead {
		w.mu.Lock()
		anim := w.animating
		w.mu.Unlock()
		if anim && !windows.PeekMessage(msg, w.hwnd, 0, 0, windows.PM_NOREMOVE) {
			w.draw(false)
			continue
		}
		windows.GetMessage(msg, w.hwnd, 0, 0)
		if msg.Message == windows.WM_QUIT {
			windows.PostQuitMessage(msg.WParam)
			break
		}
		windows.TranslateMessage(msg)
		windows.DispatchMessage(msg)
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
	if err := windows.PostMessage(w.hwnd, _WM_REDRAW, 0, 0); err != nil {
		panic(err)
	}
}

func (w *window) setStage(s system.Stage) {
	w.stage = s
	w.w.Event(system.StageEvent{Stage: s})
}

func (w *window) draw(sync bool) {
	var r windows.Rect
	windows.GetClientRect(w.hwnd, &r)
	w.width = int(r.Right - r.Left)
	w.height = int(r.Bottom - r.Top)
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
		windows.ReleaseDC(w.hdc)
		w.hdc = 0
	}
	if w.hwnd != 0 {
		windows.DestroyWindow(w.hwnd)
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
	case windows.VK_ESCAPE:
		r = key.NameEscape
	case windows.VK_LEFT:
		r = key.NameLeftArrow
	case windows.VK_RIGHT:
		r = key.NameRightArrow
	case windows.VK_RETURN:
		r = key.NameReturn
	case windows.VK_UP:
		r = key.NameUpArrow
	case windows.VK_DOWN:
		r = key.NameDownArrow
	case windows.VK_HOME:
		r = key.NameHome
	case windows.VK_END:
		r = key.NameEnd
	case windows.VK_BACK:
		r = key.NameDeleteBackward
	case windows.VK_DELETE:
		r = key.NameDeleteForward
	case windows.VK_PRIOR:
		r = key.NamePageUp
	case windows.VK_NEXT:
		r = key.NamePageDown
	case windows.VK_F1:
		r = "F1"
	case windows.VK_F2:
		r = "F2"
	case windows.VK_F3:
		r = "F3"
	case windows.VK_F4:
		r = "F4"
	case windows.VK_F5:
		r = "F5"
	case windows.VK_F6:
		r = "F6"
	case windows.VK_F7:
		r = "F7"
	case windows.VK_F8:
		r = "F8"
	case windows.VK_F9:
		r = "F9"
	case windows.VK_F10:
		r = "F10"
	case windows.VK_F11:
		r = "F11"
	case windows.VK_F12:
		r = "F12"
	case windows.VK_TAB:
		r = key.NameTab
	case windows.VK_SPACE:
		r = "Space"
	case windows.VK_OEM_1:
		r = ";"
	case windows.VK_OEM_PLUS:
		r = "+"
	case windows.VK_OEM_COMMA:
		r = ","
	case windows.VK_OEM_MINUS:
		r = "-"
	case windows.VK_OEM_PERIOD:
		r = "."
	case windows.VK_OEM_2:
		r = "/"
	case windows.VK_OEM_3:
		r = "`"
	case windows.VK_OEM_4:
		r = "["
	case windows.VK_OEM_5, windows.VK_OEM_102:
		r = "\\"
	case windows.VK_OEM_6:
		r = "]"
	case windows.VK_OEM_7:
		r = "'"
	default:
		return "", false
	}
	return r, true
}

func configForDC() config {
	dpi := windows.GetSystemDPI()
	const inchPrDp = 1.0 / 96.0
	ppdp := float32(dpi) * inchPrDp
	return config{
		pxPerDp: ppdp,
		pxPerSp: ppdp,
	}
}
