// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"gioui.org/io/semantic"
	"gioui.org/layout"
)

type Bool struct {
	Value bool

	clk Clickable
}

// Update the widget state and report whether Value was changed.
func (b *Bool) Update(gtx layout.Context) bool {
	changed := false
	for b.clk.clicked(b, gtx) {
		b.Value = !b.Value
		changed = true
	}
	return changed
}

// Hovered reports whether pointer is over the element.
func (b *Bool) Hovered() bool {
	return b.clk.Hovered()
}

// Pressed reports whether pointer is pressing the element.
func (b *Bool) Pressed() bool {
	return b.clk.Pressed()
}

func (b *Bool) History() []Press {
	return b.clk.History()
}

func (b *Bool) Layout(gtx layout.Context, w layout.Widget) layout.Dimensions {
	b.Update(gtx)
	dims := b.clk.layout(b, gtx, func(gtx layout.Context) layout.Dimensions {
		semantic.SelectedOp(b.Value).Add(gtx.Ops)
		semantic.EnabledOp(gtx.Enabled()).Add(gtx.Ops)
		return w(gtx)
	})
	return dims
}
