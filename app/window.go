// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"errors"
	"fmt"
	"image"
	"time"

	"gioui.org/app/internal/input"
	"gioui.org/app/internal/window"
	"gioui.org/io/event"
	"gioui.org/io/profile"
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/unit"

	_ "gioui.org/app/internal/log"
)

// WindowOption configures a Window.
type Option func(opts *window.Options)

// Window represents an operating system window.
type Window struct {
	driver window.Driver
	loop   *renderLoop

	// driverFuncs is a channel of functions to run when
	// the Window has a valid driver.
	driverFuncs chan func()

	out         chan event.Event
	in          chan event.Event
	ack         chan struct{}
	invalidates chan struct{}
	frames      chan *op.Ops
	frameAck    chan struct{}

	stage        system.Stage
	animating    bool
	hasNextFrame bool
	nextFrame    time.Time
	delayedDraw  *time.Timer

	queue Queue

	callbacks callbacks
}

type callbacks struct {
	w *Window
}

// Queue is an event.Queue implementation that distributes system events
// to the input handlers declared in the most recent frame.
type Queue struct {
	q input.Router
}

// driverEvent is sent when a new native driver
// is available for the Window.
type driverEvent struct {
	driver window.Driver
}

// Pre-allocate the ack event to avoid garbage.
var ackEvent event.Event

// NewWindow creates a new window for a set of window
// options. The options are hints; the platform is free to
// ignore or adjust them.
//
// If opts are nil, a set of sensible defaults are used.
//
// If the current program is running on iOS and Android,
// NewWindow returns the window previously created by the
// platform.
//
// BUG: Calling NewWindow more than once is not yet supported.
func NewWindow(options ...Option) *Window {
	opts := &window.Options{
		Width:  unit.Dp(800),
		Height: unit.Dp(600),
		Title:  "Gio",
	}

	for _, o := range options {
		o(opts)
	}

	w := &Window{
		in:          make(chan event.Event),
		out:         make(chan event.Event),
		ack:         make(chan struct{}),
		invalidates: make(chan struct{}, 1),
		frames:      make(chan *op.Ops),
		frameAck:    make(chan struct{}),
		driverFuncs: make(chan func()),
	}
	w.callbacks.w = w
	go w.run(opts)
	return w
}

// Events returns the channel where events are delivered.
func (w *Window) Events() <-chan event.Event {
	return w.out
}

// Queue returns the Window's event queue. The queue contains
// the events received since the last frame.
func (w *Window) Queue() *Queue {
	return &w.queue
}

// update updates the Window. Paint operations updates the
// window contents, input operations declare input handlers,
// and so on. The supplied operations list completely replaces
// the window state from previous calls.
func (w *Window) update(frame *op.Ops) {
	w.frames <- frame
	<-w.frameAck
}

func (w *Window) draw(frameStart time.Time, size image.Point, frame *op.Ops) {
	sync := w.loop.Draw(w.queue.q.Profiling(), size, frame)
	w.queue.q.Frame(frame)
	switch w.queue.q.TextInputState() {
	case input.TextInputOpen:
		w.driver.ShowTextInput(true)
	case input.TextInputClose:
		w.driver.ShowTextInput(false)
	}
	if w.queue.q.Profiling() {
		frameDur := time.Since(frameStart)
		frameDur = frameDur.Truncate(100 * time.Microsecond)
		q := 100 * time.Microsecond
		timings := fmt.Sprintf("tot:%7s %s", frameDur.Round(q), w.loop.Summary())
		w.queue.q.AddProfile(profile.Event{Timings: timings})
	}
	if t, ok := w.queue.q.WakeupTime(); ok {
		w.setNextFrame(t)
	}
	w.updateAnimation()
	// Wait for the GPU goroutine to finish processing frame.
	<-sync
}

// Invalidate the window such that a FrameEvent will be generated
// immediately. If the window is inactive, the event is sent when the
// window becomes active.
// Invalidate is safe for concurrent use.
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
	if w.stage >= system.StageRunning && w.hasNextFrame {
		if dt := time.Until(w.nextFrame); dt <= 0 {
			animate = true
		} else {
			w.delayedDraw = time.NewTimer(dt)
		}
	}
	if animate != w.animating {
		w.animating = animate
		w.driver.SetAnimating(animate)
	}
}

