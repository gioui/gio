// SPDX-License-Identifier: Unlicense OR MIT

package widget_test

import (
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
)

func TestClickable(t *testing.T) {
	var (
		r  input.Router
		b1 widget.Clickable
		b2 widget.Clickable
	)
	gtx := layout.Context{
		Ops:    new(op.Ops),
		Source: r.Source(),
	}
	layout := func() {
		b1.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(100, 100)}
		})
		// buttons are on top of each other but we only use focus and keyevents, so this is fine
		b2.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: image.Pt(100, 100)}
		})
	}
	frame := func() {
		gtx.Reset()
		layout()
		r.Frame(gtx.Ops)
	}
	gtx.Execute(key.FocusCmd{Tag: &b1})
	frame()
	if !gtx.Focused(&b1) {
		t.Error("button 1 did not gain focus")
	}
	if gtx.Focused(&b2) {
		t.Error("button 2 should not have focus")
	}
	r.Queue(
		key.Event{
			Name:  key.NameReturn,
			State: key.Press,
		},
		key.Event{
			Name:  key.NameReturn,
			State: key.Release,
		},
	)
	if !b1.Clicked(gtx) {
		t.Error("button 1 did not get clicked when it got return press & release")
	}
	if b2.Clicked(gtx) {
		t.Error("button 2 got clicked when it did not have focus")
	}
	r.Queue(
		key.Event{
			Name:  key.NameReturn,
			State: key.Press,
		},
	)
	if b1.Clicked(gtx) {
		t.Error("button 1 got clicked, even if it only got return press")
	}
	frame()
	gtx.Execute(key.FocusCmd{Tag: &b2})
	frame()
	if gtx.Focused(&b1) {
		t.Error("button 1 should not have focus")
	}
	if !gtx.Focused(&b2) {
		t.Error("button 2 did not gain focus")
	}
	r.Queue(
		key.Event{
			Name:  key.NameReturn,
			State: key.Release,
		},
	)
	if b1.Clicked(gtx) {
		t.Error("button 1 got clicked, even if it had lost focus")
	}
	if b2.Clicked(gtx) {
		t.Error("button 2 should not have been clicked, as it only got return release")
	}
}
