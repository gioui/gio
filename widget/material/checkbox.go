// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

type CheckBoxStyle struct {
	checkable
	CheckBox *widget.Bool
}

func CheckBox(th *Theme, checkBox *widget.Bool, label string) CheckBoxStyle {
	return CheckBoxStyle{
		CheckBox: checkBox,
		checkable: checkable{
			Label:              label,
			Color:              th.Color.Text,
			IconColor:          th.Color.Primary,
			TextSize:           th.TextSize.Scale(14.0 / 16.0),
			Size:               unit.Dp(26),
			shaper:             th.Shaper,
			checkedStateIcon:   th.Icon.CheckBoxChecked,
			uncheckedStateIcon: th.Icon.CheckBoxUnchecked,
		},
	}
}

// Layout updates the checkBox and displays it.
func (c CheckBoxStyle) Layout(gtx layout.Context) layout.Dimensions {
	dims := c.layout(gtx, c.CheckBox.Value)
	gtx.Constraints.Min = dims.Size
	c.CheckBox.Layout(gtx)
	return dims
}
