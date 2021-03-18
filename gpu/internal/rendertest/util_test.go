// SPDX-License-Identifier: Unlicense OR MIT

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

	"golang.org/x/image/colornames"

	"gioui.org/f32"
	"gioui.org/gpu/headless"
	"gioui.org/internal/f32color"
	"gioui.org/op"
	"gioui.org/op/paint"
)

var (
	dumpImages = flag.Bool("saveimages", false, "save test images")
	squares    paint.ImageOp
)

var (
	red     = f32color.RGBAToNRGBA(colornames.Red)
	green   = f32color.RGBAToNRGBA(colornames.Green)
	blue    = f32color.RGBAToNRGBA(colornames.Blue)
	magenta = f32color.RGBAToNRGBA(colornames.Magenta)
	yellow  = f32color.RGBAToNRGBA(colornames.Yellow)
	black   = f32color.RGBAToNRGBA(colornames.Black)
	white   = f32color.RGBAToNRGBA(colornames.White)
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
		t.Errorf("image is a %T, expected *image.RGBA", r)
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
	const delta = 0.01 // magic value obtained from experimentation.
	return yiqEqApprox(c1, c2, delta)
}

// yiqEqApprox compares the colors of 2 pixels, in the NTSC YIQ color space,
// as described in:
//
//   Measuring perceived color difference using YIQ NTSC
//   transmission color space in mobile applications.
//   Yuriy Kotsarenko, Fernando Ramos.
//
// An electronic version is available at:
//
// - http://www.progmat.uaem.mx:8080/artVol2Num2/Articulo3Vol2Num2.pdf
func yiqEqApprox(c1, c2 color.RGBA, d2 float64) bool {
	const max = 35215.0 // difference between 2 maximally different pixels.

	var (
		r1 = float64(c1.R)
		g1 = float64(c1.G)
		b1 = float64(c1.B)

		r2 = float64(c2.R)
		g2 = float64(c2.G)
		b2 = float64(c2.B)

		y1 = r1*0.29889531 + g1*0.58662247 + b1*0.11448223
		i1 = r1*0.59597799 - g1*0.27417610 - b1*0.32180189
		q1 = r1*0.21147017 - g1*0.52261711 + b1*0.31114694

		y2 = r2*0.29889531 + g2*0.58662247 + b2*0.11448223
		i2 = r2*0.59597799 - g2*0.27417610 - b2*0.32180189
		q2 = r2*0.21147017 - g2*0.52261711 + b2*0.31114694

		y = y1 - y2
		i = i1 - i2
		q = q1 - q2

		diff = 0.5053*y*y + 0.299*i*i + 0.1957*q*q
	)
	return diff <= max*d2
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
