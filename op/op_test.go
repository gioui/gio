// SPDX-License-Identifier: Unlicense OR MIT

package op

import (
	"image"
	"testing"

	"gioui.org/internal/ops"
)

func TestTransformChecks(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("cross-macro Pop didn't panic")
		}
	}()
	var ops Ops
	trans := Offset(image.Point{}).Push(&ops)
	Record(&ops)
	trans.Pop()
}

func TestIncompleteMacroReader(t *testing.T) {
	var o Ops
	// Record, but don't Stop it.
	Record(&o)
	Offset(image.Point{}).Push(&o)

	var r ops.Reader

	r.Reset(&o.Internal)
	if _, more := r.Decode(); more {
		t.Error("decoded an operation from a semantically empty Ops")
	}
}
