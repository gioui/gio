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
	"gioui.org/io/system"
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
	ohosEvInvalidate
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
	ohosEvAction
	ohosEvWindowMode
	ohosEvOrientation
	ohosEvMetric
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
	// sysAction carries a system.Action for ohosEvAction.
	sysAction system.Action
	// metric carries a runtime density/font-scale change.
	metric unit.Metric
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
	host   ohosHost
	metric unit.Metric

	config     Config
	insets     ohosInsets
	valid      bool
	animating  bool
	life       ohosLifecycle
	surfaceWin uintptr

	// frameRequested is set by requestFrame and cleared by tick. It
	// coalesces any number of redraw requests into one vsync frame.
	frameRequested bool
	// frameSync remembers that the pending frame must refresh the GPU
	// surface (surface change or insets change).
	frameSync bool

	// destroyed is set exactly once by Stop. After it is set the pump
	// ignores all further callbacks.
	destroyed bool

	// activeTouches 是当前按下的触点。axis pinch 只在没有触点时接受
	// （触控板）；单指触摸会错误上报 pinch，双指由原生 pointer 处理。
	activeTouches map[int]bool
	pinchActive   bool
	lastPinch     float32
	lastTouch     time.Time

	mouseButtons pointer.Buttons
	start        time.Time

	lastKeyText string
	lastKeyAt   time.Time
}

func newOhosPump(sink ohosSink, host ohosHost, metric unit.Metric) *ohosPump {
	if metric.PxPerDp <= 0 {
		metric.PxPerDp = 1
	}
	if metric.PxPerSp <= 0 {
		metric.PxPerSp = metric.PxPerDp
	}
	return &ohosPump{
		sink:          sink,
		host:          host,
		metric:        metric,
		life:          ohosForeground,
		config:        Config{Decorated: true, Focused: true},
		activeTouches: make(map[int]bool),
		start:         time.Now(),
	}
}

// Handle processes one platform callback.
func (p *ohosPump) Handle(e ohosEvent) {
	if p.destroyed {
		return
	}
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
			// A hardware key may already have inserted the same text; drop
			// the duplicate if it arrives within the same key frame.
			if e.text == p.lastKeyText && time.Since(p.lastKeyAt) < 50*time.Millisecond {
				break
			}
			p.sink.ohosEditorInsert(e.text)
			p.requestFrame()
		}
	case ohosEvFocus:
		p.setFocused(e.focus)
	case ohosEvClipboard:
		p.clipboard(e.text, e.text2)
	case ohosEvLifecycle:
		p.setLifecycle(e.life)
	case ohosEvFrame:
		p.tick()
	case ohosEvInvalidate:
		p.Invalidate()
	case ohosEvInsets:
		p.setInsets(ohosInsets{Top: e.top, Right: e.right, Bottom: e.bottom, Left: e.left})
	case ohosEvPreedit:
		p.sink.ohosEditorPreedit(e.text, e.a, e.b)
		p.sink.ohosIMEChanged()
		p.requestFrame()
	case ohosEvIMEDelete:
		p.sink.ohosEditorDelete(e.a, e.b)
		p.sink.ohosIMEChanged()
		p.requestFrame()
	case ohosEvIMEMove:
		p.sink.ohosEditorMove(e.a)
		p.sink.ohosIMEChanged()
		p.requestFrame()
	case ohosEvIMESelection:
		p.sink.ohosEditorSelect(e.a, e.b)
		p.sink.ohosIMEChanged()
		p.requestFrame()
	case ohosEvIMEEnter:
		p.sink.ohosEditorEnter(e.a)
		p.sink.ohosIMEChanged()
		p.requestFrame()
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
	case ohosEvAction:
		p.handleAction(e.sysAction)
	case ohosEvWindowMode:
		p.setWindowMode(e.a)
	case ohosEvOrientation:
		p.setOrientation(e.a)
	case ohosEvMetric:
		p.SetMetric(e.metric)
	}
}

