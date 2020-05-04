package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type Bool struct {
	Value bool

	click gesture.Click
}

// Update the checked state according to incoming events.
func (b *Bool) Update(gtx *layout.Context) {
	for _, e := range b.click.Events(gtx) {
		switch e.Type {
		case gesture.TypeClick:
			b.Value = !b.Value
		}
	}
}

func (b *Bool) Layout(gtx *layout.Context) {
	b.click.Add(gtx.Ops)
}
