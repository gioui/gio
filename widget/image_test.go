// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
)

func TestImageScale(t *testing.T) {
	var ops op.Ops
	gtx := layout.Context{
		Ops: &ops,
		Constraints: layout.Constraints{
			Max: image.Pt(50, 50),
		},
	}
	imgSize := image.Pt(10, 10)
	img := image.NewNRGBA(image.Rectangle{Max: imgSize})
	imgOp := paint.NewImageOp(img)

	// Ensure the default scales correctly.
	dims := Image{Src: imgOp}.Layout(gtx)
	expectedSize := imgSize
	expectedSize.X = int(float32(expectedSize.X))
	expectedSize.Y = int(float32(expectedSize.Y))
	if dims.Size != expectedSize {
		t.Fatalf("non-scaled image is wrong size, expected %v, got %v", expectedSize, dims.Size)
	}

	// Ensure scaling the image via the Scale field works.
	currentScale := float32(0.5)
	dims = Image{Src: imgOp, Scale: float32(currentScale)}.Layout(gtx)
	expectedSize = imgSize
	expectedSize.X = int(float32(expectedSize.X) * currentScale)
	expectedSize.Y = int(float32(expectedSize.Y) * currentScale)
	if dims.Size != expectedSize {
		t.Fatalf(".5 scale image is wrong size, expected %v, got %v", expectedSize, dims.Size)
	}

	// Ensure the image responds to changes in DPI.
	currentScale = float32(1)
	gtx.Metric.PxPerDp = 2
	dims = Image{Src: imgOp, Scale: float32(currentScale)}.Layout(gtx)
	expectedSize = imgSize
	expectedSize.X = int(float32(expectedSize.X) * currentScale * gtx.Metric.PxPerDp)
	expectedSize.Y = int(float32(expectedSize.Y) * currentScale * gtx.Metric.PxPerDp)
	if dims.Size != expectedSize {
		t.Fatalf("HiDPI non-scaled image is wrong size, expected %v, got %v", expectedSize, dims.Size)
	}

	// Ensure scaling the image responds to changes in DPI.
	currentScale = float32(.5)
	gtx.Metric.PxPerDp = 2
	dims = Image{Src: imgOp, Scale: float32(currentScale)}.Layout(gtx)
	expectedSize = imgSize
	expectedSize.X = int(float32(expectedSize.X) * currentScale * gtx.Metric.PxPerDp)
	expectedSize.Y = int(float32(expectedSize.Y) * currentScale * gtx.Metric.PxPerDp)
	if dims.Size != expectedSize {
		t.Fatalf("HiDPI .5 scale image is wrong size, expected %v, got %v", expectedSize, dims.Size)
	}
}
