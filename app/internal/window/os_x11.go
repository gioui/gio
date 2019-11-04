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
#define GIO_FIELD_OFFSET(typ, field) const int gio_##typ##_##field##_off = offsetof(typ, field)
GIO_FIELD_OFFSET(XClientMessageEvent, data);
GIO_FIELD_OFFSET(XExposeEvent, count);
GIO_FIELD_OFFSET(XConfigureEvent, width);
GIO_FIELD_OFFSET(XConfigureEvent, height);
GIO_FIELD_OFFSET(XButtonEvent, x);
GIO_FIELD_OFFSET(XButtonEvent, y);
GIO_FIELD_OFFSET(XButtonEvent, state);
GIO_FIELD_OFFSET(XButtonEvent, button);
GIO_FIELD_OFFSET(XButtonEvent, time);
GIO_FIELD_OFFSET(XMotionEvent, x);
GIO_FIELD_OFFSET(XMotionEvent, y);
GIO_FIELD_OFFSET(XMotionEvent, time);
GIO_FIELD_OFFSET(XKeyEvent, state);

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

int gio_x11_connection_number(Display *dpy) {
	return ConnectionNumber(dpy);
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
	h := x11EventHandler{w: w, xev: new(xEvent), text: make([]byte, 4)}
	xfd := C.gio_x11_connection_number(w.x)

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

// x11EventHandler wraps static variables for the main event loop.
// Its sole purpose is to prevent heap allocation and reduce clutter
// in x11window.loop.
//
type x11EventHandler struct {
	w      *x11Window
	text   []byte
	xev    *xEvent
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
		C.XNextEvent(w.x, (*C.XEvent)(unsafe.Pointer(xev)))
		if C.XFilterEvent((*C.XEvent)(unsafe.Pointer(xev)), C.None) == C.True {
			continue
		}
		switch xev.Type {
		case C.KeyPress:
		lookup:
			l := int(C.Xutf8LookupString(w.xic,
				(*C.XKeyPressedEvent)(unsafe.Pointer(xev)),
				(*C.char)(unsafe.Pointer(&h.text[0])), C.int(len(h.text)),
				&h.keysym, &h.status))
			switch h.status {
			case C.XBufferOverflow:
				h.text = make([]byte, l)
				goto lookup
			case C.XLookupChars:
				w.w.Event(key.EditEvent{Text: string(h.text[:l])})
			case C.XLookupKeySym:
				if r, ok := x11KeySymToRune(h.keysym); ok {
					w.w.Event(key.Event{
						Name:      r,
						Modifiers: x11KeyStateToModifiers(xev.GetKeyState()),
					})
				}
			case C.XLookupBoth:
				// here we need to choose if we send a key.Event or key.EditEvent
				mods := x11KeyStateToModifiers(xev.GetKeyState())
				if mods&key.ModCommand != 0 {
					r, ok := x11KeySymToRune(h.keysym)
					if !ok {
						// on AZERTY keyboards, CTRL-1, 2, etc do not have a consistent behavior.
						// Since keysim as set by Xutf8LookupString is layout dependent, get its layout independent
						// version and use that instead (i.e. send CTRL-1, CTRL-2, etc. instead of CTRL-&, CTRL-é, …)
						r, ok = x11KeySymToRune(C.XLookupKeysym((*C.XKeyEvent)(unsafe.Pointer(xev)), 0))
					}
					if ok {
						w.w.Event(key.Event{Name: r, Modifiers: mods})
					}
				} else if r, ok := x11SpecialKeySymToRune(h.keysym); ok {
					w.w.Event(key.Event{Name: r, Modifiers: mods})
				} else {
					w.w.Event(key.EditEvent{Text: string(h.text[:l])})
				}
			}
		case C.KeyRelease:
		case C.ButtonPress, C.ButtonRelease:
			ev := pointer.Event{
				Type:   pointer.Press,
				Source: pointer.Mouse,
				Position: f32.Point{
					X: float32(xev.GetButtonX()),
					Y: float32(xev.GetButtonY()),
				},
				Time: xev.GetButtonTime(),
			}
			if xev.Type == C.ButtonRelease {
				ev.Type = pointer.Release
			}
			const scrollScale = 10
			switch xev.GetButtonButton() {
			case C.Button1:
				// left click by default
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
			w.w.Event(ev)
		case C.MotionNotify:
			w.w.Event(pointer.Event{
				Type:   pointer.Move,
				Source: pointer.Mouse,
				Position: f32.Point{
					X: float32(xev.GetMotionX()),
					Y: float32(xev.GetMotionY()),
				},
				Time: xev.GetMotionTime(),
			})
		case C.Expose: // update
			// redraw only on the last expose event
			redraw = xev.GetExposeCount() == 0
		case C.FocusIn:
			w.w.Event(key.FocusEvent{Focus: true})
		case C.FocusOut:
			w.w.Event(key.FocusEvent{Focus: false})
		case C.ConfigureNotify: // window configuration change
			w.width = int(xev.GetConfigureWidth())
			w.height = int(xev.GetConfigureHeight())
			// redraw will be done by a later expose event
		case C.ClientMessage: // extensions
			switch xev.GetClientDataLong()[0] {
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
	if s&C.ControlMask != 0 {
		m |= key.ModCommand
	}
	if s&C.ShiftMask != 0 {
		m |= key.ModShift
	}
	return m
}

func x11KeySymToRune(s C.KeySym) (rune, bool) {
	if '0' <= s && s <= '9' || 'A' <= s && s <= 'Z' {
		return rune(s), true
	}
	if 'a' <= s && s <= 'z' {
		return rune(s - 0x20), true
	}
	return x11SpecialKeySymToRune(s)
}

func x11SpecialKeySymToRune(s C.KeySym) (rune, bool) {
	var n rune
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
	default:
		return 0, false
	}
	return n, true
}

const xEventSize = unsafe.Sizeof(C.XEvent{})

// Make sure the Go struct has the same size.
// We can't use C.XEvent directly because it's a union.
var _ = [1]struct{}{}[unsafe.Sizeof(xEvent{})-xEventSize]

type xEvent struct {
	Type C.int
	Data [xEventSize - unsafe.Sizeof(C.int(0))]byte
}

func (e *xEvent) getInt(off int) C.int {
	return *(*C.int)(unsafe.Pointer(uintptr(unsafe.Pointer(e)) + uintptr(off)))
}

func (e *xEvent) getUint(off int) C.uint {
	return *(*C.uint)(unsafe.Pointer(uintptr(unsafe.Pointer(e)) + uintptr(off)))
}

func (e *xEvent) getUlong(off int) C.ulong {
	return *(*C.ulong)(unsafe.Pointer(uintptr(unsafe.Pointer(e)) + uintptr(off)))
}

func (e *xEvent) getUlongMs(off int) time.Duration {
	return time.Duration(e.getUlong(off)) * time.Millisecond
}

// GetExposeCount returns a XEvent.xexpose.count field.
func (e *xEvent) GetExposeCount() C.int {
	return e.getInt(int(C.gio_XExposeEvent_count_off))
}

// GetConfigureWidth returns a XEvent.xconfigure.width field.
func (e *xEvent) GetConfigureWidth() C.int {
	return e.getInt(int(C.gio_XConfigureEvent_width_off))
}

// GetConfigureWidth returns a XEvent.xconfigure.height field.
func (e *xEvent) GetConfigureHeight() C.int {
	return e.getInt(int(C.gio_XConfigureEvent_height_off))
}

// GetButtonX returns a XEvent.xbutton.x field.
func (e *xEvent) GetButtonX() C.int {
	return e.getInt(int(C.gio_XButtonEvent_x_off))
}

// GetButtonY returns a XEvent.xbutton.y field.
func (e *xEvent) GetButtonY() C.int {
	return e.getInt(int(C.gio_XButtonEvent_y_off))
}

// GetButtonState returns a XEvent.xbutton.state field.
func (e *xEvent) GetButtonState() C.uint {
	return e.getUint(int(C.gio_XButtonEvent_state_off))
}

// GetButtonButton returns a XEvent.xbutton.button field.
func (e *xEvent) GetButtonButton() C.uint {
	return e.getUint(int(C.gio_XButtonEvent_button_off))
}

// GetButtonTime returns a XEvent.xbutton.time field.
func (e *xEvent) GetButtonTime() time.Duration {
	return e.getUlongMs(int(C.gio_XButtonEvent_time_off))
}

// GetMotionX returns a XEvent.xmotion.x field.
func (e *xEvent) GetMotionX() C.int {
	return e.getInt(int(C.gio_XMotionEvent_x_off))
}

// GetMotionY returns a XEvent.xmotion.y field.
func (e *xEvent) GetMotionY() C.int {
	return e.getInt(int(C.gio_XMotionEvent_y_off))
}

// GetMotionTime returns a XEvent.xmotion.time field.
func (e *xEvent) GetMotionTime() time.Duration {
	return e.getUlongMs(int(C.gio_XMotionEvent_time_off))
}

// GetClientDataLong returns a XEvent.xclient.data.l field.
func (e *xEvent) GetClientDataLong() [5]C.long {
	ptr := (*[5]C.long)(unsafe.Pointer(uintptr(unsafe.Pointer(e)) + uintptr(C.gio_XClientMessageEvent_data_off)))
	return *ptr
}

// GetKeyState returns a XKeyEvent.state field.
func (e *xEvent) GetKeyState() C.uint {
	return e.getUint(int(C.gio_XKeyEvent_state_off))
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

	root := C.XDefaultRootWindow(dpy)
	screen := C.XDefaultScreen(dpy)
	ppsp := x11DetectUIScale(dpy, screen)
	cfg := config{pxPerDp: ppsp, pxPerSp: ppsp}
	var (
		swa C.XSetWindowAttributes
		xim C.XIM
		xic C.XIC
	)
	swa.event_mask = C.ExposureMask | C.PointerMotionMask | C.KeyPressMask
	win := C.XCreateWindow(dpy, root,
		0, 0, C.uint(cfg.Px(opts.Width)), C.uint(cfg.Px(opts.Height)), 0,
		C.CopyFromParent, C.InputOutput,
		nil, C.CWEventMask|C.CWBackPixel,
		&swa)
	C.gio_x11_init_ime(dpy, win, &xim, &xic)
	C.XSelectInput(dpy, win, 0|
		C.ExposureMask|C.FocusChangeMask| // update
		C.KeyPressMask|C.KeyReleaseMask| // keyboard
		C.ButtonPressMask|C.ButtonReleaseMask| // mouse clicks
		C.PointerMotionMask| // mouse movement
		C.StructureNotifyMask, // resize
	)

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

	var xattr C.XSetWindowAttributes
	xattr.override_redirect = C.False
	C.XChangeWindowAttributes(dpy, win, C.CWOverrideRedirect, &xattr)

	var hints C.XWMHints
	hints.input = C.True
	hints.flags = C.InputHint
	C.XSetWMHints(dpy, win, &hints)

	// make the window visible on the screen
	C.XMapWindow(dpy, win)

	// set the name
	ctitle := C.CString(opts.Title)
	C.XStoreName(dpy, win, ctitle)
	C.free(unsafe.Pointer(ctitle))

	// extensions
	ckey := C.CString("WM_DELETE_WINDOW")
	w.evDelWindow = C.XInternAtom(dpy, ckey, C.False)
	C.free(unsafe.Pointer(ckey))
	C.XSetWMProtocols(dpy, win, &w.evDelWindow, 1)

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
func x11DetectUIScale(dpy *C.Display, screen C.int) float32 {
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
