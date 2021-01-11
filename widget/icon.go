// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/exp/shiny/iconvg"

	"gioui.org/internal/f32color"
	"gioui.org/layout"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

type Icon struct {
	Color color.NRGBA
	src   []byte
	// Cached values.
	op       paint.ImageOp
	imgSize  int
	imgColor color.NRGBA
}

// NewIcon returns a new Icon from IconVG data.
func NewIcon(data []byte) (*Icon, error) {
	_, err := iconvg.DecodeMetadata(data)
	if err != nil {
		return nil, err
	}
	return &Icon{src: data, Color: color.NRGBA{A: 0xff}}, nil
}

func (ic *Icon) Layout(gtx layout.Context, sz unit.Value) layout.Dimensions {
	ico := ic.image(gtx.Px(sz))
	ico.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
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
	m.Palette[0] = f32color.NRGBAToLinearRGBA(ic.Color)
	iconvg.Decode(&ico, ic.src, &iconvg.DecodeOptions{
		Palette: &m.Palette,
	})
	ic.op = paint.NewImageOp(img)
	ic.imgSize = sz
	ic.imgColor = ic.Color
	return ic.op
}
