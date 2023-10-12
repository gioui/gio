package material_test

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

func TestListAnchorStrategies(t *testing.T) {
	gtx := layout.Context{
		Ops: new(op.Ops),
		Metric: unit.Metric{
			PxPerDp: 1,
			PxPerSp: 1,
		},
		Constraints: layout.Exact(image.Point{
			X: 500,
			Y: 500,
		}),
	}
	gtx.Constraints.Min = image.Point{}

	var spaceConstraints layout.Constraints
	space := func(gtx layout.Context, index int) layout.Dimensions {
		spaceConstraints = gtx.Constraints
		if spaceConstraints.Min.X < 0 || spaceConstraints.Min.Y < 0 ||
			spaceConstraints.Max.X < 0 || spaceConstraints.Max.Y < 0 {
			t.Errorf("invalid constraints at index %d: %#+v", index, spaceConstraints)
		}
		return layout.Dimensions{Size: image.Point{
			X: gtx.Constraints.Max.X,
			Y: gtx.Dp(20),
		}}
	}

	var list widget.List
	list.Axis = layout.Vertical
	elements := 100
	th := material.NewTheme()
	materialList := material.List(th, &list)
	indicatorWidth := gtx.Dp(materialList.Width())

	materialList.AnchorStrategy = material.Occupy
	occupyDims := materialList.Layout(gtx, elements, space)
	occupyConstraints := spaceConstraints

	materialList.AnchorStrategy = material.Overlay
	overlayDims := materialList.Layout(gtx, elements, space)
	overlayConstraints := spaceConstraints

	// Both anchor strategies should use all space available if their elements do.
	if occupyDims != overlayDims {
		t.Errorf("expected occupy dims (%v) to be equal to overlay dims (%v)", occupyDims, overlayDims)
	}
	// The overlay strategy should not reserve any space for the scroll indicator,
	// so the constraints that it presents to its elements should be larger than
	// those presented by the occupy strategy.
	if overlayConstraints.Max.X != occupyConstraints.Max.X+indicatorWidth {
		t.Errorf("overlay max width (%d) != occupy max width (%d) + indicator width (%d)",
			overlayConstraints.Max.X, occupyConstraints.Max.X, indicatorWidth)
	}
}
