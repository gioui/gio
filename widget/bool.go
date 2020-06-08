package widget

import (
	"image"
	"time"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

type Bool struct {
	Value bool
	// Last is the last registered click.
	Last Press

	changed bool

	gesture gesture.Click
}

// Changed reports whether Value has changed since the last
// call to Changed.
func (b *Bool) Changed() bool {
	changed := b.changed
	b.changed = false
	return changed
}

func (b *Bool) Layout(gtx layout.Context) layout.Dimensions {
	for _, e := range b.gesture.Events(gtx) {
		switch e.Type {
		case gesture.TypeClick:
			now := gtx.Now()
			b.Last = Press{
				Start:    now,
				End:      now.Add(time.Second),
				Position: e.Position,
			}
			b.Value = !b.Value
			b.changed = true
		}
	}
	defer op.Push(gtx.Ops).Pop()
	pointer.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Add(gtx.Ops)
	b.gesture.Add(gtx.Ops)
	return layout.Dimensions{Size: gtx.Constraints.Min}
}
