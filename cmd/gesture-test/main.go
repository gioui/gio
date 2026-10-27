// SPDX-License-Identifier: Unlicense OR MIT

// Command gesture-test is an interactive diagnostic window for Gio pointer
// gestures. It shows native semantic events (Pinch/Rotate/Swipe/Hold/Scroll)
// and their parameters, so trackpad and touchscreen behavior can be checked
// by hand.
package main

import (
	"fmt"
	"image/color"
	"log"
	"os"
	"time"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gesture"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

const history = 5

var allKinds = pointer.Press | pointer.Release | pointer.Move | pointer.Drag |
	pointer.Enter | pointer.Leave | pointer.Cancel | pointer.Scroll |
	pointer.Pinch | pointer.Rotate | pointer.Swipe | pointer.Hold

var debugLog = func(string) {}

type appState struct {
	theme  *material.Theme
	reset  widget.Clickable
	events []pointer.Event
	last   pointer.Event
	has    bool
	counts map[pointer.Kind]int

	clicks       int
	doubleClicks int
	lastPress    time.Duration
	lastPressPos f32.Point
	lastPressBtn pointer.Buttons

	pinch         gesture.Pinch
	rotate        gesture.Rotate
	swipe         gesture.Swipe
	hold          gesture.Hold
	fallbackCount map[pointer.Kind]int
	fallbackLast  pointer.Event
	fallbackHas   bool
}

func main() {
	debugLog("gesture-test: main")
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Gio Gesture Test"),
			app.Size(unit.Dp(900), unit.Dp(720)),
			app.MinSize(unit.Dp(640), unit.Dp(480)),
		)
		if err := loop(w); err != nil {
			debugLog("gesture-test: loop error: " + err.Error())
			log.Print(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	theme := material.NewTheme()
	// material.NewTheme leaves Shaper zero; OHOS cannot discover a system
	// font for it, so load the Go font collection explicitly.
	theme.Shaper = text.NewShaper(text.WithCollection(gofont.Collection()))
	a := &appState{
		theme:         theme,
		counts:        make(map[pointer.Kind]int),
		fallbackCount: make(map[pointer.Kind]int),
	}
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}

func (a *appState) layout(gtx layout.Context) layout.Dimensions {
	a.advanceFallback(gtx.Now)
	return layout.UniformInset(unit.Dp(16)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.H4(a.theme, "Gio Gesture Test").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Spacer{Height: unit.Dp(4)}.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return material.Body2(a.theme, "Move the pointer into the panel. Try two-finger pinch/rotate/scroll and three-finger swipe. Native semantic events and raw pointer events are shown below.").Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.counterRow(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.fallbackRow(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
			}),
			layout.Flexed(1, a.panel),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Spacer{Height: unit.Dp(10)}.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.details(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Spacer{Height: unit.Dp(8)}.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return a.log(gtx)
			}),
		)
	})
}

func (a *appState) counterRow(gtx layout.Context) layout.Dimensions {
	if a.reset.Clicked(gtx) {
		a.counts = make(map[pointer.Kind]int)
		a.events = nil
		a.has = false
		a.clicks = 0
		a.doubleClicks = 0
		a.lastPress = 0
		a.fallbackCount = make(map[pointer.Kind]int)
		a.fallbackHas = false
		a.pinch = gesture.Pinch{}
		a.rotate = gesture.Rotate{}
		a.swipe = gesture.Swipe{}
		a.hold = gesture.Hold{}
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Button(a.theme, &a.reset, "Reset").Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: unit.Dp(16)}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			txt := fmt.Sprintf("Pinch %d  Rotate %d  Swipe %d  Hold %d  Scroll %d  Press %d  Clicks %d  Double %d",
				a.counts[pointer.Pinch], a.counts[pointer.Rotate], a.counts[pointer.Swipe],
				a.counts[pointer.Hold], a.counts[pointer.Scroll], a.counts[pointer.Press],
				a.clicks, a.doubleClicks)
			return material.Body2(a.theme, txt).Layout(gtx)
		}),
	)
}

func (a *appState) fallbackRow(gtx layout.Context) layout.Dimensions {
	txt := fmt.Sprintf("Fallback (raw touch): Pinch %d  Rotate %d  Swipe %d  Hold %d",
		a.fallbackCount[pointer.Pinch], a.fallbackCount[pointer.Rotate],
		a.fallbackCount[pointer.Swipe], a.fallbackCount[pointer.Hold])
	return material.Body2(a.theme, txt).Layout(gtx)
}

func (a *appState) panel(gtx layout.Context) layout.Dimensions {
	sz := gtx.Constraints.Max
	defer clip.Rect{Max: sz}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, a)
	paint.Fill(gtx.Ops, color.NRGBA{R: 0xf4, G: 0xf6, B: 0xf8, A: 0xff})

	for {
		e, ok := gtx.Event(pointer.Filter{Target: a, Kinds: allKinds})
		if !ok {
			break
		}
		pe, ok := e.(pointer.Event)
		if !ok {
			continue
		}
		a.record(pe, gtx.Now)
	}

	return layout.Dimensions{Size: sz}
}

