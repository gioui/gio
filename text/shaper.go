// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/op/clip"
	"github.com/go-text/typesetting/font"
	"golang.org/x/image/math/fixed"
)

// Parameters are static text shaping attributes applied to the entire shaped text.
type Parameters struct {
	// Font describes the preferred typeface.
	Font Font
	// Alignment characterizes the positioning of text within the line. It does not directly
	// impact shaping, but is provided in order to allow efficient offset computation.
	Alignment Alignment
	// PxPerEm is the pixels-per-em to shape the text with.
	PxPerEm fixed.Int26_6
	// MaxLines limits the quantity of shaped lines. Zero means no limit.
	MaxLines int
	// Truncator is a string of text to insert where the shaped text was truncated, which
	// can currently ohly happen if MaxLines is nonzero and the text on the final line is
	// truncated.
	Truncator string

	// MinWidth and MaxWidth provide the minimum and maximum horizontal space constraints
	// for the shaped text.
	MinWidth, MaxWidth int
	// Locale provides primary direction and language information for the shaped text.
	Locale system.Locale

	// forceTruncate controls whether the truncator string is inserted on the final line of
	// text with a MaxLines. It is unexported because this behavior only makes sense for the
	// shaper to control when it iterates paragraphs of text.
	forceTruncate bool
}

// A FontFace is a Font and a matching Face.
type FontFace struct {
	Font Font
	Face Face
}

// Glyph describes a shaped font glyph. Many fields are distances relative
// to the "dot", which is a point on the baseline (the line upon which glyphs
// visually rest) for the line of text containing the glyph.
//
// Glyphs are organized into "glyph clusters," which are sequences that
// may represent an arbitrary number of runes.
//
// Sequences of glyph clusters that share style parameters are grouped into "runs."
//
// "Document coordinates" are pixel values relative to the text's origin at (0,0)
// in the upper-left corner" Displaying each shaped glyph at the document
// coordinates of its dot will correctly visualize the text.
type Glyph struct {
	// ID is a unique, per-shaper identifier for the shape of the glyph.
	// Glyphs from the same shaper will share an ID when they are from
	// the same face and represent the same glyph at the same size.
	ID GlyphID

	// X is the x coordinate of the dot for this glyph in document coordinates.
	X fixed.Int26_6
	// Y is the y coordinate of the dot for this glyph in document coordinates.
	Y int32

	// Advance is the logical width of the glyph. The glyph may be visually
	// wider than this.
	Advance fixed.Int26_6
	// Ascent is the distance from the dot to the logical top of glyphs in
	// this glyph's face. The specific glyph may be shorter than this.
	Ascent fixed.Int26_6
	// Descent is the distance from the dot to the logical bottom of glyphs
	// in this glyph's face. The specific glyph may descend less than this.
	Descent fixed.Int26_6
	// Offset encodes the origin of the drawing coordinate space for this glyph
	// relative to the dot. This value is used when converting glyphs to paths.
	Offset fixed.Point26_6
	// Bounds encodes the visual dimensions of the glyph relative to the dot.
	Bounds fixed.Rectangle26_6
	// Runes is the number of runes represented by the glyph cluster this glyph
	// belongs to. If Flags does not contain FlagClusterBreak, this value will
	// always be zero. The final glyph in the cluster contains the runes count
	// for the entire cluster.
	Runes int
	// Flags encode special properties of this glyph.
	Flags Flags
}

type Flags uint16

const (
	// FlagTowardOrigin is set for glyphs in runs that flow
	// towards the origin (RTL).
	FlagTowardOrigin Flags = 1 << iota
	// FlagLineBreak is set for the last glyph in a line.
	FlagLineBreak
	// FlagRunBreak is set for the last glyph in a run. A run is a sequence of
	// glyphs sharing constant style properties (same size, same face, same
	// direction, etc...).
	FlagRunBreak
	// FlagClusterBreak is set for the last glyph in a glyph cluster. A glyph cluster is a
	// sequence of glyphs which are logically a single unit, but require multiple
	// symbols from a font to display.
	FlagClusterBreak
	// FlagParagraphBreak indicates that the glyph cluster does not represent actual
	// font glyphs, but was inserted by the shaper to represent line-breaking
	// whitespace characters. After a glyph with FlagParagraphBreak set, the shaper
	// will always return a glyph with FlagParagraphStart providing the X and Y
	// coordinates of the start of the next line, even if that line has no contents.
	FlagParagraphBreak
	// FlagParagraphStart indicates that the glyph starts a new paragraph.
	FlagParagraphStart
	// FlagTruncator indicates that the glyph is part of a special truncator run that
	// represents the portion of text removed due to truncation. A glyph with both
	// FlagTruncator and FlagClusterBreak will have a Runes field accounting for all
	// runes truncated.
	FlagTruncator
)

