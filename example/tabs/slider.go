// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
)

const defaultDuration = 300 * time.Millisecond

// Slider implements sliding between old/new widget values.
type Slider struct {
	Duration time.Duration

	push int

	last *op.Ops
	next *op.Ops

	t0     time.Time
	offset float32
}

// PushLeft pushes the existing widget to the left.
func (s *Slider) PushLeft() { s.push = 1 }

// PushRight pushes the existing widget to the right.
func (s *Slider) PushRight() { s.push = -1 }

// Layout lays out widget that can be pushed.
func (s *Slider) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	if s.push != 0 {
		s.last, s.next = s.next, new(op.Ops)
		s.offset = float32(s.push)
		s.t0 = gtx.Now()
		s.push = 0
	}

	var delta time.Duration
	if !s.t0.IsZero() {
		now := gtx.Now()
		delta = now.Sub(s.t0)
		s.t0 = now
	}

	if s.offset != 0 {
		duration := s.Duration
		if duration == 0 {
			duration = defaultDuration
		}
		movement := float32(delta.Seconds()) / float32(duration.Seconds())
		if s.offset < 0 {
			s.offset += movement
			if s.offset >= 0 {
				s.offset = 0
			}
		} else {
			s.offset -= movement
			if s.offset <= 0 {
				s.offset = 0
			}
		}

		op.InvalidateOp{}.Add(gtx.Ops)
	}

	var dims layout.Dimensions
	{
		if s.next == nil {
			s.next = new(op.Ops)
		}
		s.next.Reset()
		gtx := gtx
		gtx.Ops = s.next
		dims = w(gtx)
	}

	if s.offset == 0 {
		op.CallOp{Ops: s.next}.Add(gtx.Ops)
		return dims
	}

	var stack op.StackOp
	stack.Push(gtx.Ops)
	defer stack.Pop()

	offset := smooth(s.offset)

	if s.offset > 0 {
		op.TransformOp{}.Offset(f32.Point{
			X: float32(dims.Size.X) * (offset - 1),
		}).Add(gtx.Ops)
		op.CallOp{Ops: s.last}.Add(gtx.Ops)

		op.TransformOp{}.Offset(f32.Point{
			X: float32(dims.Size.X),
		}).Add(gtx.Ops)
		op.CallOp{Ops: s.next}.Add(gtx.Ops)
	} else {
		op.TransformOp{}.Offset(f32.Point{
			X: float32(dims.Size.X) * (offset + 1),
		}).Add(gtx.Ops)
		op.CallOp{Ops: s.last}.Add(gtx.Ops)

		op.TransformOp{}.Offset(f32.Point{
			X: float32(-dims.Size.X),
		}).Add(gtx.Ops)
		op.CallOp{Ops: s.next}.Add(gtx.Ops)
	}
	return dims
}

// smooth handles -1 to 1 with ease-in-out cubic easing func.
func smooth(t float32) float32 {
	if t < 0 {
		return -easeInOutCubic(-t)
	}
	return easeInOutCubic(t)
}

// easeInOutCubic maps a linear value to a ease-in-out-cubic easing function.
func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return (t-1)*(2*t-2)*(2*t-2) + 1
}
