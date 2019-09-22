// SPDX-License-Identifier: Unlicense OR MIT

// A dead simple app that just paints the background red.
package main

import (
	"image/color"
	"log"

	"gioui.org/ui"
	"gioui.org/ui/app"
	"gioui.org/ui/f32"
	"gioui.org/ui/paint"
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
	background := color.RGBA{255, 0, 0, 255}
	ops := new(ui.Ops)
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.UpdateEvent:
			ops.Reset()
			paint.ColorOp{Color: background}.Add(ops)
			paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{
				X: float32(e.Size.X),
				Y: float32(e.Size.Y),
			}}}.Add(ops)
			w.Update(ops)
		}
	}
}
