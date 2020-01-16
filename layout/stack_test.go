// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"testing"
)

func TestStack(t *testing.T) {
	var gtx Context
	gtx.Reset(nil, image.Point{X: 100, Y: 100})
	gtx.Constraints.Width.Min = 0
	gtx.Constraints.Height.Min = 0
	exp := image.Point{X: 60, Y: 70}
	Stack{Alignment: Center}.Layout(&gtx,
		Expanded(func() {
			gtx.Dimensions.Size = exp
		}),
		Stacked(func() {
			gtx.Dimensions.Size = image.Point{X: 50, Y: 50}
		}),
	)
	if got := gtx.Dimensions.Size; got != exp {
		t.Errorf("Stack ignored Expanded size, got %v expected %v", got, exp)
	}
}