func (f Flags) String() string {
	var b strings.Builder
	if f&FlagParagraphStart != 0 {
		b.WriteString("S")
	} else {
		b.WriteString("_")
	}
	if f&FlagParagraphBreak != 0 {
		b.WriteString("P")
	} else {
		b.WriteString("_")
	}
	if f&FlagTowardOrigin != 0 {
		b.WriteString("T")
	} else {
		b.WriteString("_")
	}
	if f&FlagLineBreak != 0 {
		b.WriteString("L")
	} else {
		b.WriteString("_")
	}
	if f&FlagRunBreak != 0 {
		b.WriteString("R")
	} else {
		b.WriteString("_")
	}
	if f&FlagClusterBreak != 0 {
		b.WriteString("C")
	} else {
		b.WriteString("_")
	}
	if f&FlagTruncator != 0 {
		b.WriteString("…")
	} else {
		b.WriteString("_")
	}
	return b.String()
}

type GlyphID uint64

// Shaper converts strings of text into glyphs that can be displayed.
type Shaper struct {
	shaper           shaperImpl
	pathCache        pathCache
	bitmapShapeCache bitmapShapeCache
	layoutCache      layoutCache
	paragraph        []rune

	reader strings.Reader

	// Iterator state.
	brokeParagraph   bool
	pararagraphStart Glyph
	txt              document
	line             int
	run              int
	glyph            int
	// advance is the width of glyphs from the current run that have already been displayed.
	advance fixed.Int26_6
	// done tracks whether iteration is over.
	done bool
	err  error
}

// NewShaper constructs a shaper with the provided collection of font faces
// available.
func NewShaper(collection []FontFace) *Shaper {
	l := &Shaper{}
	for _, f := range collection {
		l.shaper.Load(f)
	}
	l.shaper.shaper.SetFontCacheSize(32)
	return l
}

// Layout text from an io.Reader according to a set of options. Results can be retrieved by
// iteratively calling NextGlyph.
func (l *Shaper) Layout(params Parameters, txt io.Reader) {
	l.layoutText(params, bufio.NewReader(txt), "")
}

// LayoutString is Layout for strings.
func (l *Shaper) LayoutString(params Parameters, str string) {
	l.layoutText(params, nil, str)
}

func (l *Shaper) reset(align Alignment) {
	l.line, l.run, l.glyph, l.advance = 0, 0, 0, 0
	l.done = false
	l.txt.reset()
	l.txt.alignment = align
}

// layoutText lays out a large text document by breaking it into paragraphs and laying
// out each of them separately. This allows the shaping results to be cached independently
// by paragraph. Only one of txt and str should be provided.
func (l *Shaper) layoutText(params Parameters, txt *bufio.Reader, str string) {
	l.reset(params.Alignment)
	if txt == nil && len(str) == 0 {
		l.txt.append(l.layoutParagraph(params, "", nil))
		return
	}
	truncating := params.MaxLines > 0
	var done bool
	var startByte int
	var endByte int
	for !done {
		var runes int
		l.paragraph = l.paragraph[:0]
		if txt != nil {
			for r, _, re := txt.ReadRune(); !done; r, _, re = txt.ReadRune() {
				if re != nil {
					done = true
					continue
				}
				l.paragraph = append(l.paragraph, r)
				runes++
				if r == '\n' {
					break
				}
			}
			_, _, re := txt.ReadRune()
			done = re != nil
			_ = txt.UnreadRune()
		} else {
			for endByte = startByte; endByte < len(str); {
				r, width := utf8.DecodeRuneInString(str[endByte:])
				endByte += width
				runes++
				if r == '\n' {
					break
				}
			}
			done = endByte == len(str)
		}
		if startByte != endByte || (len(l.paragraph) > 0 || len(l.txt.lines) == 0) {
			params.forceTruncate = truncating && !done
			lines := l.layoutParagraph(params, str[startByte:endByte], l.paragraph)
			if truncating {
				params.MaxLines -= len(lines.lines)
				if params.MaxLines == 0 {
					done = true
					// We've truncated the text, but we need to account for all of the runes we never
					// decoded in the truncator.
					var unreadRunes int
					if txt == nil {
						unreadRunes = utf8.RuneCountInString(str[endByte:])
					} else {
						for {
							_, _, e := txt.ReadRune()
							if e != nil {
								break
							}
							unreadRunes++
						}
					}
					lastLineIdx := len(lines.lines) - 1
					lastRunIdx := len(lines.lines[lastLineIdx].runs) - 1
					lastGlyphIdx := len(lines.lines[lastLineIdx].runs[lastRunIdx].Glyphs) - 1
					lines.lines[lastLineIdx].runs[lastRunIdx].Runes.Count += unreadRunes
					lines.lines[lastLineIdx].runs[lastRunIdx].Glyphs[lastGlyphIdx].runeCount += unreadRunes
				}
			}
			l.txt.append(lines)
		}
		if done {
			return
		}
		startByte = endByte
	}
}

