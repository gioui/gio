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
	anim   bool
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
// (docs/ohos-backend.md).  All methods must be called from the single ohosLoop goroutine.
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

	// activeTouches 是当前按下的触点。axis pinch 只在没有触点时接受
	// （触控板）；单指触摸会错误上报 pinch，双指由原生 pointer 处理。
	activeTouches map[int]bool
	touchSeen     bool
	pinchActive   bool
	lastPinch     float32
}

func newOhosPump(sink ohosSink, metric unit.Metric) *ohosPump {
	if metric.PxPerDp <= 0 {
		metric.PxPerDp = 1
	}
	if metric.PxPerSp <= 0 {
		metric.PxPerSp = metric.PxPerDp
	}
	return &ohosPump{
		sink:          sink,
		metric:        metric,
		life:          ohosForeground,
		config:        Config{Decorated: true, Focused: true},
		activeTouches: make(map[int]bool),
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
		p.SetAnimating(e.anim)
	case ohosEvConfigure:
		p.Configure(e.opts)
	}
}

// surface handles created/changed callbacks.  A zero-sized surface is
// remembered but produces no frame: creating an EGL surface for it would fail
// and spam the log (see docs/ohos-remediation-plan.md, WP2).
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
	p.activeTouches = make(map[int]bool)
	p.pinchActive = false
	p.lastPinch = 0
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
	if p.life != ohosForeground || !p.valid {
		return
	}
	// 每个 vsync 回调都要出帧：静态界面由 touch/axis 等事件请求单帧，
	// 不能因为没有设置 animating 就把这一帧丢掉。
	p.frame(false)
	if p.animating {
		p.sink.ohosRequestFrame()
	}
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
		p.activeTouches = make(map[int]bool)
		p.pinchActive = false
		p.lastPinch = 0
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
	p.touchSeen = true
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
	if p.activeTouches == nil {
		p.activeTouches = make(map[int]bool)
	}
	if kind == pointer.Release || kind == pointer.Cancel {
		delete(p.activeTouches, e.id)
	} else {
		p.activeTouches[e.id] = true
	}
	if len(p.activeTouches) == 0 {
		p.pinchActive = false
		p.lastPinch = 0
	}
	p.sink.ohosDeliver(pointer.Event{
		Kind:      kind,
		Source:    pointer.Touch,
		PointerID: pointer.ID(e.id),
		// Positions arrive in surface pixels.
		Position:  f32.Pt(e.x, e.y),
		Modifiers: ohosModifiers(e.mods),
	})
	// 触摸事件不会自动触发重绘；每个 Move 都必须请求一帧，否则只有第一下
	// 能更新画面（单指只移动一次、捏合只跳一次）。
	p.sink.ohosRequestFrame()
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
	p.sink.ohosRequestFrame()
}

func (p *ohosPump) axis(e ohosEvent) {
	scale := float64(e.scale)
	// 手机触摸会错误上报单指 pinch；只要本窗口见过任何原生触点，
	// 就不再接受 axis pinch。纯触控板窗口从未有 touch 事件，仍可走 axis。
	if p.touchSeen && scale != 0 && scale != 1 {
		scale = 0
	}
	if scale == 1 {
		p.pinchActive = false
		p.lastPinch = 0
		scale = 0
	}
	if scale != 0 {
		if !p.pinchActive {
			p.pinchActive = true
			p.lastPinch = float32(scale)
			scale = 0
		} else {
			prev := float64(p.lastPinch)
			p.lastPinch = float32(scale)
			if prev > 0 {
				scale /= prev
			} else {
				scale = 0
			}
		}
	}
	if !ohosAxisChanged(float64(e.dy), float64(e.dx), scale) {
		return
	}
	scroll, mods := ohosAxisScroll(float64(e.dy), float64(e.dx), scale, e.mods)
	p.sink.ohosDeliver(pointer.Event{
		Kind:      pointer.Scroll,
		Source:    pointer.Mouse,
		PointerID: 1,
		Position:  f32.Pt(e.x, e.y),
		Scroll:    scroll,
		Modifiers: mods,
	})
	p.sink.ohosRequestFrame()
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