// surface handles created/changed callbacks.  A zero-sized surface is
// remembered but produces no frame: creating an EGL surface for it would fail
// and spam the log (see docs/ohos-remediation-plan.md, WP2).
func (p *ohosPump) surface(win uintptr, w, h int, sync bool) {
	if win == 0 || w <= 0 || h <= 0 {
		p.valid = false
		p.config.Size = image.Point{}
		p.frameRequested = false
		p.frameSync = false
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
	p.requestFrameSync(sync)
}

func (p *ohosPump) surfaceDestroyed() {
	p.activeTouches = make(map[int]bool)
	p.pinchActive = false
	p.lastPinch = 0
	p.frameRequested = false
	p.frameSync = false
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

// requestFrame coalesces redraw requests and asks the shim for one vsync.
func (p *ohosPump) requestFrame() {
	p.requestFrameSync(false)
}

func (p *ohosPump) requestFrameSync(sync bool) {
	if sync {
		p.frameSync = true
	}
	if p.destroyed || p.life != ohosForeground || !p.valid || p.config.Size == (image.Point{}) {
		return
	}
	if p.frameRequested {
		return
	}
	p.frameRequested = true
	p.sink.ohosRequestFrame()
}

// Invalidate requests a frame. Background windows produce none.
func (p *ohosPump) Invalidate() {
	p.requestFrame()
}

// SetAnimating starts/stops the vsync loop.
func (p *ohosPump) SetAnimating(anim bool) {
	if p.destroyed || p.animating == anim {
		return
	}
	p.animating = anim
	if p.host != nil {
		p.host.HostSetAnimating(anim)
	}
	if anim {
		p.requestFrame()
	}
}

// tick is a vsync callback from the shim.
func (p *ohosPump) tick() {
	if p.destroyed || p.life != ohosForeground || !p.valid {
		return
	}
	if !p.frameRequested && !p.animating {
		return
	}
	sync := p.frameSync
	p.frameRequested = false
	p.frameSync = false
	p.frame(sync)
	if p.animating {
		p.requestFrame()
	}
}

func (p *ohosPump) frame(sync bool) {
	if p.destroyed || !p.valid || p.config.Size.X <= 0 || p.config.Size.Y <= 0 {
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
	if !focused {
		p.mouseButtons = 0
		p.activeTouches = make(map[int]bool)
	}
	p.sink.ohosDeliver(ConfigEvent{Config: p.config})
	p.requestFrameSync(true)
}

func (p *ohosPump) setInsets(in ohosInsets) {
	if p.insets == in {
		return
	}
	p.insets = in
	// Re-layout so insets (and therefore RevealFocus) take effect.
	p.requestFrameSync(true)
}

func (p *ohosPump) setLifecycle(l ohosLifecycle) {
	switch l {
	case ohosBackground:
		p.life = ohosBackground
		p.activeTouches = make(map[int]bool)
		p.pinchActive = false
		p.lastPinch = 0
		p.lastTouch = time.Time{}
		p.mouseButtons = 0
		p.frameRequested = false
		p.frameSync = false
	case ohosForeground:
		p.life = ohosForeground
		if p.valid {
			p.requestFrameSync(true)
		}
	case ohosDestroyed:
		p.life = ohosDestroyed
		p.Stop()
	}
}

// Stop delivers DestroyEvent once and makes the pump inert. It is called
// from the loop goroutine.
func (p *ohosPump) Stop() {
	if p.destroyed {
		return
	}
	p.destroyed = true
	p.frameRequested = false
	p.frameSync = false
	p.sink.ohosDeliver(DestroyEvent{})
}

// touch translates a pointer callback.  action: 0 down, 1 move, 2 up, 3 cancel.
func (p *ohosPump) touch(e ohosEvent) {
	p.lastTouch = time.Now()
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
		Time:      time.Since(p.start),
	})
	p.requestFrame()
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
	switch kind {
	case pointer.Press:
		p.mouseButtons |= btns
	case pointer.Release:
		p.mouseButtons &^= btns
	}
	p.sink.ohosDeliver(pointer.Event{
		Kind:      kind,
		Source:    pointer.Mouse,
		Buttons:   p.mouseButtons,
		PointerID: 1,
		Position:  f32.Pt(e.x, e.y),
		Modifiers: ohosModifiers(e.mods),
		Time:      time.Since(p.start),
	})
	p.requestFrame()
}

func (p *ohosPump) axis(e ohosEvent) {
	scale := float64(e.scale)
	// 触摸设备会在触摸后补发单指 pinch。只要还有触点，或最近 150ms
	// 内出现过触摸，就忽略 axis pinch；纯触控板窗口没有触摸历史。
	if len(p.activeTouches) > 0 || (!p.lastTouch.IsZero() && time.Since(p.lastTouch) < 150*time.Millisecond) {
		if scale != 0 && scale != 1 {
			scale = 0
		}
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
	p.requestFrame()
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
			p.lastKeyText, p.lastKeyAt = text, time.Now()
			p.sink.ohosEditorInsert(text)
		}
	}
	p.requestFrame()
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
	old := p.config
	p.config.apply(p.metric, options)
	p.config.Decorated = true
	p.config.Size = old.Size
	if p.host != nil {
		p.host.Apply(planOHOSConfig(old, p.config))
	}
}

