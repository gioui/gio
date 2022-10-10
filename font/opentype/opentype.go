// SPDX-License-Identifier: Unlicense OR MIT

// Package opentype implements text layout and shaping for OpenType
// files.
package opentype

import (
	"bytes"
	"fmt"

	"github.com/benoitkugler/textlayout/fonts/truetype"
	"github.com/go-text/typesetting/font"
)

// Face is a shapeable representation of a font.
type Face struct {
	face font.Face
}

// Parse constructs a Face from source bytes.
func Parse(src []byte) (Face, error) {
	face, err := truetype.Parse(bytes.NewReader(src))
	if err != nil {
		return Face{}, fmt.Errorf("failed parsing truetype font: %w", err)
	}
	return Face{face: face}, nil
}

func (f Face) Face() font.Face {
	return f.face
}
