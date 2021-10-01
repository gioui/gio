// SPDX-License-Identifier: Unlicense OR MIT

package op

import (
	"testing"

	"gioui.org/f32"
)

func TestTransformChecks(t *testing.T) {
	defer func() {
		if err := recover(); err == nil {
			t.Error("cross-macro Pop didn't panic")
		}
	}()
	var ops Ops
	trans := Offset(f32.Point{}).Push(&ops)
	Record(&ops)
	trans.Pop()
}
