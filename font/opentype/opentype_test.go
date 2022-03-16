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
	"gioui.org/io/system"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/text"
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
func shapeRune(f text.Face, r rune) (clip.PathSpec, error) {
	ppem := fixed.I(200)
	lines, err := f.Layout(ppem, 2000, english, strings.NewReader(string(r)))
	if err != nil {
		return clip.PathSpec{}, err
	}
	if len(lines) != 1 {
		return clip.PathSpec{}, fmt.Errorf("unexpected rendering for \"U+%08X\": got %d lines (expected: 1)", r, len(lines))
	}
	return f.Shape(ppem, lines[0].Layout), nil
}

// areShapesEqual returns true iff both given text shapes are produced with identical operations.
func areShapesEqual(shape1, shape2 clip.PathSpec) bool {
	var ops1, ops2 op.Ops
	clip.Outline{Path: shape1}.Op().Push(&ops1).Pop()
	clip.Outline{Path: shape2}.Op().Push(&ops2).Pop()
	var r1, r2 ops.Reader
	r1.Reset(&ops1.Internal)
	r2.Reset(&ops2.Internal)
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
