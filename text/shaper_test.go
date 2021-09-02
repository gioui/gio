package text

import (
	"testing"
)

var (
	testTF1 Typeface = "MockFace"
	testTF2 Typeface = "TestFace"
	testTF3 Typeface = "AnotherFace"
)

func TestClosestFontByWeight(t *testing.T) {
	c := newTestCache(
		Font{Style: Regular, Weight: Normal},
		Font{Style: Regular, Weight: Light},
		Font{Style: Regular, Weight: Bold},
		Font{Style: Italic, Weight: Thin},
	)
	weightOnlyTests := []struct {
		Lookup   Weight
		Expected Weight
	}{
		// Test for existing weights.
		{Lookup: Normal, Expected: Normal},
		{Lookup: Light, Expected: Light},
		{Lookup: Bold, Expected: Bold},
		// Test for missing weights.
		{Lookup: Thin, Expected: Light},
		{Lookup: ExtraLight, Expected: Light},
		{Lookup: Medium, Expected: Normal},
		{Lookup: SemiBold, Expected: Bold},
		{Lookup: ExtraBlack, Expected: Bold},
	}
	for _, test := range weightOnlyTests {
		got, ok := c.closestFont(Font{Typeface: testTF1, Weight: test.Lookup})
		if !ok {
			t.Fatalf("expected closest font for %v to exist", test.Lookup)
		}
		if got.Weight != test.Expected {
			t.Fatalf("got weight %v, expected %v", got.Weight, test.Expected)
		}
	}
	c = newTestCache(
		Font{Style: Regular, Weight: Light},
		Font{Style: Regular, Weight: Bold},
		Font{Style: Italic, Weight: Normal},
		Font{Typeface: testTF3, Style: Italic, Weight: Bold},
	)
	otherTests := []struct {
		Lookup         Font
		Expected       Font
		ExpectedToFail bool
	}{
		// Test for existing fonts.
		{
			Lookup:   Font{Typeface: testTF1, Weight: Light},
			Expected: Font{Typeface: testTF1, Style: Regular, Weight: Light},
		},
		{
			Lookup:   Font{Typeface: testTF1, Style: Italic, Weight: Normal},
			Expected: Font{Typeface: testTF1, Style: Italic, Weight: Normal},
		},
		// Test for missing fonts.
		{
			Lookup:   Font{Typeface: testTF1, Weight: Normal},
			Expected: Font{Typeface: testTF1, Style: Regular, Weight: Light},
		},
		{
			Lookup:   Font{Typeface: testTF3, Style: Italic, Weight: Normal},
			Expected: Font{Typeface: testTF3, Style: Italic, Weight: Bold},
		},
		{
			Lookup:   Font{Typeface: testTF1, Style: Italic, Weight: Thin},
			Expected: Font{Typeface: testTF1, Style: Italic, Weight: Normal},
		},
		{
			Lookup:   Font{Typeface: testTF1, Style: Italic, Weight: Bold},
			Expected: Font{Typeface: testTF1, Style: Italic, Weight: Normal},
		},
		{
			Lookup:         Font{Typeface: testTF2, Weight: Normal},
			ExpectedToFail: true,
		},
		{
			Lookup:         Font{Typeface: testTF2, Style: Italic, Weight: Normal},
			ExpectedToFail: true,
		},
	}
	for _, test := range otherTests {
		got, ok := c.closestFont(test.Lookup)
		if test.ExpectedToFail {
			if ok {
				t.Fatalf("expected closest font for %v to not exist", test.Lookup)
			} else {
				continue
			}
		}
		if !ok {
			t.Fatalf("expected closest font for %v to exist", test.Lookup)
		}
		if got != test.Expected {
			t.Fatalf("got %v, expected %v", got, test.Expected)
		}
	}
}

func newTestCache(fonts ...Font) *Cache {
	c := &Cache{faces: make(map[Font]*faceCache)}
	c.def = testTF1
	for _, font := range fonts {
		if font.Typeface == "" {
			font.Typeface = testTF1
		}
		c.faces[font] = &faceCache{face: nil}
	}
	return c
}
