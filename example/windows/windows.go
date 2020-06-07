// SPDX-License-Identifier: Unlicense OR MIT

package main

// Multiple windows in Gio.

import (
	"log"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gioui.org/font/gofont"
)

type window struct {
	btn widget.Clickable
}

func main() {
	newWindow()
	app.Main()
}

func newWindow() {
	go func() {
		w := new(window)
		evts := app.NewWindow().Events()
		if err := w.loop(evts); err != nil {
			log.Fatal(err)
		}
	}()
}

func (w *window) loop(events <-chan event.Event) error {
	th := material.NewTheme(gofont.Collection())
	var ops op.Ops
	for {
		e := <-events
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			for w.btn.Clicked() {
				newWindow()
			}
			gtx := layout.NewContext(&ops, e.Queue, e.Config, e.Size)
			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return material.Button(th, &w.btn, "More!").Layout(gtx)
			})
			e.Frame(gtx.Ops)
		}
	}
}
