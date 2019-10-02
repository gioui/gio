// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/text/shape"
	"gioui.org/unit"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
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
	regular, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		panic("failed to load font")
	}
	var faces shape.Faces
	maroon := color.RGBA{127, 0, 0, 255}
	face := faces.For(regular, unit.Sp(72))
	message := "Hello, Gio"
	gtx := &layout.Context{
		Queue: w.Queue(),
	}
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.UpdateEvent:
			gtx.Reset(&e.Config, e.Size)
			faces.Reset(gtx.Config)
			var material op.MacroOp
			material.Record(gtx.Ops)
			paint.ColorOp{Color: maroon}.Add(gtx.Ops)
			material.Stop()
			text.Label{Material: material, Face: face, Alignment: text.Middle, Text: message}.Layout(gtx)
			w.Update(gtx.Ops)
		}
	}
}
