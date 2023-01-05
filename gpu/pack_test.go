// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"image"
	"testing"
)

func BenchmarkPacker(b *testing.B) {
	var p packer
	p.maxDims = image.Point{X: 4096, Y: 4096}
	for i := 0; i < b.N; i++ {
		p.clear()
		p.newPage()
		for k := 0; k < 500; k++ {
			_, ok := p.tryAdd(xy(k))
			if !ok {
				b.Fatal("add failed", i, k, xy(k))
			}
		}
	}
}

func xy(v int) image.Point {
	return image.Point{
		X: ((v / 16) % 16) + 8,
		Y: (v % 16) + 8,
	}
}
