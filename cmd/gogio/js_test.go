// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"image"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	_ "gioui.org/unit" // the build tool adds it to go.mod, so keep it there
)

var headless = flag.Bool("headless", true, "run end-to-end tests in headless mode")

func TestJSOnChrome(t *testing.T) {
	t.Parallel()

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

		// The default would be use-gl=desktop when there's a GPU we can
		// use, falling back to use-gl=swiftshader otherwise or when we
		// are running in headless mode. Swiftshader allows full WebGL
		// support with just a CPU.
		//
		// Unfortunately, many Linux distros like Arch and Alpine
		// package Chromium without Swiftshader, so we can't rely on the
		// defaults above. use-gl=egl works on any machine with a GPU,
		// even if we run Chrome in headless mode, which is OK for now.
		//
		// TODO(mvdan): remove all of this once these issues are fixed:
		//
		//    https://bugs.archlinux.org/task/64307
		//    https://gitlab.alpinelinux.org/alpine/aports/issues/10920
		chromedp.Flag("use-gl", "egl"),
	)

	actx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(actx,
		// Send all logf/errf calls to t.Logf
		chromedp.WithLogf(t.Logf),
	)
	defer cancel()

	if err := chromedp.Run(ctx); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			t.Skipf("test requires Chrome to be installed: %v", err)
			return
		}
		t.Fatal(err)
	}
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			switch ev.Type {
			case "log", "info", "warning", "error":
				var args strings.Builder
				for i, arg := range ev.Args {
					if i > 0 {
						args.WriteString(", ")
					}
					args.Write(arg.Value)
				}
				t.Logf("console %s: %s", ev.Type, args.String())
			}
		}
	})

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
	wantColor(t, img, 5, 5, 0xdede, 0xadad, 0xbebe)
	wantColor(t, img, 595, 595, 0xdede, 0xadad, 0xbebe)
}

func wantColor(t *testing.T, img image.Image, x, y int, r, g, b uint32) {
	color := img.At(x, y)
	r_, g_, b_, _ := color.RGBA()
	if r_ != r || g_ != g || b_ != b {
		t.Errorf("got 0x%04x%04x%04x at (%d,%d), want 0x%04x%04x%04x",
			r_, g_, b_, x, y, r, g, b)
	}
}
