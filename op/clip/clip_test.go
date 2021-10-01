// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"testing"

	"gioui.org/f32"
	"gioui.org/op"
)

func TestOpenPathOutlinePanic(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("Outline of an open path didn't panic")
		}
	}()
	var p Path
	p.Begin(new(op.Ops))
	p.Line(f32.Pt(10, 10))
	Outline{Path: p.End()}.Op()
}

func TestTransformChecks(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("cross-macro Pop didn't panic")
		}
	}()
	var ops op.Ops
	st := Op{}.Push(&ops)
	op.Record(&ops)
	st.Pop()
}
