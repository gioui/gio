// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"image/color"
	"strings"
	"testing"

	nsareg "eliasnaur.com/font/noto/sans/arabic/regular"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/gpu/headless"
	"gioui.org/internal/f32color"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
)

func TestPaintSelectedText(t *testing.T) {
	for _, tc := range []struct {
		name       string
		text       string
		start, end int
		scroll     image.Point
		alpha      uint8
	}{
		{name: "first character", text: "Hello", start: 0, end: 1},
		{name: "middle character", text: "Hello", start: 2, end: 3},
		{name: "reversed selection", text: "Hello", start: 3, end: 2},
		{name: "empty selection", text: "Hello", start: 2, end: 2},
		{name: "horizontal scroll", text: "Hello", start: 0, end: 5, scroll: image.Pt(20, 0)},
		{name: "vertical scroll", text: "Hello\nWorld\nAgain", start: 6, end: 11, scroll: image.Pt(0, 19)},
		{name: "ligature first rune", text: "لا", start: 0, end: 1},
		{name: "ligature last rune", text: "لا", start: 1, end: 2},
		{name: "ligature reversed selection", text: "لا", start: 1, end: 0},
		{name: "ligature horizontal scroll", text: "لا", start: 0, end: 1, scroll: image.Pt(4, 0)},
		{name: "ligature vertical scroll", text: "Hello\nلا", start: 6, end: 7, scroll: image.Pt(0, 19)},
		{name: "ligature translucent text", text: "لا", start: 0, end: 1, alpha: 128},
		{name: "two partial ligatures", text: "لا لا", start: 1, end: 4},
		{name: "ligature in mixed text", text: "Hi لا end", start: 3, end: 4},
		{name: "disjoint bidi selection", text: "Hi لا end", start: 1, end: 4, alpha: 128},
		{name: "mixed text across lines", text: "Hello لا\nWorld", start: 6, end: 14},
		{name: "partial ligature across lines", text: "Hello لا\nWorld", start: 7, end: 14},
		{name: "reversed mixed text across lines", text: "Hello لا\nWorld", start: 14, end: 6},
		{name: "wrapped rtl selection", text: strings.Repeat("لا ", 15), start: 0, end: 44},
	} {
		t.Run(tc.name, func(t *testing.T) {
			size := image.Pt(100, 25)
			window, err := headless.NewWindow(size.X, size.Y)
			if err != nil {
				t.Skipf("headless renderer unavailable: %v", err)
			}
			defer window.Release()
			gtx := layout.Context{
				Ops:         new(op.Ops),
				Constraints: layout.Exact(size),
				Locale:      english,
			}
			face, err := opentype.Parse(nsareg.TTF)
			if err != nil {
				t.Fatal(err)
			}
			collection := append(gofont.Collection(), font.FontFace{Face: face})
			shaper := text.NewShaper(text.NoSystemFonts(), text.WithCollection(collection))
			var view textView
			view.SetSource(newStringSource(tc.text))
			view.Layout(gtx, shaper, font.Font{}, unit.Sp(16))
			if strings.Contains(tc.text, "لا") {
				var ligature bool
				for _, glyph := range view.index.glyphs {
					ligature = ligature || glyph.Runes == 2
				}
				if !ligature {
					t.Fatal("test font did not shape the ligature")
				}
			}
			view.SetCaret(tc.start, tc.end)
			view.scrollOff = tc.scroll

			black := color.NRGBA{A: 255}
			red := color.NRGBA{R: 255, A: 255}
			if tc.alpha != 0 {
				black.A, red.A = tc.alpha, tc.alpha
			}
			render := func(normal color.NRGBA, selected bool) *image.RGBA {
				gtx.Ops.Reset()
				material := colorMaterial(gtx.Ops, normal)
				if selected {
					view.PaintSelectedText(gtx, material, colorMaterial(gtx.Ops, red))
				} else {
					view.PaintText(gtx, material)
				}
				if err := window.Frame(gtx.Ops); err != nil {
					t.Fatal(err)
				}
				pixels := image.NewRGBA(image.Rectangle{Max: size})
				if err := window.Screenshot(pixels); err != nil {
					t.Fatal(err)
				}
				return pixels
			}
			plain := render(black, false)
			selected := render(red, false)
			got := render(black, true)
			regions := view.index.locate(image.Rectangle{Max: size}.Add(tc.scroll), tc.start, tc.end, nil)
			var redPixels int
			for y := range size.Y {
				for x := range size.X {
					p := image.Pt(x, y)
					want := plain.RGBAAt(x, y)
					for _, region := range regions {
						if p.In(region.Bounds) {
							want = selected.RGBAAt(x, y)
							break
						}
					}
					actual := got.RGBAAt(x, y)
					// Compare in linear space to allow small GPU antialiasing differences.
					a := f32color.LinearFromSRGB(color.NRGBA{R: actual.R, G: actual.G, B: actual.B, A: 255})
					w := f32color.LinearFromSRGB(color.NRGBA{R: want.R, G: want.G, B: want.B, A: 255})
					for _, delta := range [...]float32{
						a.R - w.R, a.G - w.G, a.B - w.B,
						float32(int(actual.A)-int(want.A)) / 255,
					} {
						if delta < -3.0/255 || delta > 3.0/255 {
							t.Fatalf("pixel %v: got %v, want %v", p, actual, want)
						}
					}
					if want.R > 0 && want.G == 0 && want.B == 0 {
						redPixels++
					}
				}
			}
			if tc.start != tc.end && redPixels == 0 {
				t.Fatal("selection did not contain any visible red text")
			}
		})
	}
}
