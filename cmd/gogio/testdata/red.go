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
	background := color.RGBA{R: 0xde, G: 0xad, B: 0xbe, A: 0xff}
	ops := new(op.Ops)
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			ops.Reset()
			paint.ColorOp{Color: background}.Add(ops)
			paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{
				X: float32(e.Size.X),
				Y: float32(e.Size.Y),
			}}}.Add(ops)
			e.Frame(ops)
		}
	}
}
