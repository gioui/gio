package opentype

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"gioui.org/internal/ops"
	"gioui.org/op"
	"gioui.org/text"
)

func TestCollectionAsFace(t *testing.T) {
	// Load two fonts with disjoint glyphs. Font 1 supports only '1', and font 2 supports only '2'.
	// The fonts have different glyphs for the replacement character (".notdef").
	font1, ttf1, err := decompressFontFile("testdata/only1.ttf.gz")
	if err != nil {
		t.Fatalf("failed to load test font 1: %v", err)
	}
	font2, ttf2, err := decompressFontFile("testdata/only2.ttf.gz")
	if err != nil {
		t.Fatalf("failed to load test font 2: %v", err)
	}

	otc := mergeFonts(ttf1, ttf2)
	coll, err := ParseCollection(otc)
	if err != nil {
		t.Fatalf("failed to load merged test font: %v", err)
	}

	shapeValid1, err := shapeRune(font1, '1')
	if err != nil {
		t.Fatalf("failed shaping valid glyph with font 1: %v", err)
	}
	shapeInvalid1, err := shapeRune(font1, '3')
	if err != nil {
		t.Fatalf("failed shaping invalid glyph with font 1: %v", err)
	}
	shapeValid2, err := shapeRune(font2, '2')
	if err != nil {
		t.Fatalf("failed shaping valid glyph with font 2: %v", err)
	}
	shapeInvalid2, err := shapeRune(font2, '3') // Same invalid glyph as before to test replacement glyph difference
	if err != nil {
		t.Fatalf("failed shaping invalid glyph with font 2: %v", err)
	}
	shapeCollValid1, err := shapeRune(coll, '1')
	if err != nil {
		t.Fatalf("failed shaping valid glyph for font 1 with font collection: %v", err)
	}
	shapeCollValid2, err := shapeRune(coll, '2')
	if err != nil {
		t.Fatalf("failed shaping valid glyph for font 2 with font collection: %v", err)
	}
	shapeCollInvalid, err := shapeRune(coll, '4') // Different invalid glyph to confirm use of the replacement glyph
	if err != nil {
		t.Fatalf("failed shaping invalid glyph with font collection: %v", err)
	}

	// All shapes from the original fonts should be distinct because the glyphs are distinct, including the replacement
	// glyphs.
	distinctShapes := []op.CallOp{shapeValid1, shapeInvalid1, shapeValid2, shapeInvalid2}
	for i := 0; i < len(distinctShapes); i++ {
		for j := i + 1; j < len(distinctShapes); j++ {
			if areShapesEqual(distinctShapes[i], distinctShapes[j]) {
				t.Errorf("font shapes %d and %d are not distinct", i, j)
			}
		}
	}

	// Font collections should render glyphs from the first supported font. Replacement glyphs should come from the
	// first font in all cases.
	if !areShapesEqual(shapeCollValid1, shapeValid1) {
		t.Error("font collection did not render the valid glyph using font 1")
	}
	if !areShapesEqual(shapeCollValid2, shapeValid2) {
		t.Error("font collection did not render the valid glyph using font 2")
	}
	if !areShapesEqual(shapeCollInvalid, shapeInvalid1) {
		t.Error("font collection did not render the invalid glyph using the replacement from font 1")
	}
}

func TestEmptyString(t *testing.T) {
	face, err := Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}

	ppem := fixed.I(200)

	lines, err := face.Layout(ppem, 2000, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) == 0 {
		t.Fatalf("Layout returned no lines for empty string; expected 1")
	}
	l := lines[0]
	exp, err := face.font.Bounds(new(sfnt.Buffer), ppem, font.HintingFull)
	if err != nil {
		t.Fatal(err)
	}
	if got := l.Bounds; got != exp {
		t.Errorf("got bounds %+v for empty string; expected %+v", got, exp)
	}
}

func decompressFontFile(name string) (*Font, []byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open file for reading: %s: %v", name, err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, nil, fmt.Errorf("font file contains invalid gzip data: %v", err)
	}
	src, err := ioutil.ReadAll(gz)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decompress font file: %v", err)
	}
	fnt, err := Parse(src)
	if err != nil {
		return nil, nil, fmt.Errorf("file did not contain a valid font: %v", err)
	}
	return fnt, src, nil
}

// mergeFonts produces a trivial OpenType Collection (OTC) file for two source fonts.
// It makes many assumptions and is not meant for general use.
// For file format details, see https://docs.microsoft.com/en-us/typography/opentype/spec/otff
// For a robust tool to generate these files, see https://pypi.org/project/afdko/
func mergeFonts(ttf1, ttf2 []byte) []byte {
	// Locations to place the two embedded fonts. All of the offsets to the fonts' internal tables will need to be
	// shifted from the start of the file by the appropriate amount, and then everything will work as expected.
	offset1 := uint32(20) // Length of OpenType collection headers
	offset2 := offset1 + uint32(len(ttf1))

	var buf bytes.Buffer
	_, _ = buf.Write([]byte("ttcf\x00\x01\x00\x00\x00\x00\x00\x02"))
	_ = binary.Write(&buf, binary.BigEndian, offset1)
	_ = binary.Write(&buf, binary.BigEndian, offset2)

	// Inline function to copy a font into the collection verbatim, except for adding an offset to all of the font's
	// table positions.
	copyOffsetTTF := func(ttf []byte, offset uint32) {
		_, _ = buf.Write(ttf[:12])
		numTables := binary.BigEndian.Uint16(ttf[4:6])
		for i := uint16(0); i < numTables; i++ {
			p := 12 + 16*i
			_, _ = buf.Write(ttf[p : p+8])
			tblLoc := binary.BigEndian.Uint32(ttf[p+8:p+12]) + offset
			_ = binary.Write(&buf, binary.BigEndian, tblLoc)
			_, _ = buf.Write(ttf[p+12 : p+16])
		}
		_, _ = buf.Write(ttf[12+16*numTables:])
	}
	copyOffsetTTF(ttf1, offset1)
	copyOffsetTTF(ttf2, offset2)

	return buf.Bytes()
}

// shapeRune uses a given Face to shape exactly one rune at a fixed size, then returns the resulting shape data.
func shapeRune(f text.Face, r rune) (op.CallOp, error) {
	ppem := fixed.I(200)
	lines, err := f.Layout(ppem, 2000, strings.NewReader(string(r)))
	if err != nil {
		return op.CallOp{}, err
	}
	if len(lines) != 1 {
		return op.CallOp{}, fmt.Errorf("unexpected rendering for \"U+%08X\": got %d lines (expected: 1)", r, len(lines))
	}
	return f.Shape(ppem, lines[0].Layout), nil
}

// areShapesEqual returns true iff both given text shapes are produced with identical operations.
func areShapesEqual(shape1, shape2 op.CallOp) bool {
	var ops1, ops2 op.Ops
	shape1.Add(&ops1)
	shape2.Add(&ops2)
	var r1, r2 ops.Reader
	r1.Reset(&ops1)
	r2.Reset(&ops2)
	for {
		encOp1, ok1 := r1.Decode()
		encOp2, ok2 := r2.Decode()
		if ok1 != ok2 {
			return false
		}
		if !ok1 {
			break
		}
		if len(encOp1.Refs) > 0 || len(encOp2.Refs) > 0 {
			panic("unexpected ops with refs in font shaping test")
		}
		if !bytes.Equal(encOp1.Data, encOp2.Data) {
			return false
		}
	}
	return true
}
