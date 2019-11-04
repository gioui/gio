package widget

import (
	"gioui.org/gesture"
	"gioui.org/layout"
)

type CheckBox struct {
	click   gesture.Click
	checked bool
}

func (c *CheckBox) SetChecked(value bool) {
	c.checked = value
}

func (c *CheckBox) Checked(gtx *layout.Context) bool {
	for _, e := range c.click.Events(gtx) {
		switch e.Type {
		case gesture.TypeClick:
			c.checked = !c.checked
		}
	}
	return c.checked
}

func (c *CheckBox) Layout(gtx *layout.Context) {
	c.click.Add(gtx.Ops)
}
