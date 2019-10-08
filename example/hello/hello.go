// SPDX-License-Identifier: Unlicense OR MIT

package main

// A simple Gio program. See https://gioui.org for more information.

import (
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/text/opentype"
	"gioui.org/widget/material"

	"golang.org/x/image/font/gofont/goregular"
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
	shaper := new(text.Shaper)
	shaper.Register(text.Font{}, opentype.Must(
		opentype.Parse(goregular.TTF),
	))
	th := material.NewTheme(shaper)
	gtx := &layout.Context{
		Queue: w.Queue(),
	}
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx.Reset(&e.Config, e.Size)
			l := th.H1("Hello, Gio")
			maroon := color.RGBA{127, 0, 0, 255}
			l.Color = maroon
			l.Alignment = text.Middle
			l.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
