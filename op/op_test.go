// SPDX-License-Identifier: Unlicense OR MIT

package op

import (
	"image"
	"testing"
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
