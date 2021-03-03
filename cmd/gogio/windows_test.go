// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"context"
	"image"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"golang.org/x/image/draw"
)

// Wine is tightly coupled with X11 at the moment, and we can reuse the same
// methods to automate screenshots and clicks. The main difference is how we
// build and run the app.

// The only quirk is that it seems impossible for the Wine window to take the
// entirety of the X server's dimensions, even if we try to resize it to take
// the entire display. It seems to want to leave some vertical space empty,
// presumably for window decorations or the "start" bar on Windows. To work
// around that, make the X server 50x50px bigger, and crop the screenshots back
// to the original size.

type WineTestDriver struct {
	X11TestDriver
}

func (d *WineTestDriver) Start(path string) {
	d.needPrograms("wine")

	// First, build the app.
	bin := filepath.Join(d.tempDir("gio-endtoend-windows"), "red.exe")
	flags := []string{"build", "-o=" + bin}
	if raceEnabled {
		if runtime.GOOS != "windows" {
			// cross-compilation disables CGo, which breaks -race.
			d.Skipf("can't cross-compile -race for Windows; skipping")
		}
		flags = append(flags, "-race")
	}
	flags = append(flags, path)
	cmd := exec.Command("go", flags...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "GOOS=windows")
	if out, err := cmd.CombinedOutput(); err != nil {
		d.Fatalf("could not build app: %s:\n%s", err, out)
	}

	var wg sync.WaitGroup
	d.Cleanup(wg.Wait)

	// Add 50x50px to the display dimensions, as discussed earlier.
	d.startServer(&wg, d.width+50, d.height+50)

	// Then, start our program via Wine on the X server above.
	{
		cacheDir, err := os.UserCacheDir()
		if err != nil {
			d.Fatal(err)
		}
		// Use a wine directory separate from the default ~/.wine, so
		// that the user's winecfg doesn't affect our test. This will
		// default to ~/.cache/gio-e2e-wine. We use the user's cache,
		// to reuse a previously set up wineprefix.
		wineprefix := filepath.Join(cacheDir, "gio-e2e-wine")

		// First, ensure that wineprefix is up to date with wineboot.
		// Wait for this separately from the first frame, as setting up
		// a new prefix might take 5s on its own.
		env := []string{
			"DISPLAY=" + d.display,
			"WINEDEBUG=fixme-all", // hide "fixme" noise
			"WINEPREFIX=" + wineprefix,

			// Disable wine-gecko (Explorer) and wine-mono (.NET).
			// Otherwise, if not installed, wineboot will get stuck
			// with a prompt to install them on the virtual X
			// display. Moreover, Gio doesn't need either, and wine
			// is faster without them.
			"WINEDLLOVERRIDES=mscoree,mshtml=",
		}
		{
			start := time.Now()
			cmd := exec.Command("wine", "wineboot", "-i")
			cmd.Env = env
			// Use a combined output pipe instead of CombinedOutput,
			// so that we only wait for the child process to exit,
			// and we don't need to wait for all of wine's
			// grandchildren to exit and stop writing. This is
			// relevant as wine leaves "wineserver" lingering for
			// three seconds by default, to be reused later.
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				d.Fatal(err)
			}
			cmd.Stderr = cmd.Stdout
			if err := cmd.Run(); err != nil {
				io.Copy(os.Stderr, stdout)
				d.Fatal(err)
			}
			d.Logf("set up WINEPREFIX in %s", time.Since(start))
		}

		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, "wine", bin)
		cmd.Env = env
		output, err := cmd.StdoutPipe()
		if err != nil {
			d.Fatal(err)
		}
		cmd.Stderr = cmd.Stdout
		d.output = output
		if err := cmd.Start(); err != nil {
			d.Fatal(err)
		}
		d.Cleanup(cancel)
		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				d.Error(err)
			}
			wg.Done()
		}()
	}
	// Wait for the gio app to render.
	d.waitForFrame()

	// xdotool seems to fail at actually moving the window if we use it
	// immediately after Gio is ready. Why?
	// We can't tell if the windowmove operation worked until we take a
	// screenshot, because the getwindowgeometry op reports the 0x0
	// coordinates even if the window wasn't moved properly.
	// A sleep of ~20ms seems to be enough on an idle laptop. Use 20x that.
	// TODO(mvdan): revisit this, when you have a spare three hours.
	time.Sleep(400 * time.Millisecond)
	id := d.xdotool("search", "--sync", "--onlyvisible", "--name", "Gio")
	d.xdotool("windowmove", "--sync", id, 0, 0)
}

func (d *WineTestDriver) Screenshot() image.Image {
	img := d.X11TestDriver.Screenshot()
	// Crop the screenshot back to the original dimensions.
	cropped := image.NewRGBA(image.Rect(0, 0, d.width, d.height))
	draw.Draw(cropped, cropped.Bounds(), img, image.Point{}, draw.Src)
	return cropped
}
