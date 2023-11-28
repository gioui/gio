// SPDX-License-Identifier: Unlicense OR MIT

package widget_test

import (
	"fmt"
	"image"
	"io"
	"strings"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/io/transfer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
)

func ExampleClickable_passthrough() {
	// When laying out clickable widgets on top of each other,
	// pointer events can be passed down for the underlying
	// widgets to pick them up.
	var button1, button2 widget.Clickable
	var r input.Router
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Source:      r.Source(),
	}

	// widget lays out two buttons on top of each other.
	widget := func() {
		content := func(gtx layout.Context) layout.Dimensions { return layout.Dimensions{Size: gtx.Constraints.Min} }
		button1.Layout(gtx, content)
		// button2 completely covers button1, but pass-through allows pointer
		// events to pass through to button1.
		defer pointer.PassOp{}.Push(gtx.Ops).Pop()
		button2.Layout(gtx, content)
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
			Kind:     pointer.Press,
			Position: f32.Pt(50, 50),
		},
		pointer.Event{
			Source:   pointer.Mouse,
			Buttons:  pointer.ButtonPrimary,
			Kind:     pointer.Release,
			Position: f32.Pt(50, 50),
		},
	)

	if button1.Clicked(gtx) {
		fmt.Println("button1 clicked!")
	}
	if button2.Clicked(gtx) {
		fmt.Println("button2 clicked!")
	}

	// Output:
	// button1 clicked!
	// button2 clicked!
}

func ExampleDraggable_Layout() {
	var r input.Router
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
		Source:      r.Source(),
	}
	// mime is the type used to match drag and drop operations.
	// It could be left empty in this example.
	const mime = "MyMime"
	drag := &widget.Draggable{Type: mime}
	var drop int
	// widget lays out the drag and drop handlers and processes
	// the transfer events.
	widget := func() {
		// Setup the draggable widget.
		w := func(gtx layout.Context) layout.Dimensions {
			sz := image.Pt(10, 10) // drag area
			return layout.Dimensions{Size: sz}
		}
		drag.Layout(gtx, w, w)
		// drag must respond with an Offer event when requested.
		// Use the drag method for this.
		if m, ok := drag.Update(gtx); ok {
			drag.Offer(gtx, m, io.NopCloser(strings.NewReader("hello world")))
		}

		// Setup the area for drops.
		ds := clip.Rect{
			Min: image.Pt(20, 20),
			Max: image.Pt(40, 40),
		}.Push(gtx.Ops)
		event.Op(gtx.Ops, &drop)
		ds.Pop()

		// Check for the received data.
		for {
			ev, ok := gtx.Event(transfer.TargetFilter{Target: &drop, Type: mime})
			if !ok {
				break
			}
			switch e := ev.(type) {
			case transfer.DataEvent:
				data := e.Open()
				defer data.Close()
				content, _ := io.ReadAll(data)
				fmt.Println(string(content))
			}
		}
	}
	// Register and lay out the widget.
	widget()
	r.Frame(gtx.Ops)

	// Send drag and drop gesture events.
	r.Queue(
		pointer.Event{
			Kind:     pointer.Press,
			Position: f32.Pt(5, 5), // in the drag area
		},
		pointer.Event{
			Kind:     pointer.Move,
			Position: f32.Pt(5, 5), // in the drop area
		},
		pointer.Event{
			Kind:     pointer.Release,
			Position: f32.Pt(30, 30), // in the drop area
		},
	)
	// Let the widget process the events.
	widget()
	r.Frame(gtx.Ops)

	// Process the transfer.DataEvent.
	widget()

	// Output:
	// hello world
}
