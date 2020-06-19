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
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"gioui.org/font/gofont"
)

type window struct {
	*app.Window

	more  widget.Clickable
	close widget.Clickable
}

func main() {
	newWindow()
	app.Main()
}

func newWindow() {
	go func() {
		w := new(window)
		w.Window = app.NewWindow()
		if err := w.loop(w.Events()); err != nil {
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
			for w.more.Clicked() {
				newWindow()
			}
			for w.close.Clicked() {
				w.Close()
			}
			gtx := layout.NewContext(&ops, e)

			layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{
					Alignment: layout.Middle,
				}.Layout(gtx,
					RigidInset(material.Button(th, &w.more, "More!").Layout),
					RigidInset(material.Button(th, &w.close, "Close").Layout),
				)
			})
			e.Frame(gtx.Ops)
		}
	}
}

func RigidInset(w layout.Widget) layout.FlexChild {
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(unit.Sp(5)).Layout(gtx, w)
	})
}
