// SPDX-License-Identifier: Unlicense OR MIT

package rendertest

import (
	"image"
	"testing"

	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"golang.org/x/image/colornames"
)

func TestBorderRendering(t *testing.T) {
	run(t, func(o *op.Ops) {
		pixelBorder := func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Max
			paint.FillShape(gtx.Ops, red, clip.Rect(image.Rect(0, 0, 1, size.Y)).Op())
			paint.FillShape(gtx.Ops, blue, clip.Rect(image.Rect(size.X-1, 0, size.X, size.Y)).Op())
			paint.FillShape(gtx.Ops, green, clip.Rect(image.Rect(1, 0, size.X-1, 1)).Op())
			paint.FillShape(gtx.Ops, black, clip.Rect(image.Rect(1, size.Y-1, size.X-1, size.Y)).Op())
			return layout.Dimensions{Size: size}
		}

		gtx := layout.NewContext(o, system.FrameEvent{Size: image.Point{X: 128, Y: 128}})
		widget.Border{
			Color: yellow,
			Width: unit.Px(4),
		}.Layout(gtx, pixelBorder)

		layout.UniformInset(unit.Px(32)).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return widget.Border{
				Color: yellow,
				Width: unit.Px(3),
			}.Layout(gtx, pixelBorder)
		})
	}, func(r result) {
		r.expect(3, 3, colornames.Yellow)
		r.expect(127-3, 127-3, colornames.Yellow)

		r.expect(5, 5, colornames.White)
		r.expect(127-5, 127-5, colornames.White)

		r.expect(4, 4, colornames.Red)
		r.expect(4, 127-4, colornames.Red)

		r.expect(127-4, 4, colornames.Blue)
		r.expect(127-4, 127-4, colornames.Blue)

		r.expect(5, 4, colornames.Green)
		r.expect(127-5, 4, colornames.Green)

		r.expect(127-5, 127-4, colornames.Black)
		r.expect(127-5, 127-4, colornames.Black)
	})
}