// layoutParagraph shapes and wraps a paragraph using the provided parameters.
// It accepts the paragraph data in either string or rune format, preferring the
// string in order to hit the shaper cache more quickly.
func (l *Shaper) layoutParagraph(params Parameters, asStr string, asRunes []rune) document {
	if l == nil {
		return document{}
	}
	if len(asStr) == 0 && len(asRunes) > 0 {
		asStr = string(asRunes)
	}
	// Alignment is not part of the cache key because changing it does not impact shaping.
	lk := layoutKey{
		ppem:          params.PxPerEm,
		maxWidth:      params.MaxWidth,
		minWidth:      params.MinWidth,
		maxLines:      params.MaxLines,
		truncator:     params.Truncator,
		locale:        params.Locale,
		font:          params.Font,
		forceTruncate: params.forceTruncate,
		str:           asStr,
	}
	if l, ok := l.layoutCache.Get(lk); ok {
		return l
	}
	if len(asRunes) == 0 && len(asStr) > 0 {
		asRunes = []rune(asStr)
	}
	lines := l.shaper.LayoutRunes(params, asRunes)
	l.layoutCache.Put(lk, lines)
	return lines
}

// NextGlyph returns the next glyph from the most recent shaping operation, if
// any. If there are no more glyphs, ok will be false.
func (l *Shaper) NextGlyph() (_ Glyph, ok bool) {
	if l.done {
		return Glyph{}, false
	}
	for {
		if l.line == len(l.txt.lines) {
			if l.brokeParagraph {
				l.brokeParagraph = false
				return l.pararagraphStart, true
			}
			if l.err == nil {
				l.err = io.EOF
			}
			return Glyph{}, false
		}
		line := l.txt.lines[l.line]
		if l.run == len(line.runs) {
			l.line++
			l.run = 0
			continue
		}
		run := line.runs[l.run]
		align := l.txt.alignment.Align(line.direction, line.width, l.txt.alignWidth)
		if l.line == 0 && l.run == 0 && len(run.Glyphs) == 0 {
			// The very first run is empty, which will only happen when the
			// entire text is a shaped empty string. Return a single synthetic
			// glyph to provide ascent/descent information to the caller.
			l.done = true
			return Glyph{
				X:       align,
				Y:       int32(line.yOffset),
				Runes:   0,
				Flags:   FlagLineBreak | FlagClusterBreak | FlagRunBreak,
				Ascent:  line.ascent,
				Descent: line.descent,
			}, true
		}
		if l.glyph == len(run.Glyphs) {
			l.run++
			l.glyph = 0
			l.advance = 0
			continue
		}
		glyphIdx := l.glyph
		rtl := run.Direction.Progression() == system.TowardOrigin
		if rtl {
			// If RTL, traverse glyphs backwards to ensure rune order.
			glyphIdx = len(run.Glyphs) - 1 - glyphIdx
		}
		g := run.Glyphs[glyphIdx]
		if rtl {
			// Modify the advance prior to computing runOffset to ensure that the
			// current glyph's width is subtracted in RTL.
			l.advance += g.xAdvance
		}
		// runOffset computes how far into the run the dot should be positioned.
		runOffset := l.advance
		if rtl {
			runOffset = run.Advance - l.advance
		}
		glyph := Glyph{
			ID:      g.id,
			X:       align + run.X + runOffset,
			Y:       int32(line.yOffset),
			Ascent:  line.ascent,
			Descent: line.descent,
			Advance: g.xAdvance,
			Runes:   g.runeCount,
			Offset: fixed.Point26_6{
				X: g.xOffset,
				Y: g.yOffset,
			},
			Bounds: g.bounds,
		}
		if run.truncator {
			glyph.Flags |= FlagTruncator
		}
		l.glyph++
		if !rtl {
			l.advance += g.xAdvance
		}

		endOfRun := l.glyph == len(run.Glyphs)
		if endOfRun {
			glyph.Flags |= FlagRunBreak
		}
		endOfLine := endOfRun && l.run == len(line.runs)-1
		if endOfLine {
			glyph.Flags |= FlagLineBreak
		}
		endOfText := endOfLine && l.line == len(l.txt.lines)-1
		nextGlyph := l.glyph
		if rtl {
			nextGlyph = len(run.Glyphs) - 1 - nextGlyph
		}
		endOfCluster := endOfRun || run.Glyphs[nextGlyph].clusterIndex != g.clusterIndex
		if run.truncator {
			// Only emit a single cluster for the entire truncator sequence.
			endOfCluster = endOfRun
		}
		if endOfCluster {
			glyph.Flags |= FlagClusterBreak
		} else {
			glyph.Runes = 0
		}
		if run.Direction.Progression() == system.TowardOrigin {
			glyph.Flags |= FlagTowardOrigin
		}
		if l.brokeParagraph {
			glyph.Flags |= FlagParagraphStart
			l.brokeParagraph = false
		}
		if g.glyphCount == 0 {
			glyph.Flags |= FlagParagraphBreak
			l.brokeParagraph = true
			if endOfText {
				l.pararagraphStart = Glyph{
					Ascent:  glyph.Ascent,
					Descent: glyph.Descent,
					Flags:   FlagParagraphStart | FlagLineBreak | FlagRunBreak | FlagClusterBreak,
				}
				// If a glyph is both a paragraph break and the final glyph, it's a newline
				// at the end of the text. We must inform widgets like the text editor
				// of a valid cursor position they can use for "after" such a newline,
				// taking text alignment into account.
				l.pararagraphStart.X = l.txt.alignment.Align(line.direction, 0, l.txt.alignWidth)
				l.pararagraphStart.Y = glyph.Y + int32((glyph.Ascent + glyph.Descent).Ceil())
			}
		}
		return glyph, true
	}
}

