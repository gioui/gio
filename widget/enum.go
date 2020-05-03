package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type Enum struct {
	Value string

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

// Update the Value according to incoming events.
func (e *Enum) Update(gtx *layout.Context) {
	for i := range e.clicks {
		for _, ev := range e.clicks[i].Events(gtx) {
			switch ev.Type {
			case gesture.TypeClick:
				e.Value = e.values[i]
			}
		}
	}
}

// Layout adds the event handler for key.
func (e *Enum) Layout(gtx *layout.Context, key string) {
	if index(e.values, key) == -1 {
		e.values = append(e.values, key)
		e.clicks = append(e.clicks, gesture.Click{})
		e.clicks[len(e.clicks)-1].Add(gtx.Ops)
	} else {
		idx := index(e.values, key)
		e.clicks[idx].Add(gtx.Ops)
	}
}
