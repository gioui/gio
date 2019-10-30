// SPDX-License-Identifier: Unlicense OR MIT

// TODO(mvdan): come up with an end-to-end platform interface, including methods
// like "take screenshot" or "close app", so that we can run the same tests on
// all supported platforms without writing them many times.

package main_test

import (
	"bytes"
	"context"
	"fmt"
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

func TestX11(t *testing.T) {
	t.Parallel()

	// Pick a random display number between 1 and 100,000. Most machines
	// will only be using :0, so there's only a 0.001% chance of two
	// concurrent test runs to run into a conflict.
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))
	display := fmt.Sprintf(":%d", rnd.Intn(100000)+1)

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
		combined := &bytes.Buffer{}
		cmd.Stdout = combined
		cmd.Stderr = combined
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		defer cancel()
		defer func() {
			// Give Xserver a chance to exit gracefully, cleaning up
			// after itself in /tmp. After 10ms, the deferred cancel
			// above will signal an os.Kill.
			cmd.Process.Signal(os.Interrupt)
			time.Sleep(10 * time.Millisecond)
		}()

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
				t.Fatalf("timed out waiting for %s", socket)
			}
		}

		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				// Print all output and error.
				io.Copy(os.Stdout, combined)
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
			time.Sleep(10 * time.Millisecond)
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
		wantColor(t, img, 5, 5, 0xffff, 0x0, 0x0)
		wantColor(t, img, 595, 595, 0xffff, 0x0, 0x0)
	}
}
