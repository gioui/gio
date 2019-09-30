// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"image/color"
	"log"

	"gioui.org/ui"
	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/measure"
	"gioui.org/paint"
	"gioui.org/text"

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
	var faces measure.Faces
	maroon := color.RGBA{127, 0, 0, 255}
	face := faces.For(regular, ui.Sp(72))
	message := "Hello, Gio"
	c := &layout.Context{
		Queue: w.Queue(),
	}
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.UpdateEvent:
			c.Reset(&e.Config, layout.RigidConstraints(e.Size))
			faces.Reset(c.Config)
			var material ui.MacroOp
			material.Record(c.Ops)
			paint.ColorOp{Color: maroon}.Add(c.Ops)
			material.Stop()
			text.Label{Material: material, Face: face, Alignment: text.Middle, Text: message}.Layout(c)
			w.Update(c.Ops)
		}
	}
}