const (
	facebits = 16
	sizebits = 16
	gidbits  = 64 - facebits - sizebits
)

// newGlyphID encodes a face and a glyph id into a GlyphID.
func newGlyphID(ppem fixed.Int26_6, faceIdx int, gid font.GID) GlyphID {
	if gid&^((1<<gidbits)-1) != 0 {
		fmt.Println(gid)
		panic("glyph id out of bounds")
	}
	if faceIdx&^((1<<facebits)-1) != 0 {
		panic("face index out of bounds")
	}
	if ppem&^((1<<sizebits)-1) != 0 {
		panic("ppem out of bounds")
	}
	// Mask off the upper 16 bits of ppem. This still allows values up to
	// 1023.
	ppem &= ((1 << sizebits) - 1)
	return GlyphID(faceIdx)<<(gidbits+sizebits) | GlyphID(ppem)<<(gidbits) | GlyphID(gid)
}

// splitGlyphID is the opposite of newGlyphID.
func splitGlyphID(g GlyphID) (fixed.Int26_6, int, font.GID) {
	faceIdx := int(g) >> (gidbits + sizebits)
	ppem := fixed.Int26_6((g & ((1<<sizebits - 1) << gidbits)) >> gidbits)
	gid := font.GID(g) & (1<<gidbits - 1)
	return ppem, faceIdx, gid
}

// Shape converts the provided glyphs into a path. The path will enclose the forms
// of all vector glyphs.
// All glyphs are expected to be from a single line of text (their Y offsets are ignored).
func (l *Shaper) Shape(gs []Glyph) clip.PathSpec {
	key := l.pathCache.hashGlyphs(gs)
	shape, ok := l.pathCache.Get(key, gs)
	if ok {
		return shape
	}
	pathOps := new(op.Ops)
	shape = l.shaper.Shape(pathOps, gs)
	l.pathCache.Put(key, gs, shape)
	return shape
}

// Bitmaps extracts bitmap glyphs from the provided slice and creates an op.CallOp to present
// them. The returned op.CallOp will align correctly with the return value of Shape() for the
// same gs slice.
// All glyphs are expected to be from a single line of text (their Y offsets are ignored).
func (l *Shaper) Bitmaps(gs []Glyph) op.CallOp {
	key := l.bitmapShapeCache.hashGlyphs(gs)
	call, ok := l.bitmapShapeCache.Get(key, gs)
	if ok {
		return call
	}
	callOps := new(op.Ops)
	call = l.shaper.Bitmaps(callOps, gs)
	l.bitmapShapeCache.Put(key, gs, call)
	return call
}
