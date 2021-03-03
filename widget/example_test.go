// SPDX-License-Identifier: Unlicense OR MIT

package widget_test

import (
	"fmt"
	"image"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/io/router"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

func ExampleClickable_passthrough() {
	// When laying out clickable widgets on top of each other,
	// pointer events can be passed down for the underlying
	// widgets to pick them up.
	var button1, button2 widget.Clickable
	var r router.Router
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Queue:       &r,
	}

	// widget lays out two buttons on top of each other.
	widget := func() {
		// button2 completely covers button1, but PassOp allows pointer
		// events to pass through to button1.
		button1.Layout(gtx)
		// PassOp is applied to the area defined by button1.
		pointer.PassOp{Pass: true}.Add(gtx.Ops)
		button2.Layout(gtx)
	}

	// The first layout and call to Frame declare the Clickable handlers
	// to the input router, so the following pointer events are propagated.
	widget()
	r.Frame(gtx.Ops)
	// Simulate one click on the buttons by sending a Press and Release event.
	r.Queue(
		pointer.Event{
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Type:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
		pointer.Event{
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Type:     pointer.Release,
			Position: f32.Pt(50, 50),
		},
	)
	// The second layout ensures that the click event is registered by the buttons.
	widget()

	if button1.Clicked() {
		fmt.Println("button1 clicked!")
	}
	if button2.Clicked() {
		fmt.Println("button2 clicked!")
	}

	// Output:
	// button1 clicked!
	// button2 clicked!
}
