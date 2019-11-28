// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nox11 freebsd

package window

/*
#cgo LDFLAGS: -lX11 -lxkbcommon -lxkbcommon-x11 -lX11-xcb
#include <stdlib.h>
#include <locale.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/Xresource.h>
#include <X11/XKBlib.h>
#include <X11/Xlib-xcb.h>
#include <xkbcommon/xkbcommon-x11.h>

*/
import "C"
import (
	"errors"
	"fmt"
	"image"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"

	"gioui.org/app/internal/xkb"
	syscall "golang.org/x/sys/unix"
)

type x11Window struct {
	w            Callbacks
	x            *C.Display
	xkb          *xkb.Context
	xkbEventBase C.int
	xw           C.Window

	evDelWindow C.Atom
	stage       system.Stage
	cfg         config
	width       int
	height      int
	notify      struct {
		read, write int
	}
	dead bool

	mu        sync.Mutex
	animating bool

	pointerBtns pointer.Buttons
}

func (w *x11Window) SetAnimating(anim bool) {
	w.mu.Lock()
	w.animating = anim
	w.mu.Unlock()
	if anim {
		w.wakeup()
	}
}

func (w *x11Window) ShowTextInput(show bool) {}

var x11OneByte = make([]byte, 1)

func (w *x11Window) wakeup() {
	if _, err := syscall.Write(w.notify.write, x11OneByte); err != nil && err != syscall.EAGAIN {
		panic(fmt.Errorf("failed to write to pipe: %v", err))
	}
}

func (w *x11Window) display() *C.Display {
	return w.x
}

func (w *x11Window) window() (C.Window, int, int) {
	return w.xw, w.width, w.height
}

func (w *x11Window) setStage(s system.Stage) {
	if s == w.stage {
		return
	}
	w.stage = s
	w.w.Event(system.StageEvent{s})
}

func (w *x11Window) loop() {
	h := x11EventHandler{w: w, xev: new(C.XEvent), text: make([]byte, 4)}
	xfd := C.XConnectionNumber(w.x)

	// Poll for events and notifications.
	pollfds := []syscall.PollFd{
		{Fd: int32(xfd), Events: syscall.POLLIN | syscall.POLLERR},
		{Fd: int32(w.notify.read), Events: syscall.POLLIN | syscall.POLLERR},
	}
	xEvents := &pollfds[0].Revents
	// Plenty of room for a backlog of notifications.
	buf := make([]byte, 100)

loop:
	for !w.dead {
		var syn, redraw bool
		// Check for pending draw events before checking animation or blocking.
		// This fixes an issue on Xephyr where on startup XPending() > 0 but
		// poll will still block. This also prevents no-op calls to poll.
		if syn = h.handleEvents(); !syn {
			w.mu.Lock()
			animating := w.animating
			w.mu.Unlock()
			if animating {
				redraw = true
			} else {
				// Clear poll events.
				*xEvents = 0
				// Wait for X event or gio notification.
				if _, err := syscall.Poll(pollfds, -1); err != nil && err != syscall.EINTR {
					panic(fmt.Errorf("x11 loop: poll failed: %w", err))
				}
				switch {
				case *xEvents&syscall.POLLIN != 0:
					syn = h.handleEvents()
					if w.dead {
						break loop
					}
				case *xEvents&(syscall.POLLERR|syscall.POLLHUP) != 0:
					break loop
				}
			}
		}
		// Clear notifications.
		for {
			_, err := syscall.Read(w.notify.read, buf)
			if err == syscall.EAGAIN {
				break
			}
			if err != nil {
				panic(fmt.Errorf("x11 loop: read from notify pipe failed: %w", err))
			}
			redraw = true
		}

		if redraw || syn {
			w.cfg.now = time.Now()
			w.w.Event(FrameEvent{
				FrameEvent: system.FrameEvent{
					Size: image.Point{
						X: w.width,
						Y: w.height,
					},
					Config: &w.cfg,
				},
				Sync: syn,
			})
		}
	}
	w.w.Event(system.DestroyEvent{Err: nil})
}

