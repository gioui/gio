// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bytes"
	"context"
	"errors"
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

type JSTestDriver struct {
	t *testing.T

	frameNotifs chan bool

	// ctx is the chromedp context.
	ctx context.Context
}

func (d *JSTestDriver) Start(t_ *testing.T, path string, width, height int) {
	d.frameNotifs = make(chan bool, 1)
	d.t = t_

	if raceEnabled {
		d.t.Skipf("js/wasm doesn't support -race; skipping")
	}

	// First, build the app.
	dir, err := ioutil.TempDir("", "gio-endtoend-js")
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(func() { os.RemoveAll(dir) })

	// TODO(mvdan): This is inefficient, as we link the gogio tool every time.
	// Consider options in the future. On the plus side, this is simple.
	cmd := exec.Command("go", "run", ".", "-target=js", "-o="+dir, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.t.Fatalf("could not build app: %s:\n%s", err, out)
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
	d.t.Cleanup(cancel)

	ctx, cancel := chromedp.NewContext(actx,
		// Send all logf/errf calls to t.Logf
		chromedp.WithLogf(d.t.Logf),
	)
	d.t.Cleanup(cancel)
	d.ctx = ctx

	if err := chromedp.Run(ctx); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			d.t.Skipf("test requires Chrome to be installed: %v", err)
			return
		}
		d.t.Fatal(err)
	}
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if ev.Type == "log" && len(ev.Args) == 1 &&
				// Note that the argument values are JSON.
				string(ev.Args[0].Value) == `"frame ready"` {

				d.frameNotifs <- true
				// These logs are expected. Don't show them.
				break
			}
			switch ev.Type {
			case "log", "info", "warning", "error":
				var args strings.Builder
				for i, arg := range ev.Args {
					if i > 0 {
						args.WriteString(", ")
					}
					args.Write(arg.Value)
				}
				d.t.Logf("console %s: %s", ev.Type, args.String())
			}
		}
	})

	// Third, serve the app folder, set the browser tab dimensions, and
	// navigate to the folder.
	ts := httptest.NewServer(http.FileServer(http.Dir(dir)))
	d.t.Cleanup(ts.Close)

	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate(ts.URL),
	); err != nil {
		d.t.Fatal(err)
	}

	// Wait for the gio app to render.
	<-d.frameNotifs
}

func (d *JSTestDriver) Screenshot() image.Image {
	var buf []byte
	if err := chromedp.Run(d.ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		d.t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		d.t.Fatal(err)
	}
	return img
}

func (d *JSTestDriver) Click(x, y int) {
	if err := chromedp.Run(d.ctx,
		chromedp.MouseClickXY(float64(x), float64(y)),
	); err != nil {
		d.t.Fatal(err)
	}

	// Wait for the gio app to render after this click.
	<-d.frameNotifs
}
