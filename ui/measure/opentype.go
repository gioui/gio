// SPDX-License-Identifier: Unlicense OR MIT

package measure

import (
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type opentype struct {
	Font    *sfnt.Font
	Hinting font.Hinting

	buf sfnt.Buffer
}

func (f *opentype) GlyphAdvance(ppem fixed.Int26_6, r rune) (advance fixed.Int26_6, ok bool) {
	g, err := f.Font.GlyphIndex(&f.buf, r)
	if err != nil {
		return 0, false
	}
	adv, err := f.Font.GlyphAdvance(&f.buf, g, ppem, f.Hinting)
	return adv, err == nil
}

func (f *opentype) Kern(ppem fixed.Int26_6, r0, r1 rune) fixed.Int26_6 {
	g0, err := f.Font.GlyphIndex(&f.buf, r0)
	if err != nil {
		return 0
	}
	g1, err := f.Font.GlyphIndex(&f.buf, r1)
	if err != nil {
		return 0
	}
	adv, err := f.Font.Kern(&f.buf, g0, g1, ppem, f.Hinting)
	if err != nil {
		return 0
	}
	return adv
}

func (f *opentype) Metrics(ppem fixed.Int26_6) font.Metrics {
	m, _ := f.Font.Metrics(&f.buf, ppem, f.Hinting)
	return m
}

func (f *opentype) Bounds(ppem fixed.Int26_6) fixed.Rectangle26_6 {
	r, _ := f.Font.Bounds(&f.buf, ppem, f.Hinting)
	return r
}

func (f *opentype) LoadGlyph(ppem fixed.Int26_6, r rune) ([]sfnt.Segment, bool) {
	g, err := f.Font.GlyphIndex(&f.buf, r)
	if err != nil {
		return nil, false
	}
	segs, err := f.Font.LoadGlyph(&f.buf, g, ppem, nil)
	if err != nil {
		return nil, false
	}
	return segs, true
}
