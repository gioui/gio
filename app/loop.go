// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"image"
	"runtime"

	"gioui.org/app/internal/gpu"
	"gioui.org/app/internal/window"
	"gioui.org/op"
)

type renderLoop struct {
	summary string
	drawing bool
	err     error

	frames     chan frame
	results    chan frameResult
	refresh    chan struct{}
	refreshErr chan error
	ack        chan struct{}
	stop       chan struct{}
	stopped    chan struct{}
}

type frame struct {
	collectStats bool
	viewport     image.Point
	ops          *op.Ops
}

type frameResult struct {
	summary string
	err     error
}

func newLoop(ctx window.Context) (*renderLoop, error) {
	l := &renderLoop{
		frames:     make(chan frame),
		results:    make(chan frameResult),
		refresh:    make(chan struct{}),
		refreshErr: make(chan error),
		// Ack is buffered so GPU commands can be issued after
		// ack'ing the frame.
		ack:     make(chan struct{}, 1),
		stop:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
	if err := l.renderLoop(ctx); err != nil {
		return nil, err
	}
	return l, nil
}

func (l *renderLoop) renderLoop(glctx window.Context) error {
	// GL Operations must happen on a single OS thread, so
	// pass initialization result through a channel.
	initErr := make(chan error)
	go func() {
		defer close(l.stopped)
		runtime.LockOSThread()
		// Don't UnlockOSThread to avoid reuse by the Go runtime.

		if err := glctx.MakeCurrent(); err != nil {
			initErr <- err
			return
		}
		g, err := gpu.New(glctx.Functions())
		if err != nil {
			initErr <- err
			return
		}
		defer glctx.Release()
		initErr <- nil
	loop:
		for {
			select {
			case <-l.refresh:
				l.refreshErr <- glctx.MakeCurrent()
			case frame := <-l.frames:
				glctx.Lock()
				g.Collect(frame.collectStats, frame.viewport, frame.ops)
				// Signal that we're done with the frame ops.
				l.ack <- struct{}{}
				g.Frame(frame.collectStats, frame.viewport)
				var res frameResult
				res.err = glctx.Present()
				res.summary = g.EndFrame(frame.collectStats)
				glctx.Unlock()
				l.results <- res
			case <-l.stop:
				break loop
			}
		}
	}()
	return <-initErr
}

func (l *renderLoop) Release() {
	// Flush error.
	l.Flush()
	close(l.stop)
	<-l.stopped
	l.stop = nil
}

func (l *renderLoop) Flush() error {
	if l.drawing {
		st := <-l.results
		l.setErr(st.err)
		if st.summary != "" {
			l.summary = st.summary
		}
		l.drawing = false
	}
	return l.err
}

func (l *renderLoop) Summary() string {
	return l.summary
}

func (l *renderLoop) Refresh() {
	if l.err != nil {
		return
	}
	// Make sure any pending frame is complete.
	l.Flush()
	l.refresh <- struct{}{}
	l.setErr(<-l.refreshErr)
}

// Draw initiates a draw of a frame. It returns a channel
// than signals when the frame is no longer being accessed.
func (l *renderLoop) Draw(profile bool, viewport image.Point, frameOps *op.Ops) <-chan struct{} {
	if l.err != nil {
		l.ack <- struct{}{}
		return l.ack
	}
	l.Flush()
	l.frames <- frame{profile, viewport, frameOps}
	l.drawing = true
	return l.ack
}

func (l *renderLoop) setErr(err error) {
	if l.err == nil {
		l.err = err
	}
}
