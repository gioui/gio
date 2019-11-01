// SPDX-License-Identifier: Unlicense OR MIT

// TODO(mvdan): come up with an end-to-end platform interface, including methods
// like "take screenshot" or "close app", so that we can run the same tests on
// all supported platforms without writing them many times.

package main_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type X11TestDriver struct {
	t *testing.T

	display string
}

func (d *X11TestDriver) Start(t_ *testing.T, path string, width, height int) (cleanups []func()) {
	d.t = t_

	// Pick a random display number between 1 and 100,000. Most machines
	// will only be using :0, so there's only a 0.001% chance of two
	// concurrent test runs to run into a conflict.
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	d.display = fmt.Sprintf(":%d", rnd.Intn(100000)+1)

	var xprog string
	xflags := []string{
		"-wr", // we want a white background; the default is black
	}
	if *headless {
		xprog = "Xvfb" // virtual X server
		xflags = append(xflags, "-screen", "0", fmt.Sprintf("%dx%dx24", width, height))
	} else {
		xprog = "Xephyr" // nested X server as a window
		xflags = append(xflags, "-screen", fmt.Sprintf("%dx%d", width, height))
	}
	xflags = append(xflags, d.display)

	for _, prog := range []string{
		xprog,     // to run the X server
		"scrot",   // to take screenshots
		"xdotool", // to send input
	} {
		if _, err := exec.LookPath(prog); err != nil {
			d.t.Skipf("%s needed to run", prog)
		}
	}

	// First, build the app.
	dir, err := ioutil.TempDir("", "gio-endtoend-x11")
	if err != nil {
		d.t.Fatal(err)
	}
	cleanups = append(cleanups, func() { os.RemoveAll(dir) })

	bin := filepath.Join(dir, "red")
	cmd := exec.Command("go", "build", "-tags", "nowayland", "-o="+bin, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.t.Fatalf("could not build app: %s:\n%s", err, out)
	}

	var wg sync.WaitGroup
	cleanups = append(cleanups, wg.Wait)

	// First, start the X server.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, xprog, xflags...)
		combined := &bytes.Buffer{}
		cmd.Stdout = combined
		cmd.Stderr = combined
		if err := cmd.Start(); err != nil {
			d.t.Fatal(err)
		}
		cleanups = append(cleanups, cancel)
		cleanups = append(cleanups, func() {
			// Give Xserver a chance to exit gracefully, cleaning up
			// after itself in /tmp. After 10ms, the deferred cancel
			// above will signal an os.Kill.
			cmd.Process.Signal(os.Interrupt)
			time.Sleep(10 * time.Millisecond)
		})

		// Wait for up to 1s (100 * 10ms) for the X server to be ready.
		for i := 0; ; i++ {
			time.Sleep(10 * time.Millisecond)
			// This socket path isn't terribly portable, but it's
			// okay for now.
			socket := fmt.Sprintf("/tmp/.X11-unix/X%s", d.display[1:])
			if _, err := os.Stat(socket); err == nil {
				break
			}
			if i >= 100 {
				d.t.Fatalf("timed out waiting for %s", socket)
			}
		}

		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				// Print all output and error.
				io.Copy(os.Stdout, combined)
				d.t.Error(err)
			}
			wg.Done()
		}()
	}

	// Then, start our program on the X server above.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, bin)
		out := &bytes.Buffer{}
		cmd.Env = []string{"DISPLAY=" + d.display}
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Start(); err != nil {
			d.t.Fatal(err)
		}
		cleanups = append(cleanups, cancel)
		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				// Print all output and error.
				io.Copy(os.Stdout, out)
				d.t.Error(err)
			}
			wg.Done()
		}()
	}

	// Wait for the gio app to render.
	// TODO(mvdan): synchronize with the app instead
	time.Sleep(400 * time.Millisecond)

	return cleanups
}

func (d *X11TestDriver) Screenshot() image.Image {
	cmd := exec.Command("scrot", "--silent", "--overwrite", "/dev/stdout")
	cmd.Env = []string{"DISPLAY=" + d.display}
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.t.Errorf("%s", out)
		d.t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		d.t.Fatal(err)
	}
	return img
}

func (d *X11TestDriver) xdotool(args ...interface{}) {
	strs := make([]string, len(args))
	for i, arg := range args {
		strs[i] = fmt.Sprint(arg)
	}
	cmd := exec.Command("xdotool", strs...)
	cmd.Env = []string{"DISPLAY=" + d.display}
	if out, err := cmd.CombinedOutput(); err != nil {
		d.t.Errorf("%s", out)
		d.t.Fatal(err)
	}
}

func (d *X11TestDriver) Click(x, y int) {
	d.xdotool("mousemove", x, y)
	d.xdotool("click", "1")

	// TODO(mvdan): synchronize with the app instead
	time.Sleep(200 * time.Millisecond)
}

func TestX11(t *testing.T) {
	t.Parallel()

	runEndToEndTest(t, &X11TestDriver{})
}

// testLogWriter is a bit of a hack to redirect libraries that use a *log.Logger
// variable to instead send their logs to t.Logf.
//
// Since *log.Logger isn't an interface and can only take an io.Writer, all we
// can do is implement an io.Writer that sends its output to t.Logf. We end up
// with duplicate log prefixes, but that doesn't seem so bad.
type testLogWriter struct {
	t *testing.T
}

func (w testLogWriter) Write(p []byte) (n int, err error) {
	w.t.Logf("%s", p)
	return len(p), nil
}
