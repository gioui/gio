// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

func TestIcon_Alpha(t *testing.T) {
	icon, err := NewIcon(icons.ToggleCheckBox)
	if err != nil {
		t.Fatal(err)
	}

	icon.Color = color.NRGBA{B: 0xff, A: 0x40}

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
	}

	_ = icon.Layout(gtx, unit.Sp(18))
}
