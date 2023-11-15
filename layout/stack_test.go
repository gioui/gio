// SPDX-License-Identifier: Unlicense OR MIT

package layout

import (
	"image"
	"testing"

	"gioui.org/op"
)

func BenchmarkStack(b *testing.B) {
	gtx := Context{
		Ops: new(op.Ops),
		Constraints: Constraints{
			Max: image.Point{X: 100, Y: 100},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gtx.Ops.Reset()

		Stack{}.Layout(gtx,
			Expanded(emptyWidget{
				Size: image.Point{X: 60, Y: 60},
			}.Layout),
			Stacked(emptyWidget{
				Size: image.Point{X: 30, Y: 30},
			}.Layout),
		)
	}
}

func BenchmarkBackground(b *testing.B) {
	gtx := Context{
		Ops: new(op.Ops),
		Constraints: Constraints{
			Max: image.Point{X: 100, Y: 100},
		},
	}
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		gtx.Ops.Reset()

		Background{}.Layout(gtx,
			emptyWidget{
				Size: image.Point{X: 60, Y: 60},
			}.Layout,
			emptyWidget{
				Size: image.Point{X: 30, Y: 30},
			}.Layout,
		)
	}
}

type emptyWidget struct {
	Size image.Point
}

func (w emptyWidget) Layout(gtx Context) Dimensions {
	return Dimensions{Size: w.Size}
}
