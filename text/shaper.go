// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"golang.org/x/image/font"

	"gioui.org/op"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

// Shaper implements layout and shaping of text.
type Shaper interface {
	// Layout a text according to a set of options.
	Layout(c unit.Converter, font Font, str string, opts LayoutOptions) []Line
	// Shape a line of text previously laid out by Layout.
	Shape(c unit.Converter, font Font, str string, layout []Glyph) op.CallOp
	Metrics(c unit.Converter, font Font) font.Metrics
}

// FontRegistry implements layout and shaping of text and a cache of
// computed results.
//
// If a font matches no registered shape, FontRegistry falls back to the
// first registered face.
type FontRegistry struct {
	def   Typeface
	faces map[Font]*face
}

type face struct {
	face        Face
	layoutCache layoutCache
	pathCache   pathCache
}

func (s *FontRegistry) Register(font Font, tf Face) {
	if s.faces == nil {
		s.def = font.Typeface
		s.faces = make(map[Font]*face)
	}
	// Treat all font sizes equally.
	font.Size = unit.Value{}
	if font.Weight == 0 {
		font.Weight = Normal
	}
	s.faces[font] = &face{
		face: tf,
	}
}

func (s *FontRegistry) Layout(c unit.Converter, font Font, str string, opts LayoutOptions) []Line {
	tf := s.faceForFont(font)
	return tf.layout(fixed.I(c.Px(font.Size)), str, opts)
}

func (s *FontRegistry) Shape(c unit.Converter, font Font, str string, layout []Glyph) op.CallOp {
	tf := s.faceForFont(font)
	return tf.shape(fixed.I(c.Px(font.Size)), str, layout)
}

func (s *FontRegistry) Metrics(c unit.Converter, font Font) font.Metrics {
	tf := s.faceForFont(font)
	return tf.metrics(fixed.I(c.Px(font.Size)))
}

func (s *FontRegistry) faceForStyle(font Font) *face {
	tf := s.faces[font]
	if tf == nil {
		font := font
		font.Weight = Normal
		tf = s.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		tf = s.faces[font]
	}
	if tf == nil {
		font := font
		font.Style = Regular
		font.Weight = Normal
		tf = s.faces[font]
	}
	return tf
}

func (s *FontRegistry) faceForFont(font Font) *face {
	font.Size = unit.Value{}
	tf := s.faceForStyle(font)
	if tf == nil {
		font.Typeface = s.def
		tf = s.faceForStyle(font)
	}
	return tf
}

func (t *face) layout(ppem fixed.Int26_6, str string, opts LayoutOptions) []Line {
	if t == nil {
		return nil
	}
	lk := layoutKey{
		ppem: ppem,
		str:  str,
		opts: opts,
	}
	if l, ok := t.layoutCache.Get(lk); ok {
		return l
	}
	l := t.face.Layout(ppem, str, opts)
	t.layoutCache.Put(lk, l)
	return l
}

func (t *face) shape(ppem fixed.Int26_6, str string, layout []Glyph) op.CallOp {
	if t == nil {
		return op.CallOp{}
	}
	pk := pathKey{
		ppem: ppem,
		str:  str,
	}
	if clip, ok := t.pathCache.Get(pk); ok {
		return clip
	}
	clip := t.face.Shape(ppem, layout)
	t.pathCache.Put(pk, clip)
	return clip
}

func (t *face) metrics(ppem fixed.Int26_6) font.Metrics {
	return t.face.Metrics(ppem)
}
