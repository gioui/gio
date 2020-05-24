package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type Bool struct {
	Value bool
	// Last is the last registered click.
	Last Click

	// changeVal tracks Value from the most recent call to Changed.
	changeVal bool

	gesture gesture.Click
}

// Changed reports whether Value has changed since the last
// call to Changed.
func (b *Bool) Changed() bool {
	changed := b.Value != b.changeVal
	b.changeVal = b.Value
	return changed
}

func (b *Bool) Layout(gtx layout.Context) {
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
	b.gesture.Add(gtx.Ops)
}
