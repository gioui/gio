// SPDX-License-Identifier: Unlicense OR MIT

// TODO(mvdan): come up with an end-to-end platform interface, including methods
// like "take screenshot" or "close app", so that we can run the same tests on
// all supported platforms without writing them many times.

package main_test

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
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

func TestX11(t *testing.T) {
	t.Parallel()

	// TODO(mvdan): pick a random one between a large pool, and retry if
	// it's already taken.
	const display = ":15"

	var xprog string
	xflags := []string{"-wr"}
	if *headless {
		xprog = "Xvfb" // virtual X server
		xflags = append(xflags, "-screen", "0", "600x600x24")
	} else {
		xprog = "Xephyr" // nested X server as a window
		xflags = append(xflags, "-screen", "600x600")
	}
	xflags = append(xflags, display)
	if _, err := exec.LookPath(xprog); err != nil {
		t.Skipf("%s needed to run with -headless=%t", xprog, *headless)
	}

	// First, build the app.
	dir, err := ioutil.TempDir("", "gio-endtoend-x11")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	bin := filepath.Join(dir, "red")
	cmd := exec.Command("go", "build", "-o="+bin, "testdata/red.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("could not build app: %s:\n%s", err, out)
	}

	var wg sync.WaitGroup
	defer wg.Wait()

	// First, start the X server.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, xprog, xflags...)
		out := &bytes.Buffer{}
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer cancel()
		// TODO(mvdan): properly wait for the display to be ready instead.
		time.Sleep(200 * time.Millisecond)
		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				// Print all output and error.
				io.Copy(os.Stdout, out)
				t.Error(err)
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
			t.Fatal(err)
		}
		defer cancel()
		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				// Print all output and error.
				io.Copy(os.Stdout, out)
				t.Error(err)
			}
			wg.Done()
		}()
	}

	// Finally, run our tests. A connection to the X server is used to
	// interact with it.
	{
		if !testing.Verbose() {
			xgb.Logger.SetOutput(ioutil.Discard)
			xgbutil.Logger.SetOutput(ioutil.Discard)
		}
		xu, err := xgbutil.NewConnDisplay(display)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			xu.Conn().Close()
			// TODO(mvdan): Figure out a way to remove this sleep
			// without introducing a panic. The xgb code will
			// encounter a panic if the Xorg server exits before xgb
			// has shut down fully.
			// See: https://github.com/BurntSushi/xgb/pull/44
			time.Sleep(20 * time.Millisecond)
		}()

		// Wait for the gio app to render.
		// TODO(mvdan): do this properly, e.g. via waiting for log lines
		// from the gio program.
		time.Sleep(200 * time.Millisecond)

		img, err := xgraphics.NewDrawable(xu, xproto.Drawable(xu.RootWin()))
		if err != nil {
			t.Fatal(err)
		}

		size := img.Bounds().Size()
		wantSize := 600 // 300px at 2.0 scaling factor
		if size.X != wantSize || size.Y != wantSize {
			t.Fatalf("expected dimensions to be %d*%d, got %d*%d",
				wantSize, wantSize, size.X, size.Y)
		}
		wantColor(t, img, 5, 5, 0xffff, 0x0, 0x0, 0xffff)
		wantColor(t, img, 595, 595, 0xffff, 0x0, 0x0, 0xffff)
	}
}
