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

var (
	once       sync.Once
	collection *text.Collection
)

func Collection() *text.Collection {
	once.Do(func() {
		c := new(text.Collection)
		register(c, text.Font{}, goregular.TTF)
		register(c, text.Font{Style: text.Italic}, goitalic.TTF)
		register(c, text.Font{Weight: text.Bold}, gobold.TTF)
		register(c, text.Font{Style: text.Italic, Weight: text.Bold}, gobolditalic.TTF)
		register(c, text.Font{Weight: text.Medium}, gomedium.TTF)
		register(c, text.Font{Weight: text.Medium, Style: text.Italic}, gomediumitalic.TTF)
		register(c, text.Font{Variant: "Mono"}, gomono.TTF)
		register(c, text.Font{Variant: "Mono", Weight: text.Bold}, gomonobold.TTF)
		register(c, text.Font{Variant: "Mono", Weight: text.Bold, Style: text.Italic}, gomonobolditalic.TTF)
		register(c, text.Font{Variant: "Mono", Style: text.Italic}, gomonoitalic.TTF)
		register(c, text.Font{Variant: "Mono", Style: text.Italic}, gomonoitalic.TTF)
		register(c, text.Font{Variant: "Smallcaps"}, gosmallcaps.TTF)
		register(c, text.Font{Variant: "Smallcaps", Style: text.Italic}, gosmallcapsitalic.TTF)
		collection = c
	})
	return collection
}

func register(c *text.Collection, fnt text.Font, ttf []byte) {
	face, err := opentype.Parse(ttf)
	if err != nil {
		panic(fmt.Sprintf("failed to parse font: %v", err))
	}
	fnt.Typeface = "Go"
	c.Register(fnt, face)
}
