// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"sync"
	"syscall/js"
	"time"

	"gioui.org/f32"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
)

type window struct {
	window                js.Value
	cnv                   js.Value
	tarea                 js.Value
	w                     *Window
	redraw                js.Func
	requestAnimationFrame js.Value
	cleanfuncs            []func()
	touches               []js.Value
	composing             bool

	mu        sync.Mutex
	scale     float32
	animating bool
}

var mainDone = make(chan struct{})

func createWindow(win *Window, opts *windowOptions) error {
	doc := js.Global().Get("document")
	cont := getContainer(doc)
	cnv := createCanvas(doc)
	cont.Call("appendChild", cnv)
	tarea := createTextArea(doc)
	cont.Call("appendChild", tarea)
	w := &window{
		cnv:    cnv,
		tarea:  tarea,
		window: js.Global().Get("window"),
	}
	w.requestAnimationFrame = w.window.Get("requestAnimationFrame")
	w.redraw = w.funcOf(func(this js.Value, args []js.Value) interface{} {
		w.animCallback()
		return nil
	})
	w.addEventListeners()
	w.w = win
	go func() {
		w.w.setDriver(w)
		w.focus()
		w.w.event(StageEvent{StageRunning})
		w.draw(true)
		select {}
		w.cleanup()
		close(mainDone)
	}()
	return nil
}

func getContainer(doc js.Value) js.Value {
	cont := doc.Call("getElementById", "giowindow")
	if cont != js.Null() {
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
	w.addEventListener(w.window, "resize", func(this js.Value, args []js.Value) interface{} {
		w.draw(true)
		return nil
	})
	w.addEventListener(w.cnv, "mousemove", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Move, 0, 0, args[0])
		return nil
	})
	w.addEventListener(w.cnv, "mousedown", func(this js.Value, args []js.Value) interface{} {
		w.pointerEvent(pointer.Press, 0, 0, args[0])
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
		w.pointerEvent(pointer.Move, float32(dx), float32(dy), e)
		return nil
	})
	w.addEventListener(w.cnv, "touchstart", func(this js.Value, args []js.Value) interface{} {
		w.touchEvent(pointer.Press, args[0])
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
		w.w.event(pointer.Event{
			Type:   pointer.Cancel,
			Source: pointer.Touch,
		})
		return nil
	})
	w.addEventListener(w.tarea, "focus", func(this js.Value, args []js.Value) interface{} {
		w.w.event(key.FocusEvent{Focus: true})
		return nil
	})
	w.addEventListener(w.tarea, "blur", func(this js.Value, args []js.Value) interface{} {
		w.w.event(key.FocusEvent{Focus: false})
		return nil
	})
	w.addEventListener(w.tarea, "keydown", func(this js.Value, args []js.Value) interface{} {
		w.keyEvent(args[0])
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
}

func (w *window) flushInput() {
	val := w.tarea.Get("value").String()
	w.tarea.Set("value", "")
	w.w.event(key.EditEvent{Text: string(val)})
}

func (w *window) blur() {
	w.tarea.Call("blur")
}

func (w *window) focus() {
	w.tarea.Call("focus")
}

func (w *window) keyEvent(e js.Value) {
	k := e.Get("key").String()
	if n, ok := translateKey(k); ok {
		cmd := key.Event{Name: n}
		if e.Call("getModifierState", "Control").Bool() {
			cmd.Modifiers |= key.ModCommand
		}
		if e.Call("getModifierState", "Shift").Bool() {
			cmd.Modifiers |= key.ModShift
		}
		w.w.event(cmd)
	}
}

func (w *window) touchEvent(typ pointer.Type, e js.Value) {
	e.Call("preventDefault")
	t := time.Duration(e.Get("timeStamp").Int()) * time.Millisecond
	changedTouches := e.Get("changedTouches")
	n := changedTouches.Length()
	rect := w.cnv.Call("getBoundingClientRect")
	w.mu.Lock()
	scale := w.scale
	w.mu.Unlock()
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
		w.w.event(pointer.Event{
			Type:      typ,
			Source:    pointer.Touch,
			Position:  pos,
			PointerID: pid,
			Time:      t,
		})
	}
}

func (w *window) touchIDFor(touch js.Value) pointer.ID {
	id := touch.Get("identifier")
	for i, id2 := range w.touches {
		if id2 == id {
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
	w.mu.Lock()
	scale := w.scale
	w.mu.Unlock()
	pos := f32.Point{
		X: float32(x) * scale,
		Y: float32(y) * scale,
	}
	scroll := f32.Point{
		X: dx * scale,
		Y: dy * scale,
	}
	t := time.Duration(e.Get("timeStamp").Int()) * time.Millisecond
	w.w.event(pointer.Event{
		Type:     typ,
		Source:   pointer.Mouse,
		Position: pos,
		Scroll:   scroll,
		Time:     t,
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
// functions to be released up.
func (w *window) funcOf(f func(this js.Value, args []js.Value) interface{}) js.Func {
	jsf := js.FuncOf(f)
	w.cleanfuncs = append(w.cleanfuncs, jsf.Release)
	return jsf
}

func (w *window) animCallback() {
	w.mu.Lock()
	anim := w.animating
	if anim {
		w.requestAnimationFrame.Invoke(w.redraw)
	}
	w.mu.Unlock()
	if anim {
		w.draw(false)
	}
}

func (w *window) setAnimating(anim bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if anim && !w.animating {
		w.requestAnimationFrame.Invoke(w.redraw)
	}
	w.animating = anim
}

func (w *window) showTextInput(show bool) {
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

func (w *window) draw(sync bool) {
	width, height, scale, cfg := w.config()
	if cfg == (Config{}) {
		return
	}
	w.mu.Lock()
	w.scale = float32(scale)
	w.mu.Unlock()
	cfg.now = time.Now()
	w.w.event(UpdateEvent{
		Size: image.Point{
			X: width,
			Y: height,
		},
		Config: cfg,
		sync:   sync,
	})
}

func (w *window) config() (int, int, float32, Config) {
	rect := w.cnv.Call("getBoundingClientRect")
	width, height := rect.Get("width").Float(), rect.Get("height").Float()
	scale := w.window.Get("devicePixelRatio").Float()
	width *= scale
	height *= scale
	iw, ih := int(width+.5), int(height+.5)
	// Adjust internal size of canvas if necessary.
	if cw, ch := w.cnv.Get("width").Int(), w.cnv.Get("height").Int(); iw != cw || ih != ch {
		w.cnv.Set("width", iw)
		w.cnv.Set("height", ih)
	}
	const ppdp = 96 * inchPrDp * monitorScale
	return iw, ih, float32(scale), Config{
		pxPerDp: ppdp * float32(scale),
		pxPerSp: ppdp * float32(scale),
	}
}

func main() {
	<-mainDone
}

func translateKey(k string) (rune, bool) {
	if len(k) == 1 {
		c := k[0]
		if '0' <= c && c <= '9' || 'A' <= c && c <= 'Z' {
			return rune(c), true
		}
		if 'a' <= c && c <= 'z' {
			return rune(c - 0x20), true
		}
	}
	var n rune
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
	default:
		return 0, false
	}
	return n, true
}