func (w *x11Window) destroy() {
	if w.notify.write != 0 {
		syscall.Close(w.notify.write)
		w.notify.write = 0
	}
	if w.notify.read != 0 {
		syscall.Close(w.notify.read)
		w.notify.read = 0
	}
	if w.xkb != nil {
		w.xkb.Destroy()
		w.xkb = nil
	}
	C.XDestroyWindow(w.x, w.xw)
	C.XCloseDisplay(w.x)
}

// atom is a wrapper around XInternAtom. Callers should cache the result
// in order to limit round-trips to the X server.
//
func (w *x11Window) atom(name string, onlyIfExists bool) C.Atom {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	flag := C.Bool(C.False)
	if onlyIfExists {
		flag = C.True
	}
	return C.XInternAtom(w.x, cname, flag)
}

// x11EventHandler wraps static variables for the main event loop.
// Its sole purpose is to prevent heap allocation and reduce clutter
// in x11window.loop.
//
type x11EventHandler struct {
	w      *x11Window
	text   []byte
	xev    *C.XEvent
	status C.Status
	keysym C.KeySym
}

// handleEvents returns true if the window needs to be redrawn.
//
func (h *x11EventHandler) handleEvents() bool {
	w := h.w
	xev := h.xev
	redraw := false
	for C.XPending(w.x) != 0 {
		C.XNextEvent(w.x, xev)
		if C.XFilterEvent(xev, C.None) == C.True {
			continue
		}
		switch _type := (*C.XAnyEvent)(unsafe.Pointer(xev))._type; _type {
		case h.w.xkbEventBase:
			xkbEvent := (*C.XkbAnyEvent)(unsafe.Pointer(xev))
			switch xkbEvent.xkb_type {
			case C.XkbNewKeyboardNotify, C.XkbMapNotify:
				if err := h.w.updateXkbKeymap(); err != nil {
					panic(err)
				}
			case C.XkbStateNotify:
				state := (*C.XkbStateNotifyEvent)(unsafe.Pointer(xev))
				h.w.xkb.UpdateMask(uint32(state.base_mods), uint32(state.latched_mods), uint32(state.locked_mods),
					uint32(state.base_group), uint32(state.latched_group), uint32(state.locked_group))
			}
		case C.KeyPress:
			kevt := (*C.XKeyPressedEvent)(unsafe.Pointer(xev))
			for _, e := range h.w.xkb.DispatchKey(uint32(kevt.keycode)) {
				w.w.Event(e)
			}
		case C.KeyRelease:
		case C.ButtonPress, C.ButtonRelease:
			bevt := (*C.XButtonEvent)(unsafe.Pointer(xev))
			ev := pointer.Event{
				Type:   pointer.Press,
				Source: pointer.Mouse,
				Position: f32.Point{
					X: float32(bevt.x),
					Y: float32(bevt.y),
				},
				Time: time.Duration(bevt.time) * time.Millisecond,
			}
			if bevt._type == C.ButtonRelease {
				ev.Type = pointer.Release
			}
			var btn pointer.Buttons
			const scrollScale = 10
			switch bevt.button {
			case C.Button1:
				btn = pointer.ButtonLeft
			case C.Button2:
				btn = pointer.ButtonMiddle
			case C.Button3:
				btn = pointer.ButtonRight
			case C.Button4:
				// scroll up
				ev.Type = pointer.Move
				ev.Scroll.Y = -scrollScale
			case C.Button5:
				// scroll down
				ev.Type = pointer.Move
				ev.Scroll.Y = +scrollScale
			default:
				continue
			}
			switch _type {
			case C.ButtonPress:
				w.pointerBtns |= btn
			case C.ButtonRelease:
				w.pointerBtns &^= btn
			}
			ev.Buttons = w.pointerBtns
			w.w.Event(ev)
		case C.MotionNotify:
			mevt := (*C.XMotionEvent)(unsafe.Pointer(xev))
			w.w.Event(pointer.Event{
				Type:    pointer.Move,
				Source:  pointer.Mouse,
				Buttons: w.pointerBtns,
				Position: f32.Point{
					X: float32(mevt.x),
					Y: float32(mevt.y),
				},
				Time: time.Duration(mevt.time) * time.Millisecond,
			})
		case C.Expose: // update
			// redraw only on the last expose event
			redraw = (*C.XExposeEvent)(unsafe.Pointer(xev)).count == 0
		case C.FocusIn:
			w.w.Event(key.FocusEvent{Focus: true})
		case C.FocusOut:
			w.w.Event(key.FocusEvent{Focus: false})
		case C.ConfigureNotify: // window configuration change
			cevt := (*C.XConfigureEvent)(unsafe.Pointer(xev))
			w.width = int(cevt.width)
			w.height = int(cevt.height)
			// redraw will be done by a later expose event
		case C.ClientMessage: // extensions
			cevt := (*C.XClientMessageEvent)(unsafe.Pointer(xev))
			switch *(*C.long)(unsafe.Pointer(&cevt.data)) {
			case C.long(w.evDelWindow):
				w.dead = true
				return false
			}
		}
	}
	return redraw
}

