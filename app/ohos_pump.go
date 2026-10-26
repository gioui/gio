// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import (
	"image"
	"io"
	"strings"
	"time"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/unit"
)

// OHOSViewEvent is sent when the XComponent surface is created or destroyed.
// An invalid event (Window == 0) releases the GPU context while keeping the
// window alive; the next surface creation recreates it.
type OHOSViewEvent struct {
	Window uintptr
}

func (OHOSViewEvent) implementsViewEvent() {}
func (OHOSViewEvent) ImplementsEvent()     {}
func (v OHOSViewEvent) Valid() bool        { return v.Window != 0 }

// ohosLifecycle values match gio_ohos_lifecycle.
type ohosLifecycle int

const (
	ohosBackground ohosLifecycle = 0
	ohosForeground ohosLifecycle = 1
	ohosDestroyed  ohosLifecycle = 2
)

// ohosInsets are system/keyboard insets in pixels.
type ohosInsets struct {
	Top, Right, Bottom, Left int
}

type ohosEventKind int

const (
	ohosEvNone ohosEventKind = iota
	ohosEvSurfaceCreated
	ohosEvSurfaceChanged
	ohosEvSurfaceDestroyed
	ohosEvTouch
	ohosEvMouse
	ohosEvAxis
	ohosEvKey
	ohosEvText
	ohosEvFocus
	ohosEvClipboard
	ohosEvLifecycle
	ohosEvFrame
	ohosEvInsets
	ohosEvPreedit
	ohosEvIMEDelete
	ohosEvIMEMove
	ohosEvIMESelection
	ohosEvIMEEnter
	ohosEvIMEKeyboard
	ohosEvPickResult
	ohosEvPickError
	ohosEvSetAnimating
	ohosEvConfigure
)

// ohosEvent is one platform callback, copied from C before it enters the
// queue.  It contains no C pointers except the opaque XComponent window
// handle, which the shim keeps alive until surface_destroyed.
type ohosEvent struct {
	kind ohosEventKind
	win  uintptr
	w, h int

	x, y   float32
	dx, dy float32
	scale  float32 // pinch scale relative to 1.0
	id     int
	action int
	code   int
	mods   int
	focus  bool
	life   ohosLifecycle

	text  string
	text2 string
	a, b  int

	// insets (pixels) for ohosEvInsets.
	top, right, bottom, left int

	// opts carries window options for ohosEvConfigure.
	opts []Option
}

// ohosSink receives the effects of the event pump.  The real implementation
// is (*window); tests use a recorder.
type ohosSink interface {
	ohosDeliver(event.Event)
	ohosRequestFrame()
	ohosEditorInsert(text string)
	ohosEditorPreedit(text string, selStart, selEnd int)
	ohosEditorDelete(before, after int)
	ohosEditorMove(direction int)
	ohosEditorSelect(start, end int)
	ohosEditorEnter(enterType int)
	ohosIMEChanged()
	ohosPickResult(mode int, uris []string)
	ohosPickError(mode int, msg string)
	ohosLog(msg string)
}

// ohosPump turns platform callbacks into Gio events.  It is deliberately free
// of cgo and of *window so that the pump can be exercised on any platform
// (docs/45 L3).  All methods must be called from the single ohosLoop goroutine.
type ohosPump struct {
	sink   ohosSink
	metric unit.Metric

	config     Config
	insets     ohosInsets
	valid      bool
	animating  bool
	life       ohosLifecycle
	surfaceWin uintptr

	// destroySent avoids delivering DestroyEvent twice.
	destroySent bool
}

func newOhosPump(sink ohosSink, metric unit.Metric) *ohosPump {
	if metric.PxPerDp <= 0 {
		metric.PxPerDp = 1
	}
	if metric.PxPerSp <= 0 {
		metric.PxPerSp = metric.PxPerDp
	}
	return &ohosPump{
		sink:   sink,
		metric: metric,
		life:   ohosForeground,
		config: Config{Decorated: true, Focused: true},
	}
}

