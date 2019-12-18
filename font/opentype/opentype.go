// SPDX-License-Identifier: Unlicense OR MIT

// Package opentype implements text layout and shaping for OpenType
// files.
package opentype

import (
	"io"

	"unicode"
	"unicode/utf8"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Font implements text.Face.
type Font struct {
	font *sfnt.Font
	buf  sfnt.Buffer
}

// Collection is a collection of one or more fonts.
type Collection struct {
	coll *sfnt.Collection
}

type opentype struct {
	Font    *sfnt.Font
	Hinting font.Hinting
}

// NewFont parses an SFNT font, such as TTF or OTF data, from a []byte
// data source.
func Parse(src []byte) (*Font, error) {
	fnt, err := sfnt.Parse(src)
	if err != nil {
		return nil, err
	}
	return &Font{font: fnt}, nil
}

// ParseCollection parses an SFNT font collection, such as TTC or OTC data,
// from a []byte data source.
//
// If passed data for a single font, a TTF or OTF instead of a TTC or OTC,
// it will return a collection containing 1 font.
func ParseCollection(src []byte) (*Collection, error) {
	c, err := sfnt.ParseCollection(src)
	if err != nil {
		return nil, err
	}
	return &Collection{c}, nil
}

// ParseCollectionReaderAt parses an SFNT collection, such as TTC or OTC data,
// from an io.ReaderAt data source.
//
// If passed data for a single font, a TTF or OTF instead of a TTC or OTC, it
// will return a collection containing 1 font.
func ParseCollectionReaderAt(src io.ReaderAt) (*Collection, error) {
	c, err := sfnt.ParseCollectionReaderAt(src)
	if err != nil {
		return nil, err
	}
	return &Collection{c}, nil
}

// NumFonts returns the number of fonts in the collection.
func (c *Collection) NumFonts() int {
	return c.coll.NumFonts()
}

// Font returns the i'th font in the collection.
func (c *Collection) Font(i int) (*Font, error) {
	fnt, err := c.coll.Font(i)
	if err != nil {
		return nil, err
	}
	return &Font{font: fnt}, nil
}

func (f *Font) Layout(ppem fixed.Int26_6, str string, opts text.LayoutOptions) *text.Layout {
	return layoutText(&f.buf, ppem, str, &opentype{Font: f.font, Hinting: font.HintingFull}, opts)
}

func (f *Font) Shape(ppem fixed.Int26_6, str text.String) op.CallOp {
	return textPath(&f.buf, ppem, &opentype{Font: f.font, Hinting: font.HintingFull}, str)
}

func (f *Font) Metrics(ppem fixed.Int26_6) font.Metrics {
	o := &opentype{Font: f.font, Hinting: font.HintingFull}
	return o.Metrics(&f.buf, ppem)
}

func layoutText(buf *sfnt.Buffer, ppem fixed.Int26_6, str string, f *opentype, opts text.LayoutOptions) *text.Layout {
	m := f.Metrics(buf, ppem)
	lineTmpl := text.Line{
		Ascent: m.Ascent,
		// m.Height is equal to m.Ascent + m.Descent + linegap.
		// Compute the descent including the linegap.
		Descent: m.Height - m.Ascent,
		Bounds:  f.Bounds(buf, ppem),
	}
	var lines []text.Line
	maxDotX := fixed.I(opts.MaxWidth)
	type state struct {
		r     rune
		advs  []fixed.Int26_6
		adv   fixed.Int26_6
		x     fixed.Int26_6
		idx   int
		valid bool
	}
	var prev, word state
	endLine := func() {
		line := lineTmpl
		line.Text.Advances = prev.advs
		line.Text.String = str[:prev.idx]
		line.Width = prev.x + prev.adv
		line.Bounds.Max.X += prev.x
		lines = append(lines, line)
		str = str[prev.idx:]
		prev = state{}
		word = state{}
	}
	for prev.idx < len(str) {
		c, s := utf8.DecodeRuneInString(str[prev.idx:])
		a, valid := f.GlyphAdvance(buf, ppem, c)
		next := state{
			r:     c,
			advs:  prev.advs,
			idx:   prev.idx + s,
			x:     prev.x + prev.adv,
			adv:   a,
			valid: valid,
		}
		if c == '\n' {
			// The newline is zero width; use the previous
			// character for line measurements.
			prev.advs = append(prev.advs, 0)
			prev.idx = next.idx
			endLine()
			continue
		}
		var k fixed.Int26_6
		if prev.valid {
			k = f.Kern(buf, ppem, prev.r, next.r)
		}
		// Break the line if we're out of space.
		if prev.idx > 0 && next.x+next.adv+k > maxDotX {
			// If the line contains no word breaks, break off the last rune.
			if word.idx == 0 {
				word = prev
			}
			next.x -= word.x + word.adv
			next.idx -= word.idx
			next.advs = next.advs[len(word.advs):]
			prev = word
			endLine()
		} else if k != 0 {
			next.advs[len(next.advs)-1] += k
			next.x += k
		}
		next.advs = append(next.advs, next.adv)
		if unicode.IsSpace(next.r) {
			word = next
		}
		prev = next
	}
	endLine()
	return &text.Layout{Lines: lines}
}

func textPath(buf *sfnt.Buffer, ppem fixed.Int26_6, f *opentype, str text.String) op.CallOp {
	var lastPos f32.Point
	var builder clip.Path
	ops := new(op.Ops)
	var x fixed.Int26_6
	var advIdx int
	builder.Begin(ops)
	for _, r := range str.String {
		if !unicode.IsSpace(r) {
			segs, ok := f.LoadGlyph(buf, ppem, r)
			if !ok {
				continue
			}
			// Move to glyph position.
			pos := f32.Point{
				X: float32(x) / 64,
			}
			builder.Move(pos.Sub(lastPos))
			lastPos = pos
			var lastArg f32.Point
			// Convert sfnt.Segments to relative segments.
			for _, fseg := range segs {
				nargs := 1
				switch fseg.Op {
				case sfnt.SegmentOpQuadTo:
					nargs = 2
				case sfnt.SegmentOpCubeTo:
					nargs = 3
				}
				var args [3]f32.Point
				for i := 0; i < nargs; i++ {
					a := f32.Point{
						X: float32(fseg.Args[i].X) / 64,
						Y: float32(fseg.Args[i].Y) / 64,
					}
					args[i] = a.Sub(lastArg)
					if i == nargs-1 {
						lastArg = a
					}
				}
				switch fseg.Op {
				case sfnt.SegmentOpMoveTo:
					builder.Move(args[0])
				case sfnt.SegmentOpLineTo:
					builder.Line(args[0])
				case sfnt.SegmentOpQuadTo:
					builder.Quad(args[0], args[1])
				case sfnt.SegmentOpCubeTo:
					builder.Cube(args[0], args[1], args[2])
				default:
					panic("unsupported segment op")
				}
			}
			lastPos = lastPos.Add(lastArg)
		}
		x += str.Advances[advIdx]
		advIdx++
	}
	builder.End().Add(ops)
	return op.CallOp{Ops: ops}
}

func (f *opentype) GlyphAdvance(buf *sfnt.Buffer, ppem fixed.Int26_6, r rune) (advance fixed.Int26_6, ok bool) {
	g, err := f.Font.GlyphIndex(buf, r)
	if err != nil {
		return 0, false
	}
	adv, err := f.Font.GlyphAdvance(buf, g, ppem, f.Hinting)
	return adv, err == nil
}

func (f *opentype) Kern(buf *sfnt.Buffer, ppem fixed.Int26_6, r0, r1 rune) fixed.Int26_6 {
	g0, err := f.Font.GlyphIndex(buf, r0)
	if err != nil {
		return 0
	}
	g1, err := f.Font.GlyphIndex(buf, r1)
	if err != nil {
		return 0
	}
	adv, err := f.Font.Kern(buf, g0, g1, ppem, f.Hinting)
	if err != nil {
		return 0
	}
	return adv
}

func (f *opentype) Metrics(buf *sfnt.Buffer, ppem fixed.Int26_6) font.Metrics {
	m, _ := f.Font.Metrics(buf, ppem, f.Hinting)
	return m
}

func (f *opentype) Bounds(buf *sfnt.Buffer, ppem fixed.Int26_6) fixed.Rectangle26_6 {
	r, _ := f.Font.Bounds(buf, ppem, f.Hinting)
	return r
}

func (f *opentype) LoadGlyph(buf *sfnt.Buffer, ppem fixed.Int26_6, r rune) ([]sfnt.Segment, bool) {
	g, err := f.Font.GlyphIndex(buf, r)
	if err != nil {
		return nil, false
	}
	segs, err := f.Font.LoadGlyph(buf, g, ppem, nil)
	if err != nil {
		return nil, false
	}
	return segs, true
}