var (
	x11Threads sync.Once
)

func init() {
	x11Driver = newX11Window
}

func newX11Window(gioWin Callbacks, opts *Options) error {
	var err error

	pipe := make([]int, 2)
	if err := syscall.Pipe2(pipe, syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		return fmt.Errorf("NewX11Window: failed to create pipe: %w", err)
	}

	x11Threads.Do(func() {
		if C.XInitThreads() == 0 {
			err = errors.New("x11: threads init failed")
		}
		C.XrmInitialize()
	})
	if err != nil {
		return err
	}
	dpy := C.XOpenDisplay(nil)
	if dpy == nil {
		return errors.New("x11: cannot connect to the X server")
	}
	var major, minor C.int = C.XkbMajorVersion, C.XkbMinorVersion
	var xkbEventBase C.int
	if C.XkbQueryExtension(dpy, nil, &xkbEventBase, nil, &major, &minor) != C.True {
		C.XCloseDisplay(dpy)
		return errors.New("x11: XkbQueryExtension failed")
	}
	const bits = C.uint(C.XkbNewKeyboardNotifyMask | C.XkbMapNotifyMask | C.XkbStateNotifyMask)
	if C.XkbSelectEvents(dpy, C.XkbUseCoreKbd, bits, bits) != C.True {
		C.XCloseDisplay(dpy)
		return errors.New("x11: XkbSelectEvents failed")
	}
	xkb, err := xkb.New()
	if err != nil {
		C.XCloseDisplay(dpy)
		return fmt.Errorf("x11: %v", err)
	}

	ppsp := x11DetectUIScale(dpy)
	cfg := config{pxPerDp: ppsp, pxPerSp: ppsp}
	swa := C.XSetWindowAttributes{
		event_mask: C.ExposureMask | C.FocusChangeMask | // update
			C.KeyPressMask | C.KeyReleaseMask | // keyboard
			C.ButtonPressMask | C.ButtonReleaseMask | // mouse clicks
			C.PointerMotionMask | // mouse movement
			C.StructureNotifyMask, // resize
		background_pixmap: C.None,
		override_redirect: C.False,
	}
	win := C.XCreateWindow(dpy, C.XDefaultRootWindow(dpy),
		0, 0, C.uint(cfg.Px(opts.Width)), C.uint(cfg.Px(opts.Height)),
		0, C.CopyFromParent, C.InputOutput, nil,
		C.CWEventMask|C.CWBackPixmap|C.CWOverrideRedirect, &swa)

	w := &x11Window{
		w: gioWin, x: dpy, xw: win,
		width:        cfg.Px(opts.Width),
		height:       cfg.Px(opts.Height),
		cfg:          cfg,
		xkb:          xkb,
		xkbEventBase: xkbEventBase,
	}
	w.notify.read = pipe[0]
	w.notify.write = pipe[1]

	if err := w.updateXkbKeymap(); err != nil {
		w.destroy()
		return err
	}

	var hints C.XWMHints
	hints.input = C.True
	hints.flags = C.InputHint
	C.XSetWMHints(dpy, win, &hints)

	// set the name
	ctitle := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(ctitle))
	C.XStoreName(dpy, win, ctitle)
	// set _NET_WM_NAME as well for UTF-8 support in window title.
	C.XSetTextProperty(dpy, win,
		&C.XTextProperty{
			value:    (*C.uchar)(unsafe.Pointer(ctitle)),
			encoding: w.atom("UTF8_STRING", false),
			format:   8,
			nitems:   C.ulong(len(opts.Title)),
		},
		w.atom("_NET_WM_NAME", false))

	// extensions
	w.evDelWindow = w.atom("WM_DELETE_WINDOW", false)
	C.XSetWMProtocols(dpy, win, &w.evDelWindow, 1)

	// make the window visible on the screen
	C.XMapWindow(dpy, win)

	go func() {
		w.w.SetDriver(w)
		w.setStage(system.StageRunning)
		w.loop()
		w.destroy()
		close(mainDone)
	}()
	return nil
}

