// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"golang.org/x/image/font"
	"unicode/utf8"

	"gioui.org/op"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"
)

// Shaper implements layout and shaping of text and a cache of
// computed results.
//
// Specify the default and fallback font by calling Register with the
// empty Font.
type Shaper struct {
	def   Typeface
	faces map[Font]*face
}

type face struct {
	face        Face
	layoutCache layoutCache
	pathCache   pathCache
}

func (s *Shaper) Register(font Font, tf Face) {
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

func (s *Shaper) Layout(c unit.Converter, font Font, str string, opts LayoutOptions) *Layout {
	tf := s.faceForFont(font)
	return tf.layout(fixed.I(c.Px(font.Size)), str, opts)
}

func (s *Shaper) Shape(c unit.Converter, font Font, str String) op.CallOp {
	tf := s.faceForFont(font)
	return tf.shape(fixed.I(c.Px(font.Size)), str)
}

func (s *Shaper) Metrics(c unit.Converter, font Font) font.Metrics {
	tf := s.faceForFont(font)
	return tf.metrics(fixed.I(c.Px(font.Size)))
}

func (s *Shaper) faceForStyle(font Font) *face {
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

func (s *Shaper) faceForFont(font Font) *face {
	font.Size = unit.Value{}
	tf := s.faceForStyle(font)
	if tf == nil {
		font.Typeface = s.def
		tf = s.faceForStyle(font)
	}
	return tf
}

func (t *face) layout(ppem fixed.Int26_6, str string, opts LayoutOptions) *Layout {
	if t == nil {
		return fallbackLayout(str)
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

func (t *face) shape(ppem fixed.Int26_6, str String) op.CallOp {
	if t == nil {
		return op.CallOp{}
	}
	pk := pathKey{
		ppem: ppem,
		str:  str.String,
	}
	if clip, ok := t.pathCache.Get(pk); ok {
		return clip
	}
	clip := t.face.Shape(ppem, str)
	t.pathCache.Put(pk, clip)
	return clip
}

func (t *face) metrics(ppem fixed.Int26_6) font.Metrics {
	return t.face.Metrics(ppem)
}

func fallbackLayout(str string) *Layout {
	l := &Layout{
		Lines: []Line{
			{Text: String{
				String: str,
			}},
		},
	}
	strlen := utf8.RuneCountInString(str)
	l.Lines[0].Text.Advances = make([]fixed.Int26_6, strlen)
	return l
}
