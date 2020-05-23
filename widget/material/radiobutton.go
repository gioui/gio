// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget"
)

type RadioButtonStyle struct {
	checkable
	Key string
}

// RadioButton returns a RadioButton with a label. The key specifies
// the value for the Enum.
func RadioButton(th *Theme, key, label string) RadioButtonStyle {
	return RadioButtonStyle{
		checkable: checkable{
			Label: label,

			Color:              th.Color.Text,
			IconColor:          th.Color.Primary,
			TextSize:           th.TextSize.Scale(14.0 / 16.0),
			Size:               unit.Dp(26),
			shaper:             th.Shaper,
			checkedStateIcon:   th.radioCheckedIcon,
			uncheckedStateIcon: th.radioUncheckedIcon,
		},
		Key: key,
	}
}

// Layout updates enum and displays the radio button.
func (r RadioButtonStyle) Layout(gtx layout.Context, enum *widget.Enum) layout.Dimensions {
	enum.Update(gtx)
	dims := r.layout(gtx, enum.Value == r.Key)
	enum.Layout(gtx, r.Key)
	return dims
}
