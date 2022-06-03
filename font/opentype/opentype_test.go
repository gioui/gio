package opentype

import (
	"strings"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"

	"gioui.org/io/system"
)

var english = system.Locale{
	Language:  "EN",
	Direction: system.LTR,
}

func TestEmptyString(t *testing.T) {
	face, err := Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}

	ppem := fixed.I(200)

	lines, err := face.Layout(ppem, 2000, english, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Fatalf("Layout returned no lines for empty string; expected 1")
	}
	l := lines[0]
	exp := fixed.Rectangle26_6{
		Min: fixed.Point26_6{
			Y: fixed.Int26_6(-12094),
		},
		Max: fixed.Point26_6{
			Y: fixed.Int26_6(2700),
		},
	}
	if got := l.Bounds; got != exp {
		t.Errorf("got bounds %+v for empty string; expected %+v", got, exp)
	}
}
