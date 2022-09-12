// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"syscall/js"
	"time"
	"unicode"
	"unicode/utf8"

	"gioui.org/internal/f32color"

	"gioui.org/f32"
	"gioui.org/io/clipboard"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/unit"
)

type ViewEvent struct{}

type contextStatus int

const (
	contextStatusOkay contextStatus = iota
	contextStatusLost
	contextStatusRestored
)

type window struct {
	window                js.Value
	document              js.Value
	head                  js.Value
	clipboard             js.Value
	cnv                   js.Value
	tarea                 js.Value
	w                     *callbacks
	redraw                js.Func
	clipboardCallback     js.Func
	requestAnimationFrame js.Value
	browserHistory        js.Value
	visualViewport        js.Value
	screenOrientation     js.Value
	cleanfuncs            []func()
	touches               []js.Value
	composing             bool
	requestFocus          bool

	chanAnimation chan struct{}
	chanRedraw    chan struct{}

	config    Config
	inset     f32.Point
	scale     float32
	animating bool
	// animRequested tracks whether a requestAnimationFrame callback
	// is pending.
	animRequested bool
	wakeups       chan struct{}

	contextStatus contextStatus
}

func newWindow(win *callbacks, options []Option) error {
	doc := js.Global().Get("document")
	cont := getContainer(doc)
	cnv := createCanvas(doc)
	cont.Call("appendChild", cnv)
	tarea := createTextArea(doc)
	cont.Call("appendChild", tarea)
	w := &window{
		cnv:       cnv,
		document:  doc,
		tarea:     tarea,
		window:    js.Global().Get("window"),
		head:      doc.Get("head"),
		clipboard: js.Global().Get("navigator").Get("clipboard"),
		wakeups:   make(chan struct{}, 1),
	}
	w.requestAnimationFrame = w.window.Get("requestAnimationFrame")
	w.browserHistory = w.window.Get("history")
	w.visualViewport = w.window.Get("visualViewport")
	if w.visualViewport.IsUndefined() {
		w.visualViewport = w.window
	}
	if screen := w.window.Get("screen"); screen.Truthy() {
		w.screenOrientation = screen.Get("orientation")
	}
	w.chanAnimation = make(chan struct{}, 1)
	w.chanRedraw = make(chan struct{}, 1)
	w.redraw = w.funcOf(func(this js.Value, args []js.Value) interface{} {
		w.chanAnimation <- struct{}{}
		return nil
	})
	w.clipboardCallback = w.funcOf(func(this js.Value, args []js.Value) interface{} {
		content := args[0].String()
		go win.Event(clipboard.Event{Text: content})
		return nil
	})
	w.addEventListeners()
	w.addHistory()
	w.w = win

	go func() {
		defer w.cleanup()
		w.w.SetDriver(w)
		w.Configure(options)
		w.blur()
		w.w.Event(system.StageEvent{Stage: system.StageRunning})
		w.resize()
		w.draw(true)
		for {
			select {
			case <-w.wakeups:
				w.w.Event(wakeupEvent{})
			case <-w.chanAnimation:
				w.animCallback()
			case <-w.chanRedraw:
				w.draw(true)
			}
		}
	}()
	return nil
}

func getContainer(doc js.Value) js.Value {
	cont := doc.Call("getElementById", "giowindow")
	if !cont.IsNull() {
		return cont
	}
	cont = doc.Call("createElement", "DIV")
	doc.Get("body").Call("appendChild", cont)
	return cont
}

func createTextArea(doc js.Value) js.Value {
	tarea := doc.Call("createElement", "input")
	style := tarea.Get("style")
	style.Set("width", "1px")
	style.Set("height", "1px")
	style.Set("opacity", "0")
	style.Set("border", "0")
	style.Set("padding", "0")
	tarea.Set("autocomplete", "off")
	tarea.Set("autocorrect", "off")
	tarea.Set("autocapitalize", "off")
	tarea.Set("spellcheck", false)
	return tarea
}

