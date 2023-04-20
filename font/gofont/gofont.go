// SPDX-License-Identifier: Unlicense OR MIT

// Package gofont exports the Go fonts as a text.Collection.
//
// See https://blog.golang.org/go-fonts for a description of the
// fonts, and the golang.org/x/image/font/gofont packages for the
// font data.
package gofont

import (
	"fmt"
	"sync"

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

	"gioui.org/font"
	"gioui.org/font/opentype"
)

var (
	regOnce    sync.Once
	reg        []font.FontFace
	once       sync.Once
	collection []font.FontFace
)

func loadRegular() {
	regOnce.Do(func() {
		face, err := opentype.Parse(goregular.TTF)
		if err != nil {
			panic(fmt.Errorf("failed to parse font: %v", err))
		}
		reg = []font.FontFace{{Font: font.Font{Typeface: "Go"}, Face: face}}
		collection = append(collection, reg[0])
	})
}

// Regular returns a collection of only the Go regular font face.
func Regular() []font.FontFace {
	loadRegular()
	return reg
}

// Regular returns a collection of all available Go font faces.
func Collection() []font.FontFace {
	loadRegular()
	once.Do(func() {
		register(font.Font{Style: font.Italic}, goitalic.TTF)
		register(font.Font{Weight: font.Bold}, gobold.TTF)
		register(font.Font{Style: font.Italic, Weight: font.Bold}, gobolditalic.TTF)
		register(font.Font{Weight: font.Medium}, gomedium.TTF)
		register(font.Font{Weight: font.Medium, Style: font.Italic}, gomediumitalic.TTF)
		register(font.Font{Variant: "Mono"}, gomono.TTF)
		register(font.Font{Variant: "Mono", Weight: font.Bold}, gomonobold.TTF)
		register(font.Font{Variant: "Mono", Weight: font.Bold, Style: font.Italic}, gomonobolditalic.TTF)
		register(font.Font{Variant: "Mono", Style: font.Italic}, gomonoitalic.TTF)
		register(font.Font{Variant: "Smallcaps"}, gosmallcaps.TTF)
		register(font.Font{Variant: "Smallcaps", Style: font.Italic}, gosmallcapsitalic.TTF)
		// Ensure that any outside appends will not reuse the backing store.
		n := len(collection)
		collection = collection[:n:n]
	})
	return collection
}

func register(fnt font.Font, ttf []byte) {
	face, err := opentype.Parse(ttf)
	if err != nil {
		panic(fmt.Errorf("failed to parse font: %v", err))
	}
	fnt.Typeface = "Go"
	collection = append(collection, font.FontFace{Font: fnt, Face: face})
}
