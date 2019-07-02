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
	wopt := app.WindowOptions{Width: ui.Px(612), Height: ui.Px(792), Title: "Hello"}
	if err := app.CreateWindow(&wopt); err != nil {
		log.Fatal(err)
	}
	app.Main()
}

// On iOS and Android main will never be called, so
// setting up the window must run in an init function.
func init() {
	go func() {
		for w := range app.Windows() {
			go loop(w)
		}
	}()
}

func loop(w *app.Window) {
	regular, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		panic("failed to load font")
	}
	var cfg ui.Config
	faces := &measure.Faces{Config: &cfg}
	maroon := color.RGBA{127, 0, 0, 255}
	face := faces.For(regular, ui.Sp(72))
	message := "Hello, Gio"
	ops := new(ui.Ops)
	for w.IsAlive() {
		e := <-w.Events()
		switch e := e.(type) {
		case app.DrawEvent:
			cfg = e.Config
			cs := layout.ExactConstraints(w.Size())
			ops.Reset()
			draw.ColorOp{Col: maroon}.Add(ops)
			text.Label{Face: face, Alignment: text.Center, Text: message}.Layout(ops, cs)
			w.Draw(ops)
			faces.Frame()
		}
	}
	if err := w.Err(); err != nil {
		log.Fatal(err)
	}
}
