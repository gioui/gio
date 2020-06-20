package rendertest

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"path/filepath"
	"testing"

	"gioui.org/app/headless"
	"gioui.org/op"
	"gioui.org/op/paint"
	"golang.org/x/image/colornames"
)

var (
	dumpImages = flag.Bool("saveimages", false, "save test images")
	squares    paint.ImageOp
)

func init() {
	// build the texture we use for testing
	size := 512
	sub := size / 4
	im := image.NewNRGBA(image.Rect(0, 0, size, size))
	c1, c2 := image.NewUniform(colornames.Green), image.NewUniform(colornames.Blue)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			c1, c2 = c2, c1
			draw.Draw(im, image.Rect(r*sub, c*sub, r*sub+sub, c*sub+sub), c1, image.Point{}, draw.Over)
		}
		c1, c2 = c2, c1
	}
	squares = paint.NewImageOp(im)
}

func drawImage(size int, draw func(o *op.Ops)) (im *image.RGBA, err error) {
	sz := image.Point{X: size, Y: size}
	w, err := headless.NewWindow(sz.X, sz.Y)
	if err != nil {
		return im, err
	}
	ops := new(op.Ops)
	draw(ops)
	w.Frame(ops)
	return w.Screenshot()
}

func run(t *testing.T, f func(o *op.Ops)) result {
	img, err := drawImage(128, f)
	if err != nil {
		t.Error("error rendering:", err)
	}

	// check for a reference image and make sure we are identical.
	ok := verifyRef(t, img)

	if *dumpImages || !ok {
		if err := saveImage(t.Name()+".png", img); err != nil {
			t.Error(err)
		}
	}
	return result{t: t, img: img}
}

func verifyRef(t *testing.T, img *image.RGBA) (ok bool) {
	// ensure identical to ref data
	path := filepath.Join("refs", t.Name()+".png")
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Error("could not open ref:", err)
		return
	}
	r, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Error("could not decode ref:", err)
		return
	}
	ref, ok := r.(*image.RGBA)
	if !ok {
		t.Error("ref image note RGBA")
		return
	}
	if len(ref.Pix) != len(img.Pix) {
		t.Error("not equal to ref (len)")
		return false
	}
	bnd := img.Bounds()
	for x := bnd.Min.X; x < bnd.Max.X; x++ {
		for y := bnd.Min.Y; y < bnd.Max.Y; y++ {
			c1, c2 := ref.RGBAAt(x, y), img.RGBAAt(x, y)
			if !colorsClose(c1, c2) {
				t.Error("not equal to ref at", x, y, " ", c1, c2)
			}
		}
	}
	return true
}

func colorsClose(c1, c2 color.RGBA) bool {
	return close(c1.A, c2.A) && close(c1.R, c2.R) && close(c1.G, c2.G) && close(c1.B, c2.B)
}

func close(b1, b2 uint8) bool {
	if b1 > b2 {
		b1, b2 = b2, b1
	}
	diff := b2 - b1
	return diff < 20
}

func (r result) expect(x, y int, col color.RGBA) {
	if r.img == nil {
		return
	}
	c := r.img.RGBAAt(x, y)
	if !colorsClose(c, col) {
		r.t.Error("expected ", col, " at ", "(", x, ",", y, ") but got ", c)
	}
}

type result struct {
	t   *testing.T
	img *image.RGBA
}

func saveImage(file string, img image.Image) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return ioutil.WriteFile(file, buf.Bytes(), 0666)
}
