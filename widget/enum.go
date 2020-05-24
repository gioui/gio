package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type Enum struct {
	Value string

	changeVal string

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

// Changed reports whether Value has changed since the last
// call to Changed.
func (e *Enum) Changed() bool {
	changed := e.changeVal != e.Value
	e.changeVal = e.Value
	return changed
}

// Layout adds the event handler for key.
func (e *Enum) Layout(gtx layout.Context, key string) {
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
				e.Value = e.values[idx]
			}
		}
		clk.Add(gtx.Ops)
	}
}
