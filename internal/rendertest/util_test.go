package rendertest

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"testing"

	"gioui.org/app/headless"
	"gioui.org/f32"
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

func drawImage(t *testing.T, size int, ops *op.Ops, draw func(o *op.Ops)) (im *image.RGBA, err error) {
	sz := image.Point{X: size, Y: size}
	w := newWindow(t, sz.X, sz.Y)
	draw(ops)
	if err := w.Frame(ops); err != nil {
		return nil, err
	}
	return w.Screenshot()
}

func run(t *testing.T, f func(o *op.Ops), c func(r result)) {
	// draw a few times and check that it is correct each time, to
	// ensure any caching effects still generate the correct images.
	var img *image.RGBA
	var err error
	ops := new(op.Ops)
	for i := 0; i < 3; i++ {
		ops.Reset()
		img, err = drawImage(t, 128, ops, f)
		if err != nil {
			t.Error("error rendering:", err)
			return
		}
		// check for a reference image and make sure we are identical.
		if !verifyRef(t, img, 0) {
			name := fmt.Sprintf("%s-%d-bad.png", t.Name(), i)
			if err := saveImage(name, img); err != nil {
				t.Error(err)
			}
		}
		c(result{t: t, img: img})
	}

	if *dumpImages {
		if err := saveImage(t.Name()+".png", img); err != nil {
			t.Error(err)
		}
	}
}

func frame(f func(o *op.Ops), c func(r result)) frameT {
	return frameT{f: f, c: c}
}

type frameT struct {
	f func(o *op.Ops)
	c func(r result)
}

// multiRun is used to run test cases over multiple frames, typically
// to test caching interactions.
func multiRun(t *testing.T, frames ...frameT) {
	// draw a few times and check that it is correct each time, to
	// ensure any caching effects still generate the correct images.
	var img *image.RGBA
	var err error
	sz := image.Point{X: 128, Y: 128}
	w := newWindow(t, sz.X, sz.Y)
	ops := new(op.Ops)
	for i := range frames {
		ops.Reset()
		frames[i].f(ops)
		if err := w.Frame(ops); err != nil {
			t.Errorf("rendering failed: %v", err)
			continue
		}
		img, err = w.Screenshot()
		if err != nil {
			t.Errorf("screenshot failed: %v", err)
			continue
		}
		// Check for a reference image and make sure they are identical.
		ok := verifyRef(t, img, i)
		if frames[i].c != nil {
			frames[i].c(result{t: t, img: img})
		}
		if *dumpImages || !ok {
			name := t.Name() + ".png"
			if i != 0 {
				name = t.Name() + "_" + strconv.Itoa(i) + ".png"
			}
			if err := saveImage(name, img); err != nil {
				t.Error(err)
			}
		}
	}

}

func verifyRef(t *testing.T, img *image.RGBA, frame int) (ok bool) {
	// ensure identical to ref data
	path := filepath.Join("refs", t.Name()+".png")
	if frame != 0 {
		path = filepath.Join("refs", t.Name()+"_"+strconv.Itoa(frame)+".png")
	}
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
				return false
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
	return diff < 10
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

func newWindow(t testing.TB, width, height int) *headless.Window {
	w, err := headless.NewWindow(width, height)
	if err != nil {
		t.Skipf("failed to create headless window, skipping: %v", err)
	}
	t.Cleanup(w.Release)
	return w
}

func scale(sx, sy float32) op.TransformOp {
	return op.Affine(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(sx, sy)))
}
