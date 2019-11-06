// SPDX-License-Identifier: Unlicense OR MIT

package material

import (
	"gioui.org/layout"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
)

type RadioButton struct {
	checkable
	Key string
}

// RadioButton returns a RadioButton with a label. The key specifies
// the value for the Enum.
func (t *Theme) RadioButton(key, label string) RadioButton {
	return RadioButton{
		checkable: checkable{
			Label: label,

			Color:     t.Color.Text,
			IconColor: t.Color.Primary,
			Font: text.Font{
				Size: t.TextSize.Scale(14.0 / 16.0),
			},
			Size:               unit.Dp(26),
			shaper:             t.Shaper,
			checkedStateIcon:   t.radioCheckedIcon,
			uncheckedStateIcon: t.radioUncheckedIcon,
		},
		Key: key,
	}
}

func (r RadioButton) Layout(gtx *layout.Context, enum *widget.Enum) {
	r.layout(gtx, enum.Value(gtx) == r.Key)
	enum.Layout(gtx, r.Key)
}
