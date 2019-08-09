// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"errors"
	"fmt"
	"image"
	"time"

	"gioui.org/ui"
	"gioui.org/ui/app/internal/gpu"
	iinput "gioui.org/ui/app/internal/input"
	"gioui.org/ui/input"
	"gioui.org/ui/system"
)

// WindowOptions specifies a set of window properties
// for creating new Windows.
type WindowOptions struct {
	Width  ui.Value
	Height ui.Value
	Title  string
}

// Window represents an operating system window.
type Window struct {
	driver    *window
	lastFrame time.Time
	drawStart time.Time
	gpu       *gpu.GPU

	out         chan input.Event
	in          chan input.Event
	ack         chan struct{}
	invalidates chan struct{}
	frames      chan *ui.Ops

	stage        Stage
	animating    bool
	hasNextFrame bool
	nextFrame    time.Time
	delayedDraw  *time.Timer

	queue Queue
}

// Queue is an input.Queue implementation that distributes
// system input events to the input handlers declared in the
// most recent call to Update.
type Queue struct {
	q iinput.Router
}

// driverEvent is sent when a new native driver
// is available for the Window.
type driverEvent struct {
	driver *window
}

// driver is the interface for the platform implementation
// of a Window.
var _ interface {
	// setAnimating sets the animation flag. When the window is animating,
	// UpdateEvents are delivered as fast as the display can handle them.
	setAnimating(anim bool)
	// showTextInput updates the virtual keyboard state.
	showTextInput(show bool)
} = (*window)(nil)

// Pre-allocate the ack event to avoid garbage.
var ackEvent input.Event

// NewWindow creates a new window for a set of window
// options. The options are hints; the platform is free to
// ignore or adjust them.
// If the current program is running on iOS and Android,
// NewWindow returns the window previously created by the
// platform.
func NewWindow(opts *WindowOptions) *Window {
	if opts == nil {
		opts = &WindowOptions{
			Width:  ui.Dp(800),
			Height: ui.Dp(600),
			Title:  "Gio",
		}
	}
	if opts.Width.V <= 0 || opts.Height.V <= 0 {
		panic("window width and height must be larger than 0")
	}

	w := &Window{
		in:          make(chan input.Event),
		out:         make(chan input.Event),
		ack:         make(chan struct{}),
		invalidates: make(chan struct{}, 1),
		frames:      make(chan *ui.Ops),
	}
	go w.run(opts)
	return w
}

// Events returns the channel where events are delivered.
func (w *Window) Events() <-chan input.Event {
	return w.out
}

// Queue returns the Window's event queue. The queue contains
// the events received since the last UpdateEvent.
func (w *Window) Queue() *Queue {
	return &w.queue
}

// Update updates the Window. Paint operations updates the
// window contents, input operations declare input handlers,
// and so on. The supplied operations list completely replaces
// the window state from previous calls.
func (w *Window) Update(frame *ui.Ops) {
	w.frames <- frame
}

func (w *Window) draw(size image.Point, frame *ui.Ops) {
	var drawDur time.Duration
	if !w.drawStart.IsZero() {
		drawDur = time.Since(w.drawStart)
		w.drawStart = time.Time{}
	}
	w.gpu.Draw(w.queue.q.Profiling(), size, frame)
	w.queue.q.Frame(frame)
	now := time.Now()
	switch w.queue.q.TextInputState() {
	case iinput.TextInputOpen:
		w.driver.showTextInput(true)
	case iinput.TextInputClose:
		w.driver.showTextInput(false)
	}
	frameDur := now.Sub(w.lastFrame)
	frameDur = frameDur.Truncate(100 * time.Microsecond)
	w.lastFrame = now
	if w.queue.q.Profiling() {
		q := 100 * time.Microsecond
		timings := fmt.Sprintf("tot:%7s cpu:%7s %s", frameDur.Round(q), drawDur.Round(q), w.gpu.Timings())
		w.queue.q.AddProfile(system.ProfileEvent{Timings: timings})
		w.setNextFrame(time.Time{})
	}
	if t, ok := w.queue.q.WakeupTime(); ok {
		w.setNextFrame(t)
	}
	w.updateAnimation()
}

