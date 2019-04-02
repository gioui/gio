// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/app/internal/gpu"
	"gioui.org/ui/key"
	"gioui.org/ui/pointer"
)

type WindowOptions struct {
	Width  ui.Value
	Height ui.Value
	Title  string
}

type Window struct {
	Profiling bool

	driver     *window
	lastFrame  time.Time
	gpu        *gpu.GPU
	timings    string
	inputState key.TextInputState
	err        error

	events chan Event

	mu           sync.Mutex
	stage        Stage
	size         image.Point
	syncGPU      bool
	animating    bool
	hasNextFrame bool
	nextFrame    time.Time
	delayedDraw  *time.Timer
}

// driver is the interface for the platform implementation
// of a Window.
var _ interface {
	// setAnimating sets the animation flag. When the window is animating,
	// Draw events are delivered as fast as the display can handle them.
	setAnimating(anim bool)
	// setTextInput updates the virtual keyboard state.
	setTextInput(s key.TextInputState)
} = (*window)(nil)

var ackEvent Event

func newWindow(nw *window) *Window {
	w := &Window{
		driver: nw,
		events: make(chan Event),
		stage:  StageInvisible,
	}
	return w
}

func (w *Window) Events() <-chan Event {
	return w.events
}

func (w *Window) Timings() string {
	return w.timings
}

func (w *Window) SetTextInput(s key.TextInputState) {
	if !w.IsAlive() {
		return
	}
	if s != w.inputState && (s == key.TextInputClosed || s == key.TextInputOpen) {
		w.driver.setTextInput(s)
	}
	if s == key.TextInputFocus {
		w.Redraw()
	}
	w.inputState = s
}

func (w *Window) Err() error {
	return w.err
}

func (w *Window) Draw(root ui.Op) {
	if !w.IsAlive() {
		return
	}
	w.mu.Lock()
	stage := w.stage
	sync := w.syncGPU
	size := w.size
	w.hasNextFrame = false
	w.syncGPU = false
	w.mu.Unlock()
	if stage < StageVisible {
		return
	}
	if w.gpu != nil {
		if sync {
			w.gpu.Refresh()
		}
		if err := w.gpu.Flush(); err != nil {
			w.gpu.Release()
			w.gpu = nil
		}
	}
	if w.gpu == nil {
		ctx, err := newContext(w.driver)
		if err != nil {
			w.err = err
			return
		}
		w.gpu, err = gpu.NewGPU(ctx)
		if err != nil {
			w.err = err
			return
		}
	}
	now := time.Now()
	frameDur := now.Sub(w.lastFrame)
	frameDur = frameDur.Truncate(100 * time.Microsecond)
	w.lastFrame = now
	if w.Profiling {
		w.timings = fmt.Sprintf("t:%7s %s", frameDur, w.gpu.Timings())
		w.setNextFrame(time.Time{})
	}
	if t, ok := collectRedraws(root); ok {
		w.setNextFrame(t)
	}
	w.updateAnimation()
	w.gpu.Draw(w.Profiling, size, root)
}

func (w *Window) Redraw() {
	if !w.IsAlive() {
		return
	}
	w.setNextFrame(time.Time{})
	w.updateAnimation()
}

func (w *Window) updateAnimation() {
	w.mu.Lock()
	defer w.mu.Unlock()
	animate := false
	if w.stage >= StageVisible && w.hasNextFrame {
		if dt := time.Until(w.nextFrame); dt <= 0 {
			animate = true
		} else {
			w.delayedDraw = time.AfterFunc(dt, w.Redraw)
		}
	}
	if animate != w.animating {
		w.animating = animate
		w.driver.setAnimating(animate)
	}
}

func (w *Window) setNextFrame(at time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hasNextFrame || at.Before(w.nextFrame) {
		if w.delayedDraw != nil {
			w.delayedDraw.Stop()
			w.delayedDraw = nil
		}
		w.hasNextFrame = true
		w.nextFrame = at
	}
}

func (w *Window) Size() image.Point {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.size
}

func (w *Window) Stage() Stage {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stage
}

func (w *Window) IsAlive() bool {
	return w.Stage() != StageDead && w.err == nil
}

func (w *Window) contextDriver() interface{} {
	return w.driver
}

func (w *Window) event(e Event) {
	w.mu.Lock()
	needAck := false
	needRedraw := false
	switch e := e.(type) {
	case pointer.Event:
		needRedraw = true
	case key.Event:
		needRedraw = true
	case *Command:
		needAck = true
		needRedraw = true
	case ChangeStage:
		w.stage = e.Stage
		if w.stage > StageDead {
			needAck = true
			w.syncGPU = true
		}
	case Draw:
		if e.Size == (image.Point{}) {
			panic(errors.New("internal error: zero-sized Draw"))
		}
		if w.stage < StageVisible {
			// No drawing if not visible.
			break
		}
		needAck = true
		w.syncGPU = e.sync
		w.size = e.Size
	}
	stage := w.stage
	w.mu.Unlock()
	if needRedraw {
		w.setNextFrame(time.Time{})
	}
	w.updateAnimation()
	w.events <- e
	if needAck {
		// Send a dummy event; when it gets through we
		// know the application has processed the actual event.
		w.events <- ackEvent
	}
	if w.gpu != nil {
		w.mu.Lock()
		sync := w.syncGPU
		w.syncGPU = false
		w.mu.Unlock()
		switch {
		case stage < StageVisible:
			w.gpu.Release()
			w.gpu = nil
		case sync:
			w.gpu.Refresh()
		}
	}
	if stage == StageDead {
		close(w.events)
	}
}

func collectRedraws(op ui.Op) (time.Time, bool) {
	type childOp interface {
		ChildOp() ui.Op
	}
	switch op := op.(type) {
	case ui.Ops:
		var earliest time.Time
		var valid bool
		for _, op := range op {
			if t, ok := collectRedraws(op); ok {
				if !valid || t.Before(earliest) {
					valid = true
					earliest = t
				}
			}
		}
		return earliest, valid
	case ui.OpRedraw:
		return op.At, true
	case childOp:
		return collectRedraws(op.ChildOp())
	default:
		return time.Time{}, false
	}
}
