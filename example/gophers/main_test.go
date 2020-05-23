// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"image"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
)

func BenchmarkUI(b *testing.B) {
	fetch := func(_ string) {}
	u := newUI(fetch)
	var ops op.Ops
	for i := 0; i < b.N; i++ {
		gtx := layout.Context{
			Ops:         &ops,
			Constraints: layout.Exact(image.Pt(800, 600)),
		}
		u.Layout(gtx)
	}
}