// Invalidate the current window such that a UpdateEvent will be generated
// immediately. If the window is inactive, the event is sent when the
// window becomes active.
func (w *Window) Invalidate() {
	select {
	case w.invalidates <- struct{}{}:
	default:
	}
}

func (w *Window) updateAnimation() {
	animate := false
	if w.delayedDraw != nil {
		w.delayedDraw.Stop()
		w.delayedDraw = nil
	}
	if w.stage >= StageRunning && w.hasNextFrame {
		if dt := time.Until(w.nextFrame); dt <= 0 {
			animate = true
		} else {
			w.delayedDraw = time.NewTimer(dt)
		}
	}
	if animate != w.animating {
		w.animating = animate
		w.driver.setAnimating(animate)
	}
}

func (w *Window) setNextFrame(at time.Time) {
	if !w.hasNextFrame || at.Before(w.nextFrame) {
		w.hasNextFrame = true
		w.nextFrame = at
	}
}

func (w *Window) setDriver(d *window) {
	w.event(driverEvent{d})
}

func (w *Window) event(e input.Event) {
	w.in <- e
	<-w.ack
}

func (w *Window) waitAck() {
	// Send a dummy event; when it gets through we
	// know the application has processed the previous event.
	w.out <- ackEvent
}

// Prematurely destroy the window and wait for the native window
// destroy event.
func (w *Window) destroy(err error) {
	// Ack the current event.
	w.ack <- struct{}{}
	w.out <- DestroyEvent{err}
	for e := range w.in {
		w.ack <- struct{}{}
		if _, ok := e.(DestroyEvent); ok {
			return
		}
	}
}

func (w *Window) run(opts *WindowOptions) {
	defer close(w.in)
	defer close(w.out)
	if err := createWindow(w, opts); err != nil {
		w.out <- DestroyEvent{err}
		return
	}
	for {
		var timer <-chan time.Time
		if w.delayedDraw != nil {
			timer = w.delayedDraw.C
		}
		select {
		case <-timer:
			w.setNextFrame(time.Time{})
			w.updateAnimation()
		case <-w.invalidates:
			w.setNextFrame(time.Time{})
			w.updateAnimation()
		case e := <-w.in:
			switch e2 := e.(type) {
			case StageEvent:
				if w.gpu != nil {
					if e2.Stage < StageRunning {
						w.gpu.Release()
						w.gpu = nil
					} else {
						w.gpu.Refresh()
					}
				}
				w.stage = e2.Stage
				w.updateAnimation()
				w.out <- e
				w.waitAck()
			case UpdateEvent:
				if e2.Size == (image.Point{}) {
					panic(errors.New("internal error: zero-sized Draw"))
				}
				if w.stage < StageRunning {
					// No drawing if not visible.
					break
				}
				w.drawStart = time.Now()
				w.hasNextFrame = false
				w.out <- e
				var frame *ui.Ops
				// Wait for either a frame or the ack event,
				// which meant that the client didn't draw.
				select {
				case frame = <-w.frames:
				case w.out <- ackEvent:
				}
				if w.gpu != nil {
					if e2.sync {
						w.gpu.Refresh()
					}
					if err := w.gpu.Flush(); err != nil {
						w.gpu.Release()
						w.gpu = nil
						w.destroy(err)
						return
					}
				} else {
					ctx, err := newContext(w.driver)
					if err != nil {
						w.destroy(err)
						return
					}
					w.gpu, err = gpu.NewGPU(ctx)
					if err != nil {
						w.destroy(err)
						return
					}
				}
				w.draw(e2.Size, frame)
				if e2.sync {
					if err := w.gpu.Flush(); err != nil {
						w.gpu.Release()
						w.gpu = nil
						w.destroy(err)
						return
					}
				}
			case *CommandEvent:
				w.out <- e
				w.waitAck()
			case driverEvent:
				w.driver = e2.driver
			case DestroyEvent:
				w.out <- e2
				w.ack <- struct{}{}
				return
			case input.Event:
				if w.queue.q.Add(e2) {
					w.setNextFrame(time.Time{})
					w.updateAnimation()
				}
				w.out <- e
			}
			w.ack <- struct{}{}
		}
	}
}

func (q *Queue) Next(k input.Key) (input.Event, bool) {
	return q.q.Next(k)
}

func (_ driverEvent) ImplementsEvent() {}
