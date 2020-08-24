package widget

import (
	"image"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
)

type Enum struct {
	Value string

	changed bool

	clicks []gesture.Click
	values []string
}

func index(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

// Changed reports whether Value has changed by user interactino since the last
// call to Changed.
func (e *Enum) Changed() bool {
	changed := e.changed
	e.changed = false
	return changed
}

// Layout adds the event handler for key.
func (e *Enum) Layout(gtx layout.Context, key string) layout.Dimensions {
	defer op.Push(gtx.Ops).Pop()
	pointer.Rect(image.Rectangle{Max: gtx.Constraints.Min}).Add(gtx.Ops)

	if index(e.values, key) == -1 {
		e.values = append(e.values, key)
		e.clicks = append(e.clicks, gesture.Click{})
		e.clicks[len(e.clicks)-1].Add(gtx.Ops)
	} else {
		idx := index(e.values, key)
		clk := &e.clicks[idx]
		for _, ev := range clk.Events(gtx) {
			switch ev.Type {
			case gesture.TypeClick:
				if new := e.values[idx]; new != e.Value {
					e.Value = new
					e.changed = true
				}
			}
		}
		clk.Add(gtx.Ops)
	}

	return layout.Dimensions{Size: gtx.Constraints.Min}
}
