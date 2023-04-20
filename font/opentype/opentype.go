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
	"github.com/go-text/typesetting/font"
	fontapi "github.com/go-text/typesetting/opentype/api/font"
	"github.com/go-text/typesetting/opentype/api/metadata"
	"github.com/go-text/typesetting/opentype/loader"
)

// Face is a thread-safe representation of a loaded font. For efficiency, applications
// should construct a face for any given font file once, reusing it across different
// text shapers.
type Face struct {
	face    font.Font
	aspect  metadata.Aspect
	family  string
	variant string
}

// Parse constructs a Face from source bytes.
func Parse(src []byte) (Face, error) {
	ld, err := loader.NewLoader(bytes.NewReader(src))
	if err != nil {
		return Face{}, err
	}
	font, aspect, family, variant, err := parseLoader(ld)
	if err != nil {
		return Face{}, fmt.Errorf("failed parsing truetype font: %w", err)
	}
	return Face{
		face:    font,
		aspect:  aspect,
		family:  family,
		variant: variant,
	}, nil
}

// ParseCollection parse an Opentype font file, with support for collections.
// Single font files are supported, returning a slice with length 1.
// The returned fonts are automatically wrapped in a text.FontFace with
// inferred font metadata.
// BUG(whereswaldon): the only Variant that can be detected automatically is
// "Mono".
func ParseCollection(src []byte) ([]giofont.FontFace, error) {
	lds, err := loader.NewLoaders(bytes.NewReader(src))
	if err != nil {
		return nil, err
	}
	out := make([]giofont.FontFace, len(lds))
	for i, ld := range lds {
		face, aspect, family, variant, err := parseLoader(ld)
		if err != nil {
			return nil, fmt.Errorf("reading font %d of collection: %s", i, err)
		}
		ff := Face{
			face:    face,
			aspect:  aspect,
			family:  family,
			variant: variant,
		}
		out[i] = giofont.FontFace{
			Face: ff,
			Font: ff.Font(),
		}
	}

	return out, nil
}

// parseLoader parses the contents of the loader into a face and its metadata.
func parseLoader(ld *loader.Loader) (_ font.Font, _ metadata.Aspect, family, variant string, _ error) {
	ft, err := fontapi.NewFont(ld)
	if err != nil {
		return nil, metadata.Aspect{}, "", "", err
	}
	data := metadata.Metadata(ld)
	if data.IsMonospace {
		variant = "Mono"
	}
	return ft, data.Aspect, data.Family, variant, nil
}

// Face returns a thread-unsafe wrapper for this Face suitable for use by a single shaper.
// Face many be invoked any number of times and is safe so long as each return value is
// only used by one goroutine.
func (f Face) Face() font.Face {
	return &fontapi.Face{Font: f.face}
}

// FontFace returns a text.Font with populated font metadata for the
// font.
// BUG(whereswaldon): the only Variant that can be detected automatically is
// "Mono".
func (f Face) Font() giofont.Font {
	return giofont.Font{
		Typeface: giofont.Typeface(f.family),
		Style:    f.style(),
		Weight:   f.weight(),
		Variant:  giofont.Variant(f.variant),
	}
}

func (f Face) style() giofont.Style {
	switch f.aspect.Style {
	case metadata.StyleItalic:
		return giofont.Italic
	case metadata.StyleNormal:
		fallthrough
	default:
		return giofont.Regular
	}
}

func (f Face) weight() giofont.Weight {
	switch f.aspect.Weight {
	case metadata.WeightThin:
		return giofont.Thin
	case metadata.WeightExtraLight:
		return giofont.ExtraLight
	case metadata.WeightLight:
		return giofont.Light
	case metadata.WeightNormal:
		return giofont.Normal
	case metadata.WeightMedium:
		return giofont.Medium
	case metadata.WeightSemibold:
		return giofont.SemiBold
	case metadata.WeightBold:
		return giofont.Bold
	case metadata.WeightExtraBold:
		return giofont.ExtraBold
	case metadata.WeightBlack:
		return giofont.Black
	default:
		return giofont.Normal
	}
}
