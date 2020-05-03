package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type CheckBox struct {
	Checked bool

	click gesture.Click
}

// Update the checked state according to incoming events.
func (c *CheckBox) Update(gtx *layout.Context) {
	for _, e := range c.click.Events(gtx) {
		switch e.Type {
		case gesture.TypeClick:
			c.Checked = !c.Checked
		}
	}
}

func (c *CheckBox) Layout(gtx *layout.Context) {
	c.click.Add(gtx.Ops)
}
