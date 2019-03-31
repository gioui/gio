// SPDX-License-Identifier: Unlicense OR MIT

package main

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
	app.Main()
}

func init() {
	regular, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		panic("failed to load font")
	}
	var faces measure.Faces
	black := &image.Uniform{color.Black}
	face := faces.For(regular, ui.Dp(50))
	go func() {
		w, err := app.NewWindow(app.WindowOptions{
			Width:  ui.Dp(400),
			Height: ui.Dp(800),
			Title:  "Hello World",
		})
		if err != nil {
			log.Fatal(err)
		}
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
			w.Ack()
		}
		if w.Err() != nil {
			log.Fatal(err)
		}
	}()
}
