// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/chromedp/chromedp"

	_ "gioui.org/unit" // the build tool adds it to go.mod, so keep it there
)

var headless = flag.Bool("headless", true, "run end-to-end tests in headless mode")

func TestJSOnChrome(t *testing.T) {
	// First, build the app.
	dir, err := ioutil.TempDir("", "gio-endtoend-js")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// TODO(mvdan): This is inefficient, as we link the gogio tool every time.
	// Consider options in the future. On the plus side, this is simple.
	cmd := exec.Command("go", "run", ".", "-target=js", "-o="+dir, "testdata/red.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("could not build app: %s:\n%s", err, out)
	}

	// Second, start Chrome.
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", *headless),

		// We need use-gl=egl instead of the default of use-gl=desktop;
		// "desktop" doesn't seem to work when we're in headless mode.
		// TODO(mvdan): Does egl require a GPU? If so, consider
		// use-gl=swiftshader, which will use CPU-based rendering. That
		// might be necessary for some CI or headless environments.
		chromedp.Flag("use-gl", "egl"),
	)

	actx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(actx)
	defer cancel()

	if err := chromedp.Run(ctx); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			t.Skipf("test requires Chrome to be installed: %v", err)
			return
		}
		t.Fatal(err)
	}

	// Third, serve the app folder, set the browser tab dimensions, and
	// navigate to the folder.
	ts := httptest.NewServer(http.FileServer(http.Dir(dir)))
	defer ts.Close()

	if err := chromedp.Run(ctx,
		// A small window with 2x HiDPI.
		chromedp.EmulateViewport(300, 300, chromedp.EmulateScale(2.0)),
		chromedp.Navigate(ts.URL),
	); err != nil {
		t.Fatal(err)
	}

	// Finally, run the test.

	// 1: Once the canvas is ready, grab a screenshot to check that the
	//    entirety of the viewport is red, as per the background color.
	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.WaitReady("canvas", chromedp.ByQuery),
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		t.Fatal(err)
	}
	size := img.Bounds().Size()
	wantSize := 600 // 300px at 2.0 scaling factor
	if size.X != wantSize || size.Y != wantSize {
		t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
			wantSize, wantSize, size.X, size.Y)
	}
	wantColor := func(x, y int, r, g, b, a uint32) {
		color := img.At(x, y)
		r_, g_, b_, a_ := color.RGBA()
		if r_ != r || g_ != g || b_ != b || a_ != a {
			t.Errorf("got 0x%04x%04x%04x%04x at (%d,%d), want 0x%04x%04x%04x%04x",
				r_, g_, b_, a_, x, y, r, g, b, a)
		}
	}
	wantColor(5, 5, 0xffff, 0x0, 0x0, 0xffff)
	wantColor(595, 595, 0xffff, 0x0, 0x0, 0xffff)
}
