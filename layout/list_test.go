// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/input"
	"gioui.org/io/pointer"
	"gioui.org/op"
)

func TestListPositionExtremes(t *testing.T) {
	var l List
	gtx := Context{
		Ops:         new(op.Ops),
		Constraints: Exact(image.Pt(20, 10)),
	}
	const n = 3
	layout := func(_ Context, idx int) Dimensions {
		if idx < 0 || idx >= n {
			t.Errorf("list index %d out of bounds [0;%d]", idx, n-1)
		}
		return Dimensions{}
	}
	l.Position.First = -1
	l.Layout(gtx, n, layout)
	l.Position.First = n + 1
	l.Layout(gtx, n, layout)
}

func TestEmptyList(t *testing.T) {
	var l List
	gtx := Context{
		Ops:         new(op.Ops),
		Constraints: Exact(image.Pt(20, 10)),
	}
	dims := l.Layout(gtx, 0, nil)
	if got, want := dims.Size, gtx.Constraints.Min; got != want {
		t.Errorf("got %v; want %v", got, want)
	}
}

func TestListScrollToEnd(t *testing.T) {
	l := List{
		ScrollToEnd: true,
	}
	gtx := Context{
		Ops:         new(op.Ops),
		Constraints: Exact(image.Pt(20, 10)),
	}
	l.Layout(gtx, 1, func(gtx Context, idx int) Dimensions {
		return Dimensions{
			Size: image.Pt(10, 10),
		}
	})
	if want, got := -10, l.Position.Offset; want != got {
		t.Errorf("got offset %d, want %d", got, want)
	}
}

func TestListPosition(t *testing.T) {
	_s := func(e ...event.Event) []event.Event { return e }
	r := new(input.Router)
	gtx := Context{
		Ops: new(op.Ops),
		Constraints: Constraints{
			Max: image.Pt(20, 10),
		},
		Source: r.Source(),
	}
	el := func(gtx Context, idx int) Dimensions {
		return Dimensions{Size: image.Pt(10, 10)}
	}
	for _, tc := range []struct {
		label  string
		num    int
		scroll []event.Event
		first  int
		count  int
		offset int
		last   int
	}{
		{label: "no item", last: 20},
		{label: "1 visible 0 hidden", num: 1, count: 1, last: 10},
		{label: "2 visible 0 hidden", num: 2, count: 2},
		{label: "2 visible 1 hidden", num: 3, count: 2},
		{label: "3 visible 0 hidden small scroll", num: 3, count: 3, offset: 5, last: -5,
			scroll: _s(
				pointer.Event{
					Source:   pointer.Mouse,
					Buttons:  pointer.ButtonPrimary,
					Kind:     pointer.Press,
					Position: f32.Pt(0, 0),
				},
				pointer.Event{
					Source: pointer.Mouse,
					Kind:   pointer.Scroll,
					Scroll: f32.Pt(5, 0),
				},
				pointer.Event{
					Source:   pointer.Mouse,
					Buttons:  pointer.ButtonPrimary,
					Kind:     pointer.Release,
					Position: f32.Pt(5, 0),
				},
			)},
		{label: "3 visible 0 hidden small scroll 2", num: 3, count: 3, offset: 3, last: -7,
			scroll: _s(
				pointer.Event{
					Source:   pointer.Mouse,
					Buttons:  pointer.ButtonPrimary,
					Kind:     pointer.Press,
					Position: f32.Pt(0, 0),
				},
				pointer.Event{
					Source: pointer.Mouse,
					Kind:   pointer.Scroll,
					Scroll: f32.Pt(3, 0),
				},
				pointer.Event{
					Source:   pointer.Mouse,
					Buttons:  pointer.ButtonPrimary,
					Kind:     pointer.Release,
					Position: f32.Pt(5, 0),
				},
			)},
		{label: "2 visible 1 hidden large scroll", num: 3, count: 2, first: 1,
			scroll: _s(
				pointer.Event{
					Source:   pointer.Mouse,
					Buttons:  pointer.ButtonPrimary,
					Kind:     pointer.Press,
					Position: f32.Pt(0, 0),
				},
				pointer.Event{
					Source: pointer.Mouse,
					Kind:   pointer.Scroll,
					Scroll: f32.Pt(10, 0),
				},
				pointer.Event{
					Source:   pointer.Mouse,
					Buttons:  pointer.ButtonPrimary,
					Kind:     pointer.Release,
					Position: f32.Pt(15, 0),
				},
			)},
	} {
		t.Run(tc.label, func(t *testing.T) {
			gtx.Ops.Reset()

			var list List
			// Initialize the list.
			list.Layout(gtx, tc.num, el)
			// Generate the scroll events.
			r.Frame(gtx.Ops)
			r.Queue(tc.scroll...)
			// Let the list process the events.
			list.Layout(gtx, tc.num, el)

			pos := list.Position
			if got, want := pos.First, tc.first; got != want {
				t.Errorf("List: invalid first position: got %v; want %v", got, want)
			}
			if got, want := pos.Count, tc.count; got != want {
				t.Errorf("List: invalid number of visible children: got %v; want %v", got, want)
			}
			if got, want := pos.Offset, tc.offset; got != want {
				t.Errorf("List: invalid first visible offset: got %v; want %v", got, want)
			}
			if got, want := pos.OffsetLast, tc.last; got != want {
				t.Errorf("List: invalid last visible offset: got %v; want %v", got, want)
			}
		})
	}
}

func TestExtraChildren(t *testing.T) {
	var l List
	l.Position.First = 1
	gtx := Context{
		Ops:         new(op.Ops),
		Constraints: Exact(image.Pt(10, 10)),
	}
	count := 0
	const all = 3
	l.Layout(gtx, all, func(gtx Context, idx int) Dimensions {
		count++
		return Dimensions{Size: image.Pt(10, 10)}
	})
	if count != all {
		t.Errorf("laid out %d of %d children", count, all)
	}
}
