// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"image"
	"image/color"
	"log"

	"gioui.org/ui"
	"gioui.org/ui/app"
	"gioui.org/ui/layout"
	"gioui.org/ui/measure"
	"gioui.org/ui/text"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

func main() {
	err := app.CreateWindow(nil)
	if err != nil {
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
	var faces measure.Faces
	black := &image.Uniform{color.Black}
	face := faces.For(regular, ui.Dp(50))
	for w.IsAlive() {
		e := <-w.Events()
		switch e := e.(type) {
		case app.Draw:
			faces.Cfg = e.Config
			cs := layout.ExactConstraints(w.Size())
			root, _ := (text.Label{Src: black, Face: face, Text: "Hello, World!"}).Layout(cs)
			w.Draw(root)
			faces.Frame()
		}
	}
	if w.Err() != nil {
		log.Fatal(err)
	}
}
