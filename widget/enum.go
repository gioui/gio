package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type Enum struct {
	clicks []gesture.Click
	values []string
	value  string
}

func index(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

// Value processes events and returns the last selected value, or
// the empty string.
func (e *Enum) Value(gtx *layout.Context) string {
	for i := range e.clicks {
		for _, ev := range e.clicks[i].Events(gtx) {
			switch ev.Type {
			case gesture.TypeClick:
				e.value = e.values[i]
			}
		}
	}
	return e.value
}

// Layout adds the event handler for key.
func (rg *Enum) Layout(gtx *layout.Context, key string) {
	if index(rg.values, key) == -1 {
		rg.values = append(rg.values, key)
		rg.clicks = append(rg.clicks, gesture.Click{})
		rg.clicks[len(rg.clicks)-1].Add(gtx.Ops)
	} else {
		idx := index(rg.values, key)
		rg.clicks[idx].Add(gtx.Ops)
	}
}

func (rg *Enum) SetValue(value string) {
	rg.value = value
}
