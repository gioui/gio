// SPDX-License-Identifier: Unlicense OR MIT

package pointer

import (
	"testing"

	"gioui.org/op"
)

func TestTransformChecks(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("cross-macro Pop didn't panic")
		}
	}()
	var ops op.Ops
	area := AreaOp{}.Push(&ops)
	op.Record(&ops)
	area.Pop()
}
