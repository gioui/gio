// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bufio"
	"bytes"
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
	d.startServer(wg, d.width+50, d.height+50)

	// Then, start our program via Wine on the X server above.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, "wine", bin)
		cmd.Env = []string{"DISPLAY=" + d.display}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			d.Fatal(err)
		}
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			d.Fatal(err)
		}
		d.Cleanup(cancel)
		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				// Print stderr and error.
				io.Copy(os.Stdout, stderr)
				d.Error(err)
			}
			wg.Done()
		}()
		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "frame ready" {
					d.frameNotifs <- true
				}
			}
		}()

	}
	// Wait for the gio app to render.
	d.waitForFrame()

	// xdotool seems to fail at actually moving the window if we use it
	// immediately after Gio is ready. Why?
	// We can't tell if the windowmove operation worked until we take a
	// screenshot, because the getwindowgeometry op reports the 0x0
	// coordinates even if the window wasn't moved properly.
	// A sleep of ~20ms seems to be enough on an idle laptop. Use 10x that.
	// TODO(mvdan): revisit this, when you have a spare three hours.
	time.Sleep(200 * time.Millisecond)
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
