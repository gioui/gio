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
	// frame: request focus for button 1
	gtx.Execute(key.FocusCmd{Tag: &b1})
	frame()
	// frame: gain focus for button 1
	frame()
	if !b1.Focused() {
		t.Error("button 1 did not gain focus")
	}
	if b2.Focused() {
		t.Error("button 2 should not have focus")
	}
	// frame: press & release return
	frame()
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
	// frame: press return down
	r.Queue(
		key.Event{
			Name:  key.NameReturn,
			State: key.Press,
		},
	)
	frame()
	if b1.Clicked(gtx) {
		t.Error("button 1 got clicked, even if it only got return press")
	}
	// frame: request focus for button 2
	gtx.Execute(key.FocusCmd{Tag: &b2})
	frame()
	// frame: gain focus for button 2
	frame()
	if b1.Focused() {
		t.Error("button 1 should not have focus")
	}
	if !b2.Focused() {
		t.Error("button 2 did not gain focus")
	}
	// frame: release return
	r.Queue(
		key.Event{
			Name:  key.NameReturn,
			State: key.Release,
		},
	)
	frame()
	if b1.Clicked(gtx) {
		t.Error("button 1 got clicked, even if it had lost focus")
	}
	if b2.Clicked(gtx) {
		t.Error("button 2 should not have been clicked, as it only got return release")
	}
}
