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

	"gioui.org/font/opentype"
	"gioui.org/text"
)

var (
	once         sync.Once
	collection   []text.FontFace
	onceHB       sync.Once
	collectionHB []text.FontFace
)

func Collection() []text.FontFace {
	once.Do(func() {
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
		register(text.Font{Variant: "Smallcaps"}, gosmallcaps.TTF)
		register(text.Font{Variant: "Smallcaps", Style: text.Italic}, gosmallcapsitalic.TTF)
		// Ensure that any outside appends will not reuse the backing store.
		n := len(collection)
		collection = collection[:n:n]
	})
	return collection
}

func CollectionHB() []text.FontFace {
	onceHB.Do(func() {
		registerHB(text.Font{}, goregular.TTF)
		registerHB(text.Font{Style: text.Italic}, goitalic.TTF)
		registerHB(text.Font{Weight: text.Bold}, gobold.TTF)
		registerHB(text.Font{Style: text.Italic, Weight: text.Bold}, gobolditalic.TTF)
		registerHB(text.Font{Weight: text.Medium}, gomedium.TTF)
		registerHB(text.Font{Weight: text.Medium, Style: text.Italic}, gomediumitalic.TTF)
		registerHB(text.Font{Variant: "Mono"}, gomono.TTF)
		registerHB(text.Font{Variant: "Mono", Weight: text.Bold}, gomonobold.TTF)
		registerHB(text.Font{Variant: "Mono", Weight: text.Bold, Style: text.Italic}, gomonobolditalic.TTF)
		registerHB(text.Font{Variant: "Mono", Style: text.Italic}, gomonoitalic.TTF)
		registerHB(text.Font{Variant: "Smallcaps"}, gosmallcaps.TTF)
		registerHB(text.Font{Variant: "Smallcaps", Style: text.Italic}, gosmallcapsitalic.TTF)
		// Ensure that any outside appends will not reuse the backing store.
		n := len(collectionHB)
		collectionHB = collectionHB[:n:n]
	})
	return collectionHB
}

func registerHB(fnt text.Font, ttf []byte) {
	face, err := opentype.ParseHarfbuzz(ttf)
	if err != nil {
		panic(fmt.Errorf("failed to parse font: %v", err))
	}
	fnt.Typeface = "Go"
	collectionHB = append(collectionHB, text.FontFace{Font: fnt, Face: face})
}

func register(fnt text.Font, ttf []byte) {
	face, err := opentype.Parse(ttf)
	if err != nil {
		panic(fmt.Errorf("failed to parse font: %v", err))
	}
	fnt.Typeface = "Go"
	collection = append(collection, text.FontFace{Font: fnt, Face: face})
}
