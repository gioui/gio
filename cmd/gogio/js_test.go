// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	_ "gioui.org/unit" // the build tool adds it to go.mod, so keep it there
)

type JSTestDriver struct {
	driverBase

	// ctx is the chromedp context.
	ctx context.Context
}

func (d *JSTestDriver) Start(path string, width, height int) {
	if raceEnabled {
		d.Skipf("js/wasm doesn't support -race; skipping")
	}

	// First, build the app.
	dir := d.tempDir("gio-endtoend-js")
	d.gogio("-target=js", "-o="+dir, path)

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
	d.Cleanup(cancel)

	ctx, cancel := chromedp.NewContext(actx,
		// Send all logf/errf calls to t.Logf
		chromedp.WithLogf(d.Logf),
	)
	d.Cleanup(cancel)
	d.ctx = ctx

	if err := chromedp.Run(ctx); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			d.Skipf("test requires Chrome to be installed: %v", err)
			return
		}
		d.Fatal(err)
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
				d.Logf("console %s: %s", ev.Type, args.String())
			}
		}
	})

	// Third, serve the app folder, set the browser tab dimensions, and
	// navigate to the folder.
	ts := httptest.NewServer(http.FileServer(http.Dir(dir)))
	d.Cleanup(ts.Close)

	if err := chromedp.Run(ctx,
		chromedp.EmulateViewport(int64(width), int64(height)),
		chromedp.Navigate(ts.URL),
	); err != nil {
		d.Fatal(err)
	}

	// Wait for the gio app to render.
	d.waitForFrame()
}

func (d *JSTestDriver) Screenshot() image.Image {
	var buf []byte
	if err := chromedp.Run(d.ctx,
		chromedp.CaptureScreenshot(&buf),
	); err != nil {
		d.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf))
	if err != nil {
		d.Fatal(err)
	}
	return img
}

func (d *JSTestDriver) Click(x, y int) {
	if err := chromedp.Run(d.ctx,
		chromedp.MouseClickXY(float64(x), float64(y)),
	); err != nil {
		d.Fatal(err)
	}

	// Wait for the gio app to render after this click.
	d.waitForFrame()
}
