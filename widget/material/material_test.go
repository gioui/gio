// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/widget"
)

func TestButtonLayout(t *testing.T) {
	var gtx layout.Context
	gtx.Reset(nil, image.Point{X: 100, Y: 100})

	ButtonLayout{}.Layout(&gtx, new(widget.Button), func() {
		if got, exp := gtx.Constraints.Width.Min, 100; got != exp {
			t.Errorf("minimum width is %d, expected %d", got, exp)
		}
		if got, exp := gtx.Constraints.Height.Min, 100; got != exp {
			t.Errorf("minimum width is %d, expected %d", got, exp)
		}
	})
}