func createCanvas(doc js.Value) js.Value {
	cnv := doc.Call("createElement", "canvas")
	style := cnv.Get("style")
	style.Set("position", "fixed")
	style.Set("width", "100%")
	style.Set("height", "100%")
	return cnv
}

func (w *window) cleanup() {
	// Cleanup in the opposite order of
	// construction.
	for i := len(w.cleanfuncs) - 1; i >= 0; i-- {
		w.cleanfuncs[i]()
	}
	w.cleanfuncs = nil
}

func (w *window) addEventListeners() {
	w.addEventListener(w.cnv, "webglcontextlost", func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		w.contextStatus = contextStatusLost
		return nil
	})
	w.addEventListener(w.cnv, "webglcontextrestored", func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		w.contextStatus = contextStatusRestored

		// Resize is required to force update the canvas content when restored.
		w.cnv.Set("width", 0)
		w.cnv.Set("height", 0)
		w.resize()
		w.requestRedraw()
		return nil
	})
	w.addEventListener(w.visualViewport, "resize", func(this js.Value, args []js.Value) interface{} {
		w.resize()
		w.requestRedraw()
		return nil
	})
	w.addEventListener(w.window, "contextmenu", func(this js.Value, args []js.Value) interface{} {
		args[0].Call("preventDefault")
		return nil
	})
	w.addEventListener(w.window, "popstate", func(this js.Value, args []js.Value) interface{} {
		if w.w.Event(key.Event{Name: key.NameBack}) {
			return w.browserHistory.Call("forward")
		}
		return w.browserHistory.Call("back")
	})
	w.addEventListener(w.document, "visibilitychange", func(this js.Value, args []js.Value) interface{} {
		ev := system.StageEvent{}
		switch w.document.Get("visibilityState").String() {
		case "hidden", "prerender", "unloaded":
			ev.Stage = system.StagePaused
		default:
			ev.Stage = system.StageRunning
		}
		w.w.Event(ev)
		return nil
	})
	w.addEventListener(w.cnv, "mousemove", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Move, 0, 0, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "mousedown", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Press, 0, 0, args[0])
		if w.requestFocus {
			w.focus()
			w.requestFocus = false
		}
		return nil
	})
	w.addEventListener(w.cnv, "mouseup", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Release, 0, 0, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "wheel", func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		dx, dy := e.Get("deltaX").Float(), e.Get("deltaY").Float()
		mode := e.Get("deltaMode").Int()
		switch mode {
		case 0x01: // DOM_DELTA_LINE
			dx *= 10
			dy *= 10
		case 0x02: // DOM_DELTA_PAGE
			dx *= 120
			dy *= 120
		}
		w.pointerEvent(pointer.Scroll, float32(dx), float32(dy), e)
		return nil
	})
	w.addEventListener(w.cnv, "touchstart", func(this js.Value, args []js.Value) interface{} {
		w.touchEvent(pointer.Press, args[0])
		if w.requestFocus {
			w.focus() // iOS can only focus inside a Touch event.
			w.requestFocus = false
		}
		return nil
	})
	w.addEventListener(w.cnv, "touchend", func(this js.Value, args []js.Value) interface{} {
		w.touchEvent(pointer.Release, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "touchmove", func(this js.Value, args []js.Value) interface{} {
		w.touchEvent(pointer.Move, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "touchcancel", func(this js.Value, args []js.Value) interface{} {
		// Cancel all touches even if only one touch was cancelled.
		for i := range w.touches {
			w.touches[i] = js.Null()
		}
		w.touches = w.touches[:0]
		w.w.Event(pointer.Event{
			Type:   pointer.Cancel,
			Source: pointer.Touch,
		})
		return nil
	})
	w.addEventListener(w.tarea, "focus", func(this js.Value, args []js.Value) interface{} {
		w.w.Event(key.FocusEvent{Focus: true})
		return nil
	})
	w.addEventListener(w.tarea, "blur", func(this js.Value, args []js.Value) interface{} {
		w.w.Event(key.FocusEvent{Focus: false})
		w.blur()
		return nil
	})
	w.addEventListener(w.tarea, "keydown", func(this js.Value, args []js.Value) interface{} {
		w.keyEvent(args[0], key.Press)
		return nil
	})
	w.addEventListener(w.tarea, "keyup", func(this js.Value, args []js.Value) interface{} {
		w.keyEvent(args[0], key.Release)
		return nil
	})
	w.addEventListener(w.tarea, "compositionstart", func(this js.Value, args []js.Value) interface{} {
		w.composing = true
		return nil
	})
	w.addEventListener(w.tarea, "compositionend", func(this js.Value, args []js.Value) interface{} {
		w.composing = false
		w.flushInput()
		return nil
	})
	w.addEventListener(w.tarea, "input", func(this js.Value, args []js.Value) interface{} {
		if w.composing {
			return nil
		}
		w.flushInput()
		return nil
	})
	w.addEventListener(w.tarea, "paste", func(this js.Value, args []js.Value) interface{} {
		if w.clipboard.IsUndefined() {
			return nil
		}
		// Prevents duplicated-paste, since "paste" is already handled through Clipboard API.
		args[0].Call("preventDefault")
		return nil
	})
}

func (w *window) addHistory() {
	w.browserHistory.Call("pushState", nil, nil, w.window.Get("location").Get("href"))
}

func (w *window) flushInput() {
	val := w.tarea.Get("value").String()
	w.tarea.Set("value", "")
	w.w.EditorInsert(string(val))
}

func (w *window) blur() {
	w.tarea.Call("blur")
	w.requestFocus = false
}

func (w *window) focus() {
	w.tarea.Call("focus")
	w.requestFocus = true
}

func (w *window) keyboard(hint key.InputHint) {
	var m string
	switch hint {
	case key.HintAny:
		m = "text"
	case key.HintText:
		m = "text"
	case key.HintNumeric:
		m = "decimal"
	case key.HintEmail:
		m = "email"
	case key.HintURL:
		m = "url"
	case key.HintTelephone:
		m = "tel"
	default:
		m = "text"
	}
	w.tarea.Set("inputMode", m)
}

func (w *window) keyEvent(e js.Value, ks key.State) {
	k := e.Get("key").String()
	if n, ok := translateKey(k); ok {
		cmd := key.Event{
			Name:      n,
			Modifiers: modifiersFor(e),
			State:     ks,
		}
		w.w.Event(cmd)
	}
}

// modifiersFor returns the modifier set for a DOM MouseEvent or
// KeyEvent.
func modifiersFor(e js.Value) key.Modifiers {
	var mods key.Modifiers
	if e.Get("getModifierState").IsUndefined() {
		// Some browsers doesn't support getModifierState.
		return mods
	}
	if e.Call("getModifierState", "Alt").Bool() {
		mods |= key.ModAlt
	}
	if e.Call("getModifierState", "Control").Bool() {
		mods |= key.ModCtrl
	}
	if e.Call("getModifierState", "Shift").Bool() {
		mods |= key.ModShift
	}
	return mods
}

func (w *window) touchEvent(typ pointer.Type, e js.Value) {
	e.Call("preventDefault")
	t := time.Duration(e.Get("timeStamp").Int()) * time.Millisecond
	changedTouches := e.Get("changedTouches")
	n := changedTouches.Length()
	rect := w.cnv.Call("getBoundingClientRect")
	scale := w.scale
	var mods key.Modifiers
	if e.Get("shiftKey").Bool() {
		mods |= key.ModShift
	}
	if e.Get("altKey").Bool() {
		mods |= key.ModAlt
	}
	if e.Get("ctrlKey").Bool() {
		mods |= key.ModCtrl
	}
	for i := 0; i < n; i++ {
		touch := changedTouches.Index(i)
		pid := w.touchIDFor(touch)
		x, y := touch.Get("clientX").Float(), touch.Get("clientY").Float()
		x -= rect.Get("left").Float()
		y -= rect.Get("top").Float()
		pos := f32.Point{
			X: float32(x) * scale,
			Y: float32(y) * scale,
		}
		w.w.Event(pointer.Event{
			Type:      typ,
			Source:    pointer.Touch,
			Position:  pos,
			PointerID: pid,
			Time:      t,
			Modifiers: mods,
		})
	}
}

func (w *window) touchIDFor(touch js.Value) pointer.ID {
	id := touch.Get("identifier")
	for i, id2 := range w.touches {
		if id2.Equal(id) {
			return pointer.ID(i)
		}
	}
	pid := pointer.ID(len(w.touches))
	w.touches = append(w.touches, id)
	return pid
}

func (w *window) pointerEvent(typ pointer.Type, dx, dy float32, e js.Value) {
	e.Call("preventDefault")
	x, y := e.Get("clientX").Float(), e.Get("clientY").Float()
	rect := w.cnv.Call("getBoundingClientRect")
	x -= rect.Get("left").Float()
	y -= rect.Get("top").Float()
	scale := w.scale
	pos := f32.Point{
		X: float32(x) * scale,
		Y: float32(y) * scale,
	}
	scroll := f32.Point{
		X: dx * scale,
		Y: dy * scale,
	}
	t := time.Duration(e.Get("timeStamp").Int()) * time.Millisecond
	jbtns := e.Get("buttons").Int()
	var btns pointer.Buttons
	if jbtns&1 != 0 {
		btns |= pointer.ButtonPrimary
	}
	if jbtns&2 != 0 {
		btns |= pointer.ButtonSecondary
	}
	if jbtns&4 != 0 {
		btns |= pointer.ButtonTertiary
	}
	w.w.Event(pointer.Event{
		Type:      typ,
		Source:    pointer.Mouse,
		Buttons:   btns,
		Position:  pos,
		Scroll:    scroll,
		Time:      t,
		Modifiers: modifiersFor(e),
	})
}

func (w *window) addEventListener(this js.Value, event string, f func(this js.Value, args []js.Value) interface{}) {
	jsf := w.funcOf(f)
	this.Call("addEventListener", event, jsf)
	w.cleanfuncs = append(w.cleanfuncs, func() {
		this.Call("removeEventListener", event, jsf)
	})
}

// funcOf is like js.FuncOf but adds the js.Func to a list of
// functions to be released during cleanup.
func (w *window) funcOf(f func(this js.Value, args []js.Value) interface{}) js.Func {
	jsf := js.FuncOf(f)
	w.cleanfuncs = append(w.cleanfuncs, jsf.Release)
	return jsf
}

func (w *window) animCallback() {
	anim := w.animating
	w.animRequested = anim
	if anim {
		w.requestAnimationFrame.Invoke(w.redraw)
	}
	if anim {
		w.draw(false)
	}
}

func (w *window) EditorStateChanged(old, new editorState) {}

func (w *window) SetAnimating(anim bool) {
	w.animating = anim
	if anim && !w.animRequested {
		w.animRequested = true
		w.requestAnimationFrame.Invoke(w.redraw)
	}
}

func (w *window) ReadClipboard() {
	if w.clipboard.IsUndefined() {
		return
	}
	if w.clipboard.Get("readText").IsUndefined() {
		return
	}
	w.clipboard.Call("readText", w.clipboard).Call("then", w.clipboardCallback)
}

func (w *window) WriteClipboard(s string) {
	if w.clipboard.IsUndefined() {
		return
	}
	if w.clipboard.Get("writeText").IsUndefined() {
		return
	}
	w.clipboard.Call("writeText", s)
}

func (w *window) Configure(options []Option) {
	prev := w.config
	cnf := w.config
	cnf.apply(unit.Metric{}, options)
	// Decorations are never disabled.
	cnf.Decorated = true

	if prev.Title != cnf.Title {
		w.config.Title = cnf.Title
		w.document.Set("title", cnf.Title)
	}
	if prev.Mode != cnf.Mode {
		w.windowMode(cnf.Mode)
	}
	if prev.NavigationColor != cnf.NavigationColor {
		w.config.NavigationColor = cnf.NavigationColor
		w.navigationColor(cnf.NavigationColor)
	}
	if prev.Orientation != cnf.Orientation {
		w.config.Orientation = cnf.Orientation
		w.orientation(cnf.Orientation)
	}
	if cnf.Decorated != prev.Decorated {
		w.config.Decorated = cnf.Decorated
	}
	w.w.Event(ConfigEvent{Config: w.config})
}

func (w *window) Perform(system.Action) {}

var webCursor = [...]string{
	pointer.CursorDefault:                  "default",
	pointer.CursorNone:                     "none",
	pointer.CursorText:                     "text",
	pointer.CursorVerticalText:             "vertical-text",
	pointer.CursorPointer:                  "pointer",
	pointer.CursorCrosshair:                "crosshair",
	pointer.CursorAllScroll:                "all-scroll",
	pointer.CursorColResize:                "col-resize",
	pointer.CursorRowResize:                "row-resize",
	pointer.CursorGrab:                     "grab",
	pointer.CursorGrabbing:                 "grabbing",
	pointer.CursorNotAllowed:               "not-allowed",
	pointer.CursorWait:                     "wait",
	pointer.CursorProgress:                 "progress",
	pointer.CursorNorthWestResize:          "nw-resize",
	pointer.CursorNorthEastResize:          "ne-resize",
	pointer.CursorSouthWestResize:          "sw-resize",
	pointer.CursorSouthEastResize:          "se-resize",
	pointer.CursorNorthSouthResize:         "ns-resize",
	pointer.CursorEastWestResize:           "ew-resize",
	pointer.CursorWestResize:               "w-resize",
	pointer.CursorEastResize:               "e-resize",
	pointer.CursorNorthResize:              "n-resize",
	pointer.CursorSouthResize:              "s-resize",
	pointer.CursorNorthEastSouthWestResize: "nesw-resize",
	pointer.CursorNorthWestSouthEastResize: "nwse-resize",
}

func (w *window) SetCursor(cursor pointer.Cursor) {
	style := w.cnv.Get("style")
	style.Set("cursor", webCursor[cursor])
}

func (w *window) Wakeup() {
	select {
	case w.wakeups <- struct{}{}:
	default:
	}
}

func (w *window) ShowTextInput(show bool) {
	// Run in a goroutine to avoid a deadlock if the
	// focus change result in an event.
	go func() {
		if show {
			w.focus()
		} else {
			w.blur()
		}
	}()
}

func (w *window) SetInputHint(mode key.InputHint) {
	w.keyboard(mode)
}

func (w *window) resize() {
	w.scale = float32(w.window.Get("devicePixelRatio").Float())

	rect := w.cnv.Call("getBoundingClientRect")
	size := image.Point{
		X: int(float32(rect.Get("width").Float()) * w.scale),
		Y: int(float32(rect.Get("height").Float()) * w.scale),
	}
	if size != w.config.Size {
		w.config.Size = size
		w.w.Event(ConfigEvent{Config: w.config})
	}

	if vx, vy := w.visualViewport.Get("width"), w.visualViewport.Get("height"); !vx.IsUndefined() && !vy.IsUndefined() {
		w.inset.X = float32(w.config.Size.X) - float32(vx.Float())*w.scale
		w.inset.Y = float32(w.config.Size.Y) - float32(vy.Float())*w.scale
	}

	if w.config.Size.X == 0 || w.config.Size.Y == 0 {
		return
	}

	w.cnv.Set("width", w.config.Size.X)
	w.cnv.Set("height", w.config.Size.Y)
}

func (w *window) draw(sync bool) {
	if w.contextStatus == contextStatusLost {
		return
	}
	size, insets, metric := w.getConfig()
	if metric == (unit.Metric{}) || size.X == 0 || size.Y == 0 {
		return
	}

	w.w.Event(frameEvent{
		FrameEvent: system.FrameEvent{
			Now:    time.Now(),
			Size:   size,
			Insets: insets,
			Metric: metric,
		},
		Sync: sync,
	})
}

func (w *window) getConfig() (image.Point, system.Insets, unit.Metric) {
	invscale := unit.Dp(1. / w.scale)
	return image.Pt(w.config.Size.X, w.config.Size.Y),
		system.Insets{
			Bottom: unit.Dp(w.inset.Y) * invscale,
			Right:  unit.Dp(w.inset.X) * invscale,
		}, unit.Metric{
			PxPerDp: w.scale,
			PxPerSp: w.scale,
		}
}

func (w *window) windowMode(mode WindowMode) {
	switch mode {
	case Windowed:
		if !w.document.Get("fullscreenElement").Truthy() {
			return // Browser is already Windowed.
		}
		if !w.document.Get("exitFullscreen").Truthy() {
			return // Browser doesn't support such feature.
		}
		w.document.Call("exitFullscreen")
		w.config.Mode = Windowed
	case Fullscreen:
		elem := w.document.Get("documentElement")
		if !elem.Get("requestFullscreen").Truthy() {
			return // Browser doesn't support such feature.
		}
		elem.Call("requestFullscreen")
		w.config.Mode = Fullscreen
	}
}

func (w *window) orientation(mode Orientation) {
	if j := w.screenOrientation; !j.Truthy() || !j.Get("unlock").Truthy() || !j.Get("lock").Truthy() {
		return // Browser don't support Screen Orientation API.
	}

	switch mode {
	case AnyOrientation:
		w.screenOrientation.Call("unlock")
	case LandscapeOrientation:
		w.screenOrientation.Call("lock", "landscape").Call("then", w.redraw)
	case PortraitOrientation:
		w.screenOrientation.Call("lock", "portrait").Call("then", w.redraw)
	}
}

func (w *window) navigationColor(c color.NRGBA) {
	theme := w.head.Call("querySelector", `meta[name="theme-color"]`)
	if !theme.Truthy() {
		theme = w.document.Call("createElement", "meta")
		theme.Set("name", "theme-color")
		w.head.Call("appendChild", theme)
	}
	rgba := f32color.NRGBAToRGBA(c)
	theme.Set("content", fmt.Sprintf("#%06X", []uint8{rgba.R, rgba.G, rgba.B}))
}

func (w *window) requestRedraw() {
	select {
	case w.chanRedraw <- struct{}{}:
	default:
	}
}

func osMain() {
	select {}
}

func translateKey(k string) (string, bool) {
	var n string

	switch k {
	case "ArrowUp":
		n = key.NameUpArrow
	case "ArrowDown":
		n = key.NameDownArrow
	case "ArrowLeft":
		n = key.NameLeftArrow
	case "ArrowRight":
		n = key.NameRightArrow
	case "Escape":
		n = key.NameEscape
	case "Enter":
		n = key.NameReturn
	case "Backspace":
		n = key.NameDeleteBackward
	case "Delete":
		n = key.NameDeleteForward
	case "Home":
		n = key.NameHome
	case "End":
		n = key.NameEnd
	case "PageUp":
		n = key.NamePageUp
	case "PageDown":
		n = key.NamePageDown
	case "Tab":
		n = key.NameTab
	case " ":
		n = key.NameSpace
	case "F1":
		n = key.NameF1
	case "F2":
		n = key.NameF2
	case "F3":
		n = key.NameF3
	case "F4":
		n = key.NameF4
	case "F5":
		n = key.NameF5
	case "F6":
		n = key.NameF6
	case "F7":
		n = key.NameF7
	case "F8":
		n = key.NameF8
	case "F9":
		n = key.NameF9
	case "F10":
		n = key.NameF10
	case "F11":
		n = key.NameF11
	case "F12":
		n = key.NameF12
	case "Control":
		n = key.NameCtrl
	case "Shift":
		n = key.NameShift
	case "Alt":
		n = key.NameAlt
	case "OS":
		n = key.NameSuper
	default:
		r, s := utf8.DecodeRuneInString(k)
		// If there is exactly one printable character, return that.
		if s == len(k) && unicode.IsPrint(r) {
			return strings.ToUpper(k), true
		}
		return "", false
	}
	return n, true
}

func (_ ViewEvent) ImplementsEvent() {}
