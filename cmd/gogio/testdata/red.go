// SPDX-License-Identifier: Unlicense OR MIT

// A dead simple app that just paints the background red.
package main

import (
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/f32"
	"gioui.org/io/system"
	"gioui.org/op"
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

	ops := new(op.Ops)
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			ops.Reset()

			paint.ColorOp{Color: topLeft}.Add(ops)
			paint.PaintOp{Rect: f32.Rectangle{
				Min: f32.Point{
					X: 0,
					Y: 0,
				},
				Max: f32.Point{
					X: float32(e.Size.X)/2,
					Y: float32(e.Size.Y)/2,
				},
			}}.Add(ops)

			paint.ColorOp{Color: topRight}.Add(ops)
			paint.PaintOp{Rect: f32.Rectangle{
				Min: f32.Point{
					X: float32(e.Size.X)/2,
					Y: 0,
				},
				Max: f32.Point{
					X: float32(e.Size.X),
					Y: float32(e.Size.Y)/2,
				},
			}}.Add(ops)

			paint.ColorOp{Color: botLeft}.Add(ops)
			paint.PaintOp{Rect: f32.Rectangle{
				Min: f32.Point{
					X: 0,
					Y: float32(e.Size.Y)/2,
				},
				Max: f32.Point{
					X: float32(e.Size.X)/2,
					Y: float32(e.Size.Y),
				},
			}}.Add(ops)

			paint.ColorOp{Color: botRight}.Add(ops)
			paint.PaintOp{Rect: f32.Rectangle{
				Min: f32.Point{
					X: float32(e.Size.X)/2,
					Y: float32(e.Size.Y)/2,
				},
				Max: f32.Point{
					X: float32(e.Size.X),
					Y: float32(e.Size.Y),
				},
			}}.Add(ops)

			e.Frame(ops)
		}
	}
}
