package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type Bool struct {
	Value bool

	// Last is the last registered click.
	Last Click

	gesture gesture.Click
}

// Update the checked state according to incoming events.
func (b *Bool) Update(gtx *layout.Context) {
	for _, e := range b.gesture.Events(gtx) {
		switch e.Type {
		case gesture.TypeClick:
			b.Last = Click{
				Time:     gtx.Now(),
				Position: e.Position,
			}
			b.Value = !b.Value
		}
	}
}

func (b *Bool) Layout(gtx *layout.Context) {
	b.gesture.Add(gtx.Ops)
}
