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

	"github.com/go-text/typesetting/font"
)

// Face is a shapeable representation of a font.
type Face struct {
	face font.Face
}

// Parse constructs a Face from source bytes.
func Parse(src []byte) (Face, error) {
	face, err := font.ParseTTF(bytes.NewReader(src))
	if err != nil {
		return Face{}, fmt.Errorf("failed parsing truetype font: %w", err)
	}
	return Face{face: face}, nil
}

func (f Face) Face() font.Face {
	return f.face
}