func (a *appState) record(e pointer.Event, now time.Time) {
	a.last = e
	a.has = true
	a.counts[e.Kind]++
	if e.Kind == pointer.Press {
		a.clicks++
		if a.lastPress != 0 && e.Time-a.lastPress < 300*time.Millisecond {
			dx, dy := e.Position.X-a.lastPressPos.X, e.Position.Y-a.lastPressPos.Y
			if dx*dx+dy*dy < 100 && e.Buttons == a.lastPressBtn {
				a.doubleClicks++
				a.lastPress = 0
			} else {
				a.lastPress = e.Time
				a.lastPressPos = e.Position
				a.lastPressBtn = e.Buttons
			}
		} else {
			a.lastPress = e.Time
			a.lastPressPos = e.Position
			a.lastPressBtn = e.Buttons
		}
	}
	if e.Kind == pointer.Press || e.Kind == pointer.Release || e.Kind == pointer.Cancel {
		debugLog(fmt.Sprintf("raw %v id=%d pos=%v", e.Kind, e.PointerID, e.Position))
	}
	a.events = append(a.events, e)
	if len(a.events) > history {
		a.events = a.events[len(a.events)-history:]
	}
	log.Printf("gesture kind=%v source=%v phase=%v buttons=%v scale=%.4f rotation=%.4f swipe=%v fingers=%d pos=%v id=%d",
		e.Kind, e.Source, phaseLabel(e), e.Buttons, e.Scale, e.Rotation, e.Swipe, e.Fingers, e.Position, e.GestureID)
	a.feedFallback(e, now)
}

type replaySource struct {
	events []pointer.Event
	i      int
}

func (r *replaySource) Event(filters ...event.Filter) (event.Event, bool) {
	if r.i >= len(r.events) {
		return nil, false
	}
	e := r.events[r.i]
	r.i++
	return e, true
}

func rawPointerKind(k pointer.Kind) bool {
	switch k {
	case pointer.Press, pointer.Move, pointer.Drag, pointer.Release, pointer.Cancel:
		return true
	default:
		return false
	}
}

func (a *appState) feedFallback(e pointer.Event, now time.Time) {
	if !rawPointerKind(e.Kind) {
		return
	}
	a.drain(&a.pinch, e)
	a.drain(&a.rotate, e)
	a.drain(&a.swipe, e)
	src := &replaySource{events: []pointer.Event{e}}
	for {
		fe, ok := a.hold.Update(src, now)
		if !ok {
			break
		}
		a.recordFallback(fe)
	}
}

func (a *appState) drain(p interface {
	Update(gesture.PointerSource) (pointer.Event, bool)
}, e pointer.Event) {
	src := &replaySource{events: []pointer.Event{e}}
	for {
		fe, ok := p.Update(src)
		if !ok {
			return
		}
		a.recordFallback(fe)
	}
}

func (a *appState) advanceFallback(now time.Time) {
	src := &replaySource{}
	for {
		fe, ok := a.hold.Update(src, now)
		if !ok {
			return
		}
		a.recordFallback(fe)
	}
}

func (a *appState) recordFallback(e pointer.Event) {
	a.fallbackCount[e.Kind]++
	a.fallbackLast = e
	a.fallbackHas = true
	debugLog(fmt.Sprintf("fallback kind=%v phase=%v scale=%.4f rotation=%.4f swipe=%v fingers=%d",
		e.Kind, phaseLabel(e), e.Scale, e.Rotation, e.Swipe, e.Fingers))
	log.Printf("fallback kind=%v source=%v phase=%v scale=%.4f rotation=%.4f swipe=%v fingers=%d",
		e.Kind, e.Source, phaseLabel(e), e.Scale, e.Rotation, e.Swipe, e.Fingers)
}

func (a *appState) details(gtx layout.Context) layout.Dimensions {
	if !a.has {
		return material.Body1(a.theme, "Waiting for pointer events...").Layout(gtx)
	}
	e := a.last
	lines := []string{
		fmt.Sprintf("Last: %v  source=%v  phase=%v", e.Kind, e.Source, phaseLabel(e)),
		fmt.Sprintf("scale=%.4f  rotation=%.4f rad  swipe=%v  fingers=%d", e.Scale, e.Rotation, e.Swipe, e.Fingers),
		fmt.Sprintf("pos=%v  id=%d  gid=%d  buttons=%v", e.Position, e.PointerID, e.GestureID, e.Buttons),
	}
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body1(a.theme, lines[0]).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(a.theme, lines[1]).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Caption(a.theme, lines[2]).Layout(gtx)
		}),
	}
	if a.fallbackHas {
		fe := a.fallbackLast
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Body2(a.theme, fmt.Sprintf("Fallback: %v  phase=%v  scale=%.4f  rotation=%.4f  swipe=%v",
				fe.Kind, phaseLabel(fe), fe.Scale, fe.Rotation, fe.Swipe)).Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (a *appState) log(gtx layout.Context) layout.Dimensions {
	children := make([]layout.FlexChild, 0, len(a.events))
	for i := len(a.events) - 1; i >= 0; i-- {
		e := a.events[i]
		txt := fmt.Sprintf("%v %v %v  S=%.2f R=%.2f F=%d",
			e.Kind, e.Source, phaseLabel(e), e.Scale, e.Rotation, e.Fingers)
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return material.Caption(a.theme, txt).Layout(gtx)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func buttonsString(b pointer.Buttons) string {
	if b == 0 {
		return "-"
	}
	return b.String()
}

func isGestureKind(k pointer.Kind) bool {
	return k&(pointer.Pinch|pointer.Rotate|pointer.Swipe|pointer.Hold) != 0
}

func phaseLabel(e pointer.Event) string {
	if !isGestureKind(e.Kind) {
		return "-"
	}
	return phaseString(e.Phase)
}

func phaseString(p pointer.GesturePhase) string {
	switch p {
	case pointer.GestureBegin:
		return "Begin"
	case pointer.GestureUpdate:
		return "Update"
	case pointer.GestureEnd:
		return "End"
	case pointer.GestureCancel:
		return "Cancel"
	default:
		return "?"
	}
}