// Handle processes one platform callback.
func (p *ohosPump) Handle(e ohosEvent) {
	switch e.kind {
	case ohosEvSurfaceCreated:
		p.surface(e.win, e.w, e.h, true)
	case ohosEvSurfaceChanged:
		p.surface(e.win, e.w, e.h, true)
	case ohosEvSurfaceDestroyed:
		p.surfaceDestroyed()
	case ohosEvTouch:
		p.touch(e)
	case ohosEvMouse:
		p.mouse(e)
	case ohosEvAxis:
		p.axis(e)
	case ohosEvKey:
		p.key(e)
	case ohosEvText:
		if e.text != "" {
			p.sink.ohosEditorInsert(e.text)
		}
	case ohosEvFocus:
		p.setFocused(e.focus)
	case ohosEvClipboard:
		p.clipboard(e.text, e.text2)
	case ohosEvLifecycle:
		p.setLifecycle(e.life)
	case ohosEvFrame:
		p.tick()
	case ohosEvInsets:
		p.setInsets(ohosInsets{Top: e.top, Right: e.right, Bottom: e.bottom, Left: e.left})
	case ohosEvPreedit:
		p.sink.ohosEditorPreedit(e.text, e.a, e.b)
		p.sink.ohosIMEChanged()
	case ohosEvIMEDelete:
		p.sink.ohosEditorDelete(e.a, e.b)
		p.sink.ohosIMEChanged()
	case ohosEvIMEMove:
		p.sink.ohosEditorMove(e.a)
		p.sink.ohosIMEChanged()
	case ohosEvIMESelection:
		p.sink.ohosEditorSelect(e.a, e.b)
		p.sink.ohosIMEChanged()
	case ohosEvIMEEnter:
		p.sink.ohosEditorEnter(e.a)
		p.sink.ohosIMEChanged()
	case ohosEvIMEKeyboard:
		// The shim reports keyboard visibility; insets arrive separately.
	case ohosEvPickResult:
		var uris []string
		for _, u := range strings.Split(e.text, "\n") {
			if u = strings.TrimSpace(u); u != "" {
				uris = append(uris, u)
			}
		}
		p.sink.ohosPickResult(e.action, uris)
	case ohosEvPickError:
		p.sink.ohosPickError(e.action, e.text)
	case ohosEvSetAnimating:
		p.SetAnimating(e.focus)
	case ohosEvConfigure:
		p.Configure(e.opts)
	}
}

// surface handles created/changed callbacks.  A zero-sized surface is
// remembered but produces no frame: creating an EGL surface for it would fail
// and spam the log (docs/45 WP-3).
func (p *ohosPump) surface(win uintptr, w, h int, sync bool) {
	if win == 0 || w <= 0 || h <= 0 {
		p.valid = false
		p.config.Size = image.Point{}
		return
	}
	first := !p.valid
	p.valid = true
	p.surfaceWin = win
	p.config.Size = image.Pt(w, h)
	if first {
		p.sink.ohosDeliver(OHOSViewEvent{Window: win})
	}
	p.sink.ohosDeliver(ConfigEvent{Config: p.config})
	p.frame(sync)
}

func (p *ohosPump) surfaceDestroyed() {
	if p.valid {
		p.valid = false
		p.surfaceWin = 0
		p.config.Size = image.Point{}
		p.sink.ohosDeliver(OHOSViewEvent{})
	}
}

// surfaceInfo returns the current OHNativeWindow handle and surface size.
// Callers run on the loop/render goroutine.
func (p *ohosPump) surfaceInfo() (uintptr, image.Point) {
	return p.surfaceWin, p.config.Size
}

// Invalidate requests a frame.  Background windows produce none.
func (p *ohosPump) Invalidate() {
	if p.life != ohosForeground || !p.valid || p.config.Size == (image.Point{}) {
		return
	}
	p.frame(false)
	if p.animating {
		p.sink.ohosRequestFrame()
	}
}

// SetAnimating starts/stops the vsync loop.
func (p *ohosPump) SetAnimating(anim bool) {
	if p.animating == anim {
		return
	}
	p.animating = anim
	if anim && p.life == ohosForeground && p.valid {
		p.sink.ohosRequestFrame()
	}
}

// tick is a vsync callback from the shim.
func (p *ohosPump) tick() {
	if !p.animating || p.life != ohosForeground || !p.valid {
		return
	}
	p.frame(false)
	p.sink.ohosRequestFrame()
}

func (p *ohosPump) frame(sync bool) {
	if !p.valid || p.config.Size.X <= 0 || p.config.Size.Y <= 0 {
		return
	}
	p.sink.ohosDeliver(frameEvent{
		FrameEvent: FrameEvent{
			Now:    time.Now(),
			Size:   p.config.Size,
			Insets: p.insets.dp(p.metric),
			Metric: p.metric,
		},
		Sync: sync,
	})
}

func (i ohosInsets) dp(m unit.Metric) Insets {
	per := m.PxPerDp
	if per <= 0 {
		per = 1
	}
	return Insets{
		Top:    unit.Dp(float32(i.Top) / per),
		Bottom: unit.Dp(float32(i.Bottom) / per),
		Left:   unit.Dp(float32(i.Left) / per),
		Right:  unit.Dp(float32(i.Right) / per),
	}
}