func (w *Window) setNextFrame(at time.Time) {
	if !w.hasNextFrame || at.Before(w.nextFrame) {
		w.hasNextFrame = true
		w.nextFrame = at
	}
}

func (c *callbacks) SetDriver(d window.Driver) {
	c.Event(driverEvent{d})
}

func (c *callbacks) Event(e event.Event) {
	c.w.in <- e
	<-c.w.ack
}

func (w *Window) waitAck() {
	// Send a dummy event; when it gets through we
	// know the application has processed the previous event.
	w.out <- ackEvent
}

// Prematurely destroy the window and wait for the native window
// destroy event.
func (w *Window) destroy(err error) {
	w.destroyGPU()
	// Ack the current event.
	w.ack <- struct{}{}
	w.out <- system.DestroyEvent{Err: err}
	for e := range w.in {
		w.ack <- struct{}{}
		if _, ok := e.(system.DestroyEvent); ok {
			return
		}
	}
}

func (w *Window) destroyGPU() {
	if w.loop != nil {
		w.loop.Release()
		w.loop = nil
	}
}

func (w *Window) run(opts *window.Options) {
	defer close(w.in)
	defer close(w.out)
	if err := window.NewWindow(&w.callbacks, opts); err != nil {
		w.out <- system.DestroyEvent{Err: err}
		return
	}
	for {
		var driverFuncs chan func() = nil
		if w.driver != nil {
			driverFuncs = w.driverFuncs
		}
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
		case f := <-driverFuncs:
			f()
		case e := <-w.in:
			switch e2 := e.(type) {
			case system.StageEvent:
				if w.loop != nil {
					if e2.Stage < system.StageRunning {
						w.loop.Release()
						w.loop = nil
					} else {
						w.loop.Refresh()
					}
				}
				w.stage = e2.Stage
				w.updateAnimation()
				w.out <- e
				w.waitAck()
			case window.FrameEvent:
				if e2.Size == (image.Point{}) {
					panic(errors.New("internal error: zero-sized Draw"))
				}
				if w.stage < system.StageRunning {
					// No drawing if not visible.
					break
				}
				frameStart := time.Now()
				w.hasNextFrame = false
				e2.Frame = w.update
				w.out <- e2.FrameEvent
				var err error
				if w.loop != nil {
					if e2.Sync {
						w.loop.Refresh()
					}
					if err = w.loop.Flush(); err != nil {
						w.loop.Release()
						w.loop = nil
					}
				} else {
					var ctx window.Context
					ctx, err = w.driver.NewContext()
					if err == nil {
						w.loop, err = newLoop(ctx)
						if err != nil {
							ctx.Release()
						}
					}
				}
				var frame *op.Ops
				// Wait for either a frame or an ack event to go
				// through, which means that the client didn't give us
				// a frame.
				gotFrame := false
				select {
				case frame = <-w.frames:
					gotFrame = true
				case w.out <- ackEvent:
				}
				if err != nil {
					if gotFrame {
						w.frameAck <- struct{}{}
					}
					w.destroy(err)
					return
				}
				w.draw(frameStart, e2.Size, frame)
				if gotFrame {
					// We're done with frame, let the client continue.
					w.frameAck <- struct{}{}
				}
				if e2.Sync {
					if err := w.loop.Flush(); err != nil {
						w.loop.Release()
						w.loop = nil
						w.destroy(err)
						return
					}
				}
			case *system.CommandEvent:
				w.out <- e
				w.waitAck()
			case driverEvent:
				w.driver = e2.driver
			case system.DestroyEvent:
				w.destroyGPU()
				w.out <- e2
				w.ack <- struct{}{}
				return
			case event.Event:
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

func (q *Queue) Events(k event.Key) []event.Event {
	return q.q.Events(k)
}

// Title sets the title of the window.
func Title(t string) Option {
	return func(opts *window.Options) {
		opts.Title = t
	}
}

// Size sets the size of the window.
func Size(w, h unit.Value) Option {
	if w.V <= 0 {
		panic("width must be larger than or equal to 0")
	}
	if h.V <= 0 {
		panic("height must be larger than or equal to 0")
	}
	return func(opts *window.Options) {
		opts.Width = w
		opts.Height = h
	}
}

func (driverEvent) ImplementsEvent() {}