// Configure applies window options.  Decorations are always provided by the
// platform, mirroring the Android backend.
func (p *ohosPump) Configure(options []Option) {
	cnf := p.config
	cnf.apply(p.metric, options)
	cnf.Decorated = true
	// Size is owned by the surface: a mobile window cannot be resized by
	// the application, and letting an Option change it makes EGL use a
	// geometry different from the XComponent.
	cnf.Size = p.config.Size
	old := p.config
	p.config = cnf
	p.applyConfigDelta(planOHOSConfig(old, cnf))
}

// applyConfigDelta pushes a delta to the platform, reports the effective
// config and schedules a frame.
func (p *ohosPump) applyConfigDelta(d ohosConfigDelta) {
	if p.host != nil {
		p.host.Apply(d)
	}
	p.sink.ohosDeliver(ConfigEvent{Config: p.config})
	p.requestFrame()
}

func (p *ohosPump) handleAction(a system.Action) {
	if a&system.ActionClose != 0 {
		if p.host == nil || !p.host.RequestClose() {
			p.Stop()
		}
		return
	}
	var d ohosConfigDelta
	switch {
	case a&system.ActionMinimize != 0:
		d.ModeSet, d.Mode = true, Minimized
	case a&system.ActionMaximize != 0:
		d.ModeSet, d.Mode = true, Maximized
	case a&system.ActionFullscreen != 0:
		d.ModeSet, d.Mode = true, Fullscreen
	case a&system.ActionUnmaximize != 0:
		d.ModeSet, d.Mode = true, Windowed
	}
	if d.ModeSet {
		p.config.Mode = d.Mode
		p.applyConfigDelta(d)
	}
	if p.host != nil {
		if a&system.ActionRaise != 0 {
			p.host.RaiseWindow()
		}
		if a&system.ActionCenter != 0 {
			p.host.CenterWindow()
		}
		if a&system.ActionMove != 0 {
			p.host.StartWindowMove()
		}
	}
}

func (p *ohosPump) setWindowMode(mode int) {
	var m WindowMode
	switch mode {
	case 0:
		m = Windowed
	case 1:
		m = Fullscreen
	case 2:
		m = Minimized
	case 3:
		m = Maximized
	default:
		return
	}
	if p.config.Mode == m {
		return
	}
	p.config.Mode = m
	p.applyConfigDelta(ohosConfigDelta{ModeSet: true, Mode: m})
}

// SetMetric applies a runtime density/font-scale change and relayouts.
func (p *ohosPump) SetMetric(m unit.Metric) {
	if p.destroyed || m == p.metric {
		return
	}
	if m.PxPerDp <= 0 {
		m.PxPerDp = 1
	}
	if m.PxPerSp <= 0 {
		m.PxPerSp = m.PxPerDp
	}
	p.metric = m
	p.requestFrameSync(true)
}

func (p *ohosPump) setOrientation(o int) {
	var orientation Orientation
	switch o {
	case 0:
		orientation = AnyOrientation
	case 1:
		orientation = LandscapeOrientation
	case 2:
		orientation = PortraitOrientation
	default:
		return
	}
	if p.config.Orientation == orientation {
		return
	}
	p.config.Orientation = orientation
	p.applyConfigDelta(ohosConfigDelta{OrientationSet: true, Orientation: orientation})
}