func (p *ohosPump) setFocused(focused bool) {
	if p.config.Focused == focused {
		return
	}
	p.config.Focused = focused
	p.sink.ohosDeliver(ConfigEvent{Config: p.config})
}

func (p *ohosPump) setInsets(in ohosInsets) {
	if p.insets == in {
		return
	}
	p.insets = in
	// Re-layout so insets (and therefore RevealFocus) take effect.
	p.frame(true)
}

func (p *ohosPump) setLifecycle(l ohosLifecycle) {
	switch l {
	case ohosBackground:
		p.life = ohosBackground
	case ohosForeground:
		p.life = ohosForeground
		if p.valid {
			p.frame(true)
		}
	case ohosDestroyed:
		p.life = ohosDestroyed
		if !p.destroySent {
			p.destroySent = true
			p.sink.ohosDeliver(DestroyEvent{})
		}
	}
}

// touch translates a pointer callback.  action: 0 down, 1 move, 2 up, 3 cancel.
func (p *ohosPump) touch(e ohosEvent) {
	var kind pointer.Kind
	switch e.action {
	case 0:
		kind = pointer.Press
	case 1:
		kind = pointer.Move
	case 2:
		kind = pointer.Release
	case 3:
		kind = pointer.Cancel
	default:
		return
	}
	p.sink.ohosDeliver(pointer.Event{
		Kind:      kind,
		Source:    pointer.Touch,
		PointerID: pointer.ID(e.id),
		// Positions arrive in surface pixels.
		Position:  f32.Pt(e.x, e.y),
		Modifiers: ohosModifiers(e.mods),
	})
}

// mouse translates a mouse callback.  action: 0 press, 1 release, 2 move.
func (p *ohosPump) mouse(e ohosEvent) {
	var kind pointer.Kind
	switch e.action {
	case 0:
		kind = pointer.Press
	case 1:
		kind = pointer.Release
	case 2:
		kind = pointer.Move
	default:
		return
	}
	var btns pointer.Buttons
	switch e.code { // button: 1 left, 2 right, 3 middle
	case 1:
		btns = pointer.ButtonPrimary
	case 2:
		btns = pointer.ButtonSecondary
	case 3:
		btns = pointer.ButtonTertiary
	}
	p.sink.ohosDeliver(pointer.Event{
		Kind:      kind,
		Source:    pointer.Mouse,
		Buttons:   btns,
		PointerID: 1,
		Position:  f32.Pt(e.x, e.y),
		Modifiers: ohosModifiers(e.mods),
	})
}

func (p *ohosPump) axis(e ohosEvent) {
	if !ohosAxisChanged(float64(e.dy), float64(e.dx), float64(e.scale)) {
		return
	}
	scroll, mods := ohosAxisScroll(float64(e.dy), float64(e.dx), float64(e.scale), e.mods)
	p.sink.ohosDeliver(pointer.Event{
		Kind:      pointer.Scroll,
		Source:    pointer.Mouse,
		PointerID: 1,
		Position:  f32.Pt(e.x, e.y),
		Scroll:    scroll,
		Modifiers: mods,
	})
}

// key translates a hardware key callback.  action: 0 down, 1 up.
func (p *ohosPump) key(e ohosEvent) {
	name, ok := ohosKeyName(e.code)
	if !ok {
		return
	}
	state := key.Release
	if e.action == 0 {
		state = key.Press
	}
	mods := ohosModifiers(e.mods)
	p.sink.ohosDeliver(key.Event{Name: name, Modifiers: mods, State: state})
	if state == key.Press {
		if text, ok := ohosPrintable(e.code, e.mods); ok {
			p.sink.ohosEditorInsert(text)
		}
	}
}

func (p *ohosPump) clipboard(mime, text string) {
	if mime == "" {
		mime = "application/text"
	}
	content := text
	p.sink.ohosDeliver(transfer.DataEvent{
		Type: mime,
		Open: func() io.ReadCloser {
			return io.NopCloser(strings.NewReader(content))
		},
	})
}

// ApplyOptions records the initial window options without emitting an event.
// newWindow must not deliver anything before the client starts reading.
func (p *ohosPump) ApplyOptions(options []Option) {
	p.config.apply(p.metric, options)
	p.config.Decorated = true
}

// Configure applies window options.  Decorations are always provided by the
// platform, mirroring the Android backend.
func (p *ohosPump) Configure(options []Option) {
	cnf := p.config
	cnf.apply(p.metric, options)
	cnf.Decorated = true
	if cnf.Size == (image.Point{}) {
		cnf.Size = p.config.Size
	}
	p.config = cnf
	p.sink.ohosDeliver(ConfigEvent{Config: p.config})
}
