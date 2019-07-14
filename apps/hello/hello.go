// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"image/color"
	"log"

	"gioui.org/ui"
	"gioui.org/ui/app"
	"gioui.org/ui/draw"
	"gioui.org/ui/layout"
	"gioui.org/ui/measure"
	"gioui.org/ui/text"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

func main() {
	app.Main()
}

// On iOS and Android main will never be called, so
// setting up the window must run in an init function.
func init() {
	go func() {
		w := app.NewWindow(nil)
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
	}()
}

func loop(w *app.Window) error {
	regular, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		panic("failed to load font")
	}
	var cfg app.Config
	faces := &measure.Faces{Config: &cfg}
	maroon := color.RGBA{127, 0, 0, 255}
	face := faces.For(regular, ui.Sp(72))
	message := "Hello, Gio"
	ops := new(ui.Ops)
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.DrawEvent:
			cfg = e.Config
			cs := layout.ExactConstraints(e.Size)
			ops.Reset()
			ops.Begin()
			draw.ColorOp{Color: maroon}.Add(ops)
			material := ops.End()
			text.Label{Material: material, Face: face, Alignment: text.Center, Text: message}.Layout(ops, cs)
			w.Draw(ops)
			faces.Frame()
		}
	}
}
