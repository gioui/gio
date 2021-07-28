// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"image/color"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"

	"golang.org/x/exp/shiny/materialdesign/icons"
)

func TestIcon_Alpha(t *testing.T) {
	icon, err := NewIcon(icons.ToggleCheckBox)
	if err != nil {
		t.Fatal(err)
	}

	col := color.NRGBA{B: 0xff, A: 0x40}

	gtx := layout.Context{
		Ops:         new(op.Ops),
		Constraints: layout.Exact(image.Pt(100, 100)),
	}

	_ = icon.Layout(gtx, col)
}

// TestWidgetConstraints tests that widgets returns dimensions within their constraints.
func TestWidgetConstraints(t *testing.T) {
	_cs := func(v ...layout.Constraints) []layout.Constraints { return v }
	for _, tc := range []struct {
		label       string
		widget      layout.Widget
		constraints []layout.Constraints
	}{
		{
			label: "Icon",
			widget: func(gtx layout.Context) layout.Dimensions {
				ic, _ := NewIcon(icons.ToggleCheckBox)
				return ic.Layout(gtx, color.NRGBA{A: 0xff})
			},
			constraints: _cs(
				layout.Constraints{
					Min: image.Pt(20, 0),
					Max: image.Pt(100, 100),
				},
				layout.Constraints{
					Max: image.Pt(100, 100),
				},
			),
		},
	} {
		t.Run(tc.label, func(t *testing.T) {
			for _, cs := range tc.constraints {
				gtx := layout.Context{
					Constraints: cs,
					Ops:         new(op.Ops),
				}
				dims := tc.widget(gtx)
				csr := image.Rectangle{
					Min: cs.Min,
					Max: cs.Max,
				}
				if !dims.Size.In(csr) {
					t.Errorf("dims size %v not within constraints %v", dims.Size, csr)
				}
			}
		})
	}
}
