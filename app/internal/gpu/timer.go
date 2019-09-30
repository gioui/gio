// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"time"

	"gioui.org/app/internal/gl"
)

type timers struct {
	ctx    *context
	timers []*timer
}

type timer struct {
	Elapsed time.Duration
	ctx     *context
	obj     gl.Query
	state   timerState
}

type timerState uint8

const (
	timerIdle timerState = iota
	timerRunning
	timerWaiting
)

func newTimers(ctx *context) *timers {
	return &timers{
		ctx: ctx,
	}
}

func (t *timers) newTimer() *timer {
	if t == nil {
		return nil
	}
	tt := &timer{
		ctx: t.ctx,
		obj: t.ctx.CreateQuery(),
	}
	t.timers = append(t.timers, tt)
	return tt
}

func (t *timer) begin() {
	if t == nil || t.state != timerIdle {
		return
	}
	t.ctx.BeginQuery(gl.TIME_ELAPSED_EXT, t.obj)
	t.state = timerRunning
}

func (t *timer) end() {
	if t == nil || t.state != timerRunning {
		return
	}
	t.ctx.EndQuery(gl.TIME_ELAPSED_EXT)
	t.state = timerWaiting
}

func (t *timers) ready() bool {
	if t == nil {
		return false
	}
	for _, tt := range t.timers {
		if tt.state != timerWaiting {
			return false
		}
		if t.ctx.GetQueryObjectuiv(tt.obj, gl.QUERY_RESULT_AVAILABLE) == 0 {
			return false
		}
	}
	for _, tt := range t.timers {
		tt.state = timerIdle
		nanos := t.ctx.GetQueryObjectuiv(tt.obj, gl.QUERY_RESULT)
		tt.Elapsed = time.Duration(nanos)
	}
	return t.ctx.GetInteger(gl.GPU_DISJOINT_EXT) == 0
}

func (t *timers) release() {
	if t == nil {
		return
	}
	for _, tt := range t.timers {
		t.ctx.DeleteQuery(tt.obj)
	}
	t.timers = nil
}
