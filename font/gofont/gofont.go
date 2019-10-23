// SPDX-License-Identifier: Unlicense OR MIT

// Package gofont registers the Go fonts in the font registry.
//
// See https://blog.golang.org/go-fonts for a description of the
// fonts, and the golang.org/x/image/font/gofont packages for the
// font data.
package gofont

import (
	"fmt"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomediumitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/gomonobolditalic"
	"golang.org/x/image/font/gofont/gomonoitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/gofont/gosmallcaps"
	"golang.org/x/image/font/gofont/gosmallcapsitalic"
)

func Register() {
	register(text.Font{}, goregular.TTF)
	register(text.Font{Style: text.Italic}, goitalic.TTF)
	register(text.Font{Weight: text.Bold}, gobold.TTF)
	register(text.Font{Style: text.Italic, Weight: text.Bold}, gobolditalic.TTF)
	register(text.Font{Weight: text.Medium}, gomedium.TTF)
	register(text.Font{Weight: text.Medium, Style: text.Italic}, gomediumitalic.TTF)
	register(text.Font{Variant: "Mono"}, gomono.TTF)
	register(text.Font{Variant: "Mono", Weight: text.Bold}, gomonobold.TTF)
	register(text.Font{Variant: "Mono", Weight: text.Bold, Style: text.Italic}, gomonobolditalic.TTF)
	register(text.Font{Variant: "Mono", Style: text.Italic}, gomonoitalic.TTF)
	register(text.Font{Variant: "Mono", Style: text.Italic}, gomonoitalic.TTF)
	register(text.Font{Variant: "Smallcaps"}, gosmallcaps.TTF)
	register(text.Font{Variant: "Smallcaps", Style: text.Italic}, gosmallcapsitalic.TTF)
}

func register(fnt text.Font, ttf []byte) {
	face, err := opentype.Parse(ttf)
	if err != nil {
		panic(fmt.Sprintf("failed to parse font: %v", err))
	}
	fnt.Typeface = "Go"
	font.Register(fnt, face)
}
