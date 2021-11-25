// SPDX-License-Identifier: Unlicense OR MIT

package clip_test

import (
	"image/color"
	"testing"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
)

func TestOpenPathOutlinePanic(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("Outline of an open path didn't panic")
		}
	}()
	var p clip.Path
	p.Begin(new(op.Ops))
	p.Line(f32.Pt(10, 10))
	clip.Outline{Path: p.End()}.Op()
}

func TestPathBegin(t *testing.T) {
	ops := new(op.Ops)
	var p clip.Path
	p.Begin(ops)
	p.LineTo(f32.Point{10, 10})
	p.Close()
	stack := clip.Outline{Path: p.End()}.Op().Push(ops)
	paint.Fill(ops, color.NRGBA{A: 255})
	stack.Pop()
	w := newWindow(t, 100, 100)
	if w == nil {
		return
	}
	// The following should not panic.
	_ = w.Frame(ops)
}

func TestTransformChecks(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("cross-macro Pop didn't panic")
		}
	}()
	var ops op.Ops
	st := clip.Op{}.Push(&ops)
	op.Record(&ops)
	st.Pop()
}

func newWindow(t testing.TB, width, height int) *headless.Window {
	w, err := headless.NewWindow(width, height)
	if err != nil {
		t.Skipf("failed to create headless window, skipping: %v", err)
	}
	return w
}
