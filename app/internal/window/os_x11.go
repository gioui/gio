// SPDX-License-Identifier: Unlicense OR MIT

// +build linux,!android,!nox11 freebsd

package window

/*
#cgo LDFLAGS: -lX11
#include <stdlib.h>
#include <locale.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/Xresource.h>

void gio_x11_init_ime(Display *dpy, Window win, XIM *xim, XIC *xic) {
	// adjust locale temporarily for XOpenIM
	char *lc = setlocale(LC_CTYPE, NULL);
	setlocale(LC_CTYPE, "");
	XSetLocaleModifiers("");

	*xim = XOpenIM(dpy, 0, 0, 0);
	if (!*xim) {
		// fallback to internal input method
		XSetLocaleModifiers("@im=none");
		*xim = XOpenIM(dpy, 0, 0, 0);
	}

	// revert locale to prevent any unexpected side effects
	setlocale(LC_CTYPE, lc);

	*xic = XCreateIC(*xim,
		XNInputStyle, XIMPreeditNothing | XIMStatusNothing,
		XNClientWindow, win,
		XNFocusWindow, win,
		NULL);

	XSetICFocus(*xic);
}
*/
import "C"
import (
	"errors"
	"fmt"
	"image"
	"strconv"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
	"unsafe"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	syscall "golang.org/x/sys/unix"
)

type x11Window struct {
	w  Callbacks
	x  *C.Display
	xw C.Window

	evDelWindow C.Atom
	stage       system.Stage
	cfg         config
	width       int
	height      int
	xim         C.XIM
	xic         C.XIC
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

func (w *x11Window) display() unsafe.Pointer {
	return unsafe.Pointer(w.x)
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
	C.XDestroyIC(w.xic)
	C.XCloseIM(w.xim)
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
		case C.KeyPress:
			kevt := (*C.XKeyPressedEvent)(unsafe.Pointer(xev))
		lookup:
			// Save state then clear CTRL & Shift bits in order to have
			// Xutf8LookupString return the unmodified key name in text[:l].
			// This addresses an issue on some non US keyboard layouts where
			// CTRL-[0..9] do not behave consistently.
			//
			// Note that this enables sending a key.Event for key combinations
			// like CTRL-SHIFT-/ on QWERTY layouts, but CTRL-? is completely
			// masked. The same applies to AZERTY layouts where CTRL-SHIFT-É is
			// available but not CTRL-2.
			state := kevt.state
			mods := x11KeyStateToModifiers(state)
			if mods.Contain(key.ModCtrl) {
				kevt.state &^= (C.uint(C.ControlMask) | C.uint(C.ShiftMask))
			}
			l := int(C.Xutf8LookupString(w.xic, kevt,
				(*C.char)(unsafe.Pointer(&h.text[0])), C.int(len(h.text)),
				&h.keysym, &h.status))
			switch h.status {
			case C.XBufferOverflow:
				h.text = make([]byte, l)
				goto lookup
			case C.XLookupChars:
				// Synthetic event from XIM.
				w.w.Event(key.EditEvent{Text: string(h.text[:l])})
			case C.XLookupKeySym:
				// Special keys.
				if n, m, ok := x11ConvertKeysym(h.keysym, mods); ok {
					w.w.Event(key.Event{Name: n, Modifiers: m})
				}
			case C.XLookupBoth:
				if n, m, ok := x11ConvertKeysym(h.keysym, mods); ok {
					w.w.Event(key.Event{Name: n, Modifiers: m})
				}
				// Do not send EditEvent for CTRL key combinations.
				if mods.Contain(key.ModCtrl) {
					break
				}
				// Report only printable runes.
				str := h.text[:l]
				for n := 0; n < len(str); {
					r, s := utf8.DecodeRune(str)
					if unicode.IsPrint(r) {
						n += s
					} else {
						copy(str[n:], str[n+s:])
						str = str[:len(str)-s]
					}
				}
				if len(str) > 0 {
					w.w.Event(key.EditEvent{Text: string(str)})
				}
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

func x11KeyStateToModifiers(s C.uint) key.Modifiers {
	var m key.Modifiers
	if s&C.Mod1Mask != 0 {
		m |= key.ModAlt
	}
	if s&C.Mod4Mask != 0 {
		m |= key.ModSuper
	}
	if s&C.ControlMask != 0 {
		m |= key.ModCtrl
	}
	if s&C.ShiftMask != 0 {
		m |= key.ModShift
	}
	return m
}

// x11ConvertKeysym returns the Gio special key that matches keysym s. For
// portability reasons, some keysyms might be translated and modifiers changed
// (like BackTab -> ModShift+Tab)
func x11ConvertKeysym(s C.KeySym, mods key.Modifiers) (string, key.Modifiers, bool) {
	if '0' <= s && s <= '9' || 'A' <= s && s <= 'Z' {
		return string(s), mods, true
	}
	if 'a' <= s && s <= 'z' {
		return string(s - 0x20), mods, true
	}
	var n string
	switch s {
	case C.XK_Escape:
		n = key.NameEscape
	case C.XK_Left, C.XK_KP_Left:
		n = key.NameLeftArrow
	case C.XK_Right, C.XK_KP_Right:
		n = key.NameRightArrow
	case C.XK_Return:
		n = key.NameReturn
	case C.XK_KP_Enter:
		n = key.NameEnter
	case C.XK_Up, C.XK_KP_Up:
		n = key.NameUpArrow
	case C.XK_Down, C.XK_KP_Down:
		n = key.NameDownArrow
	case C.XK_Home, C.XK_KP_Home:
		n = key.NameHome
	case C.XK_End, C.XK_KP_End:
		n = key.NameEnd
	case C.XK_BackSpace:
		n = key.NameDeleteBackward
	case C.XK_Delete, C.XK_KP_Delete:
		n = key.NameDeleteForward
	case C.XK_Page_Up, C.XK_KP_Prior:
		n = key.NamePageUp
	case C.XK_Page_Down, C.XK_KP_Next:
		n = key.NamePageDown
	case C.XK_F1:
		n = "F1"
	case C.XK_F2:
		n = "F2"
	case C.XK_F3:
		n = "F3"
	case C.XK_F4:
		n = "F4"
	case C.XK_F5:
		n = "F5"
	case C.XK_F6:
		n = "F6"
	case C.XK_F7:
		n = "F7"
	case C.XK_F8:
		n = "F8"
	case C.XK_F9:
		n = "F9"
	case C.XK_F10:
		n = "F10"
	case C.XK_F11:
		n = "F11"
	case C.XK_F12:
		n = "F12"
	case C.XK_ISO_Left_Tab:
		mods |= key.ModShift
		fallthrough
	case C.XK_Tab:
		n = key.NameTab
	case 0x20, C.XK_KP_Space:
		n = "Space"
	default:
		return "", mods, false
	}
	return n, mods, true
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
	var (
		xim C.XIM
		xic C.XIC
	)
	C.gio_x11_init_ime(dpy, win, &xim, &xic)

	w := &x11Window{
		w: gioWin, x: dpy, xw: win,
		width:  cfg.Px(opts.Width),
		height: cfg.Px(opts.Height),
		cfg:    cfg,
		xim:    xim,
		xic:    xic,
	}
	w.notify.read = pipe[0]
	w.notify.write = pipe[1]

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
