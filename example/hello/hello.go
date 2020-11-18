// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"image/color"
	"log"
	"os"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/widget/material"

	"gioui.org/font/gofont"
)

func main() {
	go func() {
		w := app.NewWindow()
		if err := loop(w); err != nil {
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}

func loop(w *app.Window) error {
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			l := material.H1(th, "Hello, Gio")
			maroon := color.NRGBA{R: 127, G: 0, B: 0, A: 255}
			l.Color = maroon
			l.Alignment = text.Middle
			l.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
