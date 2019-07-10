// SPDX-License-Identifier: Unlicense OR MIT

package measure

import (
	"math"
	"unicode"
	"unicode/utf8"

	"gioui.org/ui"
	"gioui.org/ui/draw"
	"gioui.org/ui/f32"
	"gioui.org/ui/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

type Faces struct {
	Config      ui.Config
	faceCache   map[faceKey]*textFace
	layoutCache map[layoutKey]cachedLayout
	pathCache   map[pathKey]cachedPath
}

type cachedLayout struct {
	active bool
	layout *text.Layout
}

type cachedPath struct {
	active bool
	path   ui.BlockOp
}

type layoutKey struct {
	f    *sfnt.Font
	ppem fixed.Int26_6
	str  string
	opts text.LayoutOptions
}

type pathKey struct {
	f    *sfnt.Font
	ppem fixed.Int26_6
	str  string
}

type faceKey struct {
	font *sfnt.Font
	size ui.Value
}

type textFace struct {
	faces *Faces
	size  ui.Value
	font  *opentype
}

func (f *Faces) Frame() {
	f.init()
	for pk, p := range f.pathCache {
		if !p.active {
			delete(f.pathCache, pk)
			continue
		}
		p.active = false
		f.pathCache[pk] = p
	}
	for lk, l := range f.layoutCache {
		if !l.active {
			delete(f.layoutCache, lk)
			continue
		}
		l.active = false
		f.layoutCache[lk] = l
	}
}

func (f *Faces) For(fnt *sfnt.Font, size ui.Value) text.Face {
	f.init()
	fk := faceKey{fnt, size}
	if f, exist := f.faceCache[fk]; exist {
		return f
	}
	face := &textFace{
		faces: f,
		size:  size,
		font:  &opentype{Font: fnt, Hinting: font.HintingFull},
	}
	f.faceCache[fk] = face
	return face
}

func (f *Faces) init() {
	if f.faceCache != nil {
		return
	}
	f.faceCache = make(map[faceKey]*textFace)
	f.pathCache = make(map[pathKey]cachedPath)
	f.layoutCache = make(map[layoutKey]cachedLayout)
}

func (f *textFace) Layout(str string, opts text.LayoutOptions) *text.Layout {
	ppem := fixed.Int26_6(f.faces.Config.Px(f.size) * 64)
	lk := layoutKey{
		f:    f.font.Font,
		ppem: ppem,
		str:  str,
		opts: opts,
	}
	if l, ok := f.faces.layoutCache[lk]; ok {
		l.active = true
		f.faces.layoutCache[lk] = l
		return l.layout
	}
	l := layoutText(ppem, str, f.font, opts)
	f.faces.layoutCache[lk] = cachedLayout{active: true, layout: l}
	return l
}

func (f *textFace) Path(str text.String) ui.BlockOp {
	ppem := fixed.Int26_6(f.faces.Config.Px(f.size) * 64)
	pk := pathKey{
		f:    f.font.Font,
		ppem: ppem,
		str:  str.String,
	}
	if p, ok := f.faces.pathCache[pk]; ok {
		p.active = true
		f.faces.pathCache[pk] = p
		return p.path
	}
	p := textPath(ppem, f.font, str)
	f.faces.pathCache[pk] = cachedPath{active: true, path: p}
	return p
}

func layoutText(ppem fixed.Int26_6, str string, f *opentype, opts text.LayoutOptions) *text.Layout {
	m := f.Metrics(ppem)
	lineTmpl := text.Line{
		Ascent: m.Ascent,
		// m.Height is equal to m.Ascent + m.Descent + linegap.
		// Compute the descent including the linegap.
		Descent: m.Height - m.Ascent,
		Bounds:  f.Bounds(ppem),
	}
	var lines []text.Line
	maxDotX := fixed.Int26_6(math.MaxInt32)
	if opts.MaxWidth != ui.Inf {
		maxDotX = fixed.I(opts.MaxWidth)
	}
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
		nl := text.IsNewline(c)
		if opts.SingleLine && nl {
			nl = false
			c = ' '
			s = 1
		}
		a, ok := f.GlyphAdvance(ppem, c)
		if !ok {
			prev.idx += s
			continue
		}
		next := state{
			r:     c,
			advs:  prev.advs,
			idx:   prev.idx + s,
			x:     prev.x + prev.adv,
			valid: true,
		}
		if nl {
			// The newline is zero width; use the previous
			// character for line measurements.
			prev.advs = append(prev.advs, 0)
			prev.idx = next.idx
			endLine()
			continue
		}
		next.adv = a
		var k fixed.Int26_6
		if prev.valid {
			k = f.Kern(ppem, prev.r, next.r)
		}
		// Break the line if we're out of space.
		if prev.idx > 0 && next.x+next.adv+k >= maxDotX {
			// If the line contains no word breaks, break off the last rune.
			if word.idx == 0 {
				word = prev
			}
			next.x -= word.x + word.adv
			next.idx -= word.idx
			next.advs = next.advs[len(word.advs):]
			prev = word
			endLine()
		} else {
			next.adv += k
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

func textPath(ppem fixed.Int26_6, f *opentype, str text.String) ui.BlockOp {
	var lastPos f32.Point
	var builder draw.PathBuilder
	ops := new(ui.Ops)
	builder.Init(ops)
	var x fixed.Int26_6
	var advIdx int
	ops.Begin()
	for _, r := range str.String {
		if !unicode.IsSpace(r) {
			segs, ok := f.LoadGlyph(ppem, r)
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
	builder.End()
	return ops.End()
}
