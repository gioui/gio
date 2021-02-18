// SPDX-License-Identifier: Unlicense OR MIT

// A simple app used for gogio's end-to-end tests.
package main

import (
	"fmt"
	"image"
	"image/color"
	"log"

	"gioui.org/app"
	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
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

type notifyFrame int

const (
	notifyNone notifyFrame = iota
	notifyInvalidate
	notifyPrint
)

// notify keeps track of whether we want to print to stdout to notify the user
// when a frame is ready. Initially we want to notify about the first frame.
var notify = notifyInvalidate

type (
	C = layout.Context
	D = layout.Dimensions
)

func loop(w *app.Window) error {
	topLeft := quarterWidget{
		color: color.NRGBA{R: 0xde, G: 0xad, B: 0xbe, A: 0xff},
	}
	topRight := quarterWidget{
		color: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
	}
	botLeft := quarterWidget{
		color: color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff},
	}
	botRight := quarterWidget{
		color: color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0x80},
	}

	var ops op.Ops
	for {
		e := <-w.Events()
		switch e := e.(type) {
		case system.DestroyEvent:
			return e.Err
		case system.FrameEvent:
			gtx := layout.NewContext(&ops, e)
			// Clear background to white, even on embedded platforms such as webassembly.
			paint.Fill(gtx.Ops, color.NRGBA{A: 0xff, R: 0xff, G: 0xff, B: 0xff})
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						// r1c1
						layout.Flexed(1, func(gtx C) D { return topLeft.Layout(gtx) }),
						// r1c2
						layout.Flexed(1, func(gtx C) D { return topRight.Layout(gtx) }),
					)
				}),
				layout.Flexed(1, func(gtx C) D {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						// r2c1
						layout.Flexed(1, func(gtx C) D { return botLeft.Layout(gtx) }),
						// r2c2
						layout.Flexed(1, func(gtx C) D { return botRight.Layout(gtx) }),
					)
				}),
			)

			e.Frame(gtx.Ops)

			switch notify {
			case notifyInvalidate:
				notify = notifyPrint
				w.Invalidate()
			case notifyPrint:
				notify = notifyNone
				fmt.Println("gio frame ready")
			}
		}
	}
}

// quarterWidget paints a quarter of the screen with one color. When clicked, it
// turns red, going back to its normal color when clicked again.
type quarterWidget struct {
	color color.NRGBA

	clicked bool
}

var red = color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}

func (w *quarterWidget) Layout(gtx layout.Context) layout.Dimensions {
	var color color.NRGBA
	if w.clicked {
		color = red
	} else {
		color = w.color
	}

	r := image.Rectangle{Max: gtx.Constraints.Max}
	paint.FillShape(gtx.Ops, color, clip.Rect(r).Op())

	pointer.Rect(image.Rectangle{
		Max: image.Pt(gtx.Constraints.Max.X, gtx.Constraints.Max.Y),
	}).Add(gtx.Ops)
	pointer.InputOp{
		Tag:   w,
		Types: pointer.Press,
	}.Add(gtx.Ops)

	for _, e := range gtx.Events(w) {
		if e, ok := e.(pointer.Event); ok && e.Type == pointer.Press {
			w.clicked = !w.clicked
			// notify when we're done updating the frame.
			notify = notifyInvalidate
		}
	}
	return layout.Dimensions{Size: gtx.Constraints.Max}
}
