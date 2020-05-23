// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"image/color"
	"image/draw"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"golang.org/x/exp/shiny/iconvg"
)

type Icon struct {
	Color color.RGBA
	src   []byte
	// Cached values.
	op       paint.ImageOp
	imgSize  int
	imgColor color.RGBA
}

// NewIcon returns a new Icon from IconVG data.
func NewIcon(data []byte) (*Icon, error) {
	_, err := iconvg.DecodeMetadata(data)
	if err != nil {
		return nil, err
	}
	return &Icon{src: data, Color: color.RGBA{A: 0xff}}, nil
}

func (ic *Icon) Layout(gtx layout.Context, sz unit.Value) layout.Dimensions {
	ico := ic.image(gtx.Px(sz))
	ico.Add(gtx.Ops)
	paint.PaintOp{
		Rect: f32.Rectangle{
			Max: layout.FPt(ico.Size()),
		},
	}.Add(gtx.Ops)
	return layout.Dimensions{
		Size: ico.Size(),
	}
}

func (ic *Icon) image(sz int) paint.ImageOp {
	if sz == ic.imgSize && ic.Color == ic.imgColor {
		return ic.op
	}
	m, _ := iconvg.DecodeMetadata(ic.src)
	dx, dy := m.ViewBox.AspectRatio()
	img := image.NewRGBA(image.Rectangle{Max: image.Point{X: sz, Y: int(float32(sz) * dy / dx)}})
	var ico iconvg.Rasterizer
	ico.SetDstImage(img, img.Bounds(), draw.Src)
	m.Palette[0] = ic.Color
	iconvg.Decode(&ico, ic.src, &iconvg.DecodeOptions{
		Palette: &m.Palette,
	})
	ic.op = paint.NewImageOp(img)
	ic.imgSize = sz
	ic.imgColor = ic.Color
	return ic.op
}
