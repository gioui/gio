// SPDX-License-Identifier: Unlicense OR MIT

// Package opentype implements text layout and shaping for OpenType
// files.
//
// NOTE: the OpenType specification allows for fonts to include bitmap images
// in a variety of formats. In the interest of small binary sizes, the opentype
// package only automatically imports the PNG image decoder. If you have a font
// with glyphs in JPEG or TIFF formats, register those decoders with the image
// package in order to ensure those glyphs are visible in text.
package opentype

import (
	"bytes"
	"fmt"
	_ "image/png"

	giofont "gioui.org/font"
	fontapi "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/font/opentype"
)

// Face is a thread-safe representation of a loaded font. For efficiency, applications
// should construct a face for any given font file once, reusing it across different
// text shapers.
type Face struct {
	face *fontapi.Font
	font giofont.Font
}

// Parse constructs a Face from source bytes.
func Parse(src []byte) (Face, error) {
	ld, err := opentype.NewLoader(bytes.NewReader(src))
	if err != nil {
		return Face{}, err
	}
	font, md, err := parseLoader(ld)
	if err != nil {
		return Face{}, fmt.Errorf("failed parsing truetype font: %w", err)
	}
	return Face{
		face: font,
		font: md,
	}, nil
}

// ParseCollection parse an Opentype font file, with support for collections.
// Single font files are supported, returning a slice with length 1.
// The returned fonts are automatically wrapped in a text.FontFace with
// inferred font font.
// BUG(whereswaldon): the only Variant that can be detected automatically is
// "Mono".
func ParseCollection(src []byte) ([]giofont.FontFace, error) {
	lds, err := opentype.NewLoaders(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	out := make([]giofont.FontFace, len(lds))
	for i, ld := range lds {
		face, md, err := parseLoader(ld)
		if err != nil {
			return nil, fmt.Errorf("reading font %d of collection: %s", i, err)
		}
		ff := Face{
			face: face,
			font: md,
		}
		out[i] = giofont.FontFace{
			Face: ff,
			Font: ff.Font(),
		}
	}

	return out, nil
}

func DescriptionToFont(md fontapi.Description) giofont.Font {
	return giofont.Font{
		Typeface: giofont.Typeface(md.Family),
		Style:    gioStyle(md.Aspect.Style),
		Weight:   gioWeight(md.Aspect.Weight),
	}
}

func FontToDescription(font giofont.Font) fontapi.Description {
	return fontapi.Description{
		Family: string(font.Typeface),
		Aspect: fontapi.Aspect{
			Style:  mdStyle(font.Style),
			Weight: mdWeight(font.Weight),
		},
	}
}

// parseLoader parses the contents of the loader into a face and its font.
func parseLoader(ld *opentype.Loader) (*fontapi.Font, giofont.Font, error) {
	ft, err := fontapi.NewFont(ld)
	if err != nil {
		return nil, giofont.Font{}, err
	}
	data := DescriptionToFont(ft.Describe())
	return ft, data, nil
}

// Face returns a thread-unsafe wrapper for this Face suitable for use by a single shaper.
// Face many be invoked any number of times and is safe so long as each return value is
// only used by one goroutine.
func (f Face) Face() *fontapi.Face {
	return &fontapi.Face{Font: f.face}
}

// FontFace returns a text.Font with populated font metadata for the
// font.
// BUG(whereswaldon): the only Variant that can be detected automatically is
// "Mono".
func (f Face) Font() giofont.Font {
	return f.font
}

func gioStyle(s fontapi.Style) giofont.Style {
	switch s {
	case fontapi.StyleItalic:
		return giofont.Italic
	case fontapi.StyleNormal:
		fallthrough
	default:
		return giofont.Regular
	}
}

func mdStyle(g giofont.Style) fontapi.Style {
	switch g {
	case giofont.Italic:
		return fontapi.StyleItalic
	case giofont.Regular:
		fallthrough
	default:
		return fontapi.StyleNormal
	}
}

func gioWeight(w fontapi.Weight) giofont.Weight {
	switch w {
	case fontapi.WeightThin:
		return giofont.Thin
	case fontapi.WeightExtraLight:
		return giofont.ExtraLight
	case fontapi.WeightLight:
		return giofont.Light
	case fontapi.WeightNormal:
		return giofont.Normal
	case fontapi.WeightMedium:
		return giofont.Medium
	case fontapi.WeightSemibold:
		return giofont.SemiBold
	case fontapi.WeightBold:
		return giofont.Bold
	case fontapi.WeightExtraBold:
		return giofont.ExtraBold
	case fontapi.WeightBlack:
		return giofont.Black
	default:
		return giofont.Normal
	}
}

func mdWeight(g giofont.Weight) fontapi.Weight {
	switch g {
	case giofont.Thin:
		return fontapi.WeightThin
	case giofont.ExtraLight:
		return fontapi.WeightExtraLight
	case giofont.Light:
		return fontapi.WeightLight
	case giofont.Normal:
		return fontapi.WeightNormal
	case giofont.Medium:
		return fontapi.WeightMedium
	case giofont.SemiBold:
		return fontapi.WeightSemibold
	case giofont.Bold:
		return fontapi.WeightBold
	case giofont.ExtraBold:
		return fontapi.WeightExtraBold
	case giofont.Black:
		return fontapi.WeightBlack
	default:
		return fontapi.WeightNormal
	}
}
