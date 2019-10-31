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
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/BurntSushi/xgbutil"
	"github.com/BurntSushi/xgbutil/xgraphics"
)

type X11TestDriver struct {
	t *testing.T

	// conn holds the connection to X.
	conn *xgbutil.XUtil
}

func (d *X11TestDriver) Start(t_ *testing.T, path string, width, height int) (cleanups []func()) {
	d.t = t_

	// Pick a random display number between 1 and 100,000. Most machines
	// will only be using :0, so there's only a 0.001% chance of two
	// concurrent test runs to run into a conflict.
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	display := fmt.Sprintf(":%d", rnd.Intn(100000)+1)

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
	xflags = append(xflags, display)
	if _, err := exec.LookPath(xprog); err != nil {
		d.t.Skipf("%s needed to run with -headless=%t", xprog, *headless)
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
			// This socket path isn't terribly portable, but the xgb
			// library we use does the same, and we only really care
			// about Linux here.
			socket := fmt.Sprintf("/tmp/.X11-unix/X%s", display[1:])
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
		cmd.Env = append(os.Environ(), "DISPLAY="+display)
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

	// Finally, connect to the X server.
	xgb.Logger.SetOutput(testLogWriter{d.t})
	xgbutil.Logger.SetOutput(testLogWriter{d.t})
	conn, err := xgbutil.NewConnDisplay(display)
	if err != nil {
		d.t.Fatal(err)
	}
	d.conn = conn
	cleanups = append(cleanups, func() {
		conn.Conn().Close()
		// TODO(mvdan): Figure out a way to remove this sleep
		// without introducing a panic. The xgb code will
		// encounter a panic if the Xorg server exits before xgb
		// has shut down fully.
		// See: https://github.com/BurntSushi/xgb/pull/44
		time.Sleep(10 * time.Millisecond)
	})

	// Wait for the gio app to render.
	// TODO(mvdan): do this properly, e.g. via waiting for log lines
	// from the gio program.
	time.Sleep(400 * time.Millisecond)

	return cleanups
}

func (d *X11TestDriver) Screenshot() image.Image {
	img, err := xgraphics.NewDrawable(d.conn, xproto.Drawable(d.conn.RootWin()))
	if err != nil {
		d.t.Fatal(err)
	}
	return img
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