// detectUIScale reports the system UI scale, or 1.0 if it fails.
func x11DetectUIScale(dpy *C.Display) float32 {
	// default fixed DPI value used in most desktop UI toolkits
	const defaultDesktopDPI = 96
	var scale float32 = 1.0

	// Get actual DPI from X resource Xft.dpi (set by GTK and Qt).
	// This value is entirely based on user preferences and conflates both
	// screen (UI) scaling and font scale.
	rms := C.XResourceManagerString(dpy)
	if rms != nil {
		db := C.XrmGetStringDatabase(rms)
		if db != nil {
			var (
				t *C.char
				v C.XrmValue
			)
			if C.XrmGetResource(db, (*C.char)(unsafe.Pointer(&[]byte("Xft.dpi\x00")[0])),
				(*C.char)(unsafe.Pointer(&[]byte("Xft.Dpi\x00")[0])), &t, &v) != C.False {
				if t != nil && C.GoString(t) == "String" {
					f, err := strconv.ParseFloat(C.GoString(v.addr), 32)
					if err == nil {
						scale = float32(f) / defaultDesktopDPI
					}
				}
			}
			C.XrmDestroyDatabase(db)
		}
	}

	return scale
}

func (w *x11Window) updateXkbKeymap() error {
	w.xkb.DestroyKeymapState()
	ctx := (*C.struct_xkb_context)(unsafe.Pointer(w.xkb.Ctx))
	xcb := C.XGetXCBConnection(w.x)
	if xcb == nil {
		return errors.New("x11: XGetXCBConnection failed")
	}
	xkbDevID := C.xkb_x11_get_core_keyboard_device_id(xcb)
	if xkbDevID == -1 {
		return errors.New("x11: xkb_x11_get_core_keyboard_device_id failed")
	}
	keymap := C.xkb_x11_keymap_new_from_device(ctx, xcb, xkbDevID, C.XKB_KEYMAP_COMPILE_NO_FLAGS)
	if keymap == nil {
		return errors.New("x11: xkb_x11_keymap_new_from_device failed")
	}
	state := C.xkb_x11_state_new_from_device(keymap, xcb, xkbDevID)
	if state == nil {
		C.xkb_keymap_unref(keymap)
		return errors.New("x11: xkb_x11_keymap_new_from_device failed")
	}
	w.xkb.SetKeymap(unsafe.Pointer(keymap), unsafe.Pointer(state))
	return nil
}
