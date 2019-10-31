// SPDX-License-Identifier: Unlicense OR MIT

// A dead simple app that just paints the background red.
package main

import (
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op/paint"
)

func main() {
	go func() {
		w := app.NewWindow()
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
	}()
	app.Main()
}

func loop(w *app.Window) error {
	topLeft := color.RGBA{R: 0xde, G: 0xad, B: 0xbe, A: 0xff}
	topRight := color.RGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	botLeft := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	botRight := color.RGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x80}

	gtx := &layout.Context{
		Queue: w.Queue(),
	}
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx.Reset(e.Config, e.Size)
			rows := layout.Flex{Axis: layout.Vertical}
			r1 := rows.Flex(gtx, 0.5, func() {
				columns := layout.Flex{Axis: layout.Horizontal}
				r1c1 := columns.Flex(gtx, 0.5, quarterWidget(gtx, topLeft))
				r1c2 := columns.Flex(gtx, 0.5, quarterWidget(gtx, topRight))
				columns.Layout(gtx, r1c1, r1c2)
			})
			r2 := rows.Flex(gtx, 0.5, func() {
				columns := layout.Flex{Axis: layout.Horizontal}
				r2c1 := columns.Flex(gtx, 0.5, quarterWidget(gtx, botLeft))
				r2c2 := columns.Flex(gtx, 0.5, quarterWidget(gtx, botRight))
				columns.Layout(gtx, r2c1, r2c2)
			})
			rows.Layout(gtx, r1, r2)
			e.Frame(gtx.Ops)
		}
	}
}

func quarterWidget(gtx *layout.Context, clr color.RGBA) func() {
	return func() {
		paint.ColorOp{Color: clr}.Add(gtx.Ops)
		paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{
			X: float32(gtx.Constraints.Width.Max),
			Y: float32(gtx.Constraints.Height.Max),
		}}}.Add(gtx.Ops)
	}
}
