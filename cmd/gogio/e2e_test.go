// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

var raceEnabled = false

var headless = flag.Bool("headless", true, "run end-to-end tests in headless mode")

const appid = "localhost.gogio.endtoend"

// TestDriver is implemented by each of the platforms we can run end-to-end
// tests on. None of its methods return any errors, as the errors are directly
// reported to testing.T via methods like Fatal.
type TestDriver interface {
	initBase(t *testing.T, width, height int)

	// Start opens the Gio app found at path. The driver should attempt to
	// run the app with the base driver's width and height, and the
	// platform's background should be white.
	//
	// When the function returns, the gio app must be ready to use on the
	// platform, with its initial frame fully drawn.
	Start(path string)

	// Screenshot takes a screenshot of the Gio app on the platform.
	Screenshot() image.Image

	// Click performs a pointer click at the specified coordinates,
	// including both press and release. It returns when the next frame is
	// fully drawn.
	Click(x, y int)
}

type driverBase struct {
	*testing.T

	width, height int

	output      io.Reader
	frameNotifs chan bool
}

func (d *driverBase) initBase(t *testing.T, width, height int) {
	d.T = t
	d.width, d.height = width, height
}

func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skipf("end-to-end tests tend to be slow")
	}

	t.Parallel()

	const (
		testdataWithGoImportPkgPath = "gioui.org/cmd/gogio/testdata"
		testdataWithRelativePkgPath = "testdata/testdata.go"
	)
	// Keep this list local, to not reuse TestDriver objects.
	subtests := []struct {
		name    string
		driver  TestDriver
		pkgPath string
	}{
		{"X11 using go import path", &X11TestDriver{}, testdataWithGoImportPkgPath},
		{"X11", &X11TestDriver{}, testdataWithRelativePkgPath},
		{"Wayland", &WaylandTestDriver{}, testdataWithRelativePkgPath},
		{"JS", &JSTestDriver{}, testdataWithRelativePkgPath},
		{"Android", &AndroidTestDriver{}, testdataWithRelativePkgPath},
		{"Windows", &WineTestDriver{}, testdataWithRelativePkgPath},
	}

	for _, subtest := range subtests {
		t.Run(subtest.name, func(t *testing.T) {
			subtest := subtest // copy the changing loop variable
			t.Parallel()
			runEndToEndTest(t, subtest.driver, subtest.pkgPath)
		})
	}
}

func runEndToEndTest(t *testing.T, driver TestDriver, pkgPath string) {
	size := image.Point{X: 800, Y: 600}
	driver.initBase(t, size.X, size.Y)

	t.Log("starting driver and gio app")
	driver.Start(pkgPath)

	beef := color.NRGBA{R: 0xde, G: 0xad, B: 0xbe, A: 0xff}
	white := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	black := color.NRGBA{R: 0x00, G: 0x00, B: 0x00, A: 0xff}
	gray := color.NRGBA{R: 0xbb, G: 0xbb, B: 0xbb, A: 0xff}
	red := color.NRGBA{R: 0xff, G: 0x00, B: 0x00, A: 0xff}

	// These are the four colors at the beginning.
	t.Log("taking initial screenshot")
	withRetries(t, 4*time.Second, func() error {
		img := driver.Screenshot()
		size = img.Bounds().Size() // override the default size
		return checkImageCorners(img, beef, white, black, gray)
	})

	// TODO(mvdan): implement this properly in the Wayland driver; swaymsg
	// almost works to automate clicks, but the button presses end up in the
	// wrong coordinates.
	if _, ok := driver.(*WaylandTestDriver); ok {
		return
	}

	// Click the first and last sections to turn them red.
	t.Log("clicking twice and taking another screenshot")
	driver.Click(1*(size.X/4), 1*(size.Y/4))
	driver.Click(3*(size.X/4), 3*(size.Y/4))
	withRetries(t, 4*time.Second, func() error {
		img := driver.Screenshot()
		return checkImageCorners(img, red, white, black, red)
	})
}

// withRetries keeps retrying fn until it succeeds, or until the timeout is hit.
// It uses a rudimentary kind of backoff, which starts with 100ms delays. As
// such, timeout should generally be in the order of seconds.
func withRetries(t *testing.T, timeout time.Duration, fn func() error) {
	t.Helper()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	backoff := 100 * time.Millisecond

	tries := 0
	var lastErr error
	for {
		if lastErr = fn(); lastErr == nil {
			return
		}
		tries++
		t.Logf("retrying after %s", backoff)

		// Use a timer instead of a sleep, so that the timeout can stop
		// the backoff early. Don't reuse this timer, since we're not in
		// a hot loop, and we don't want tricky code.
		backoffTimer := time.NewTimer(backoff)
		defer backoffTimer.Stop()

		select {
		case <-timeoutTimer.C:
			t.Errorf("last error: %v", lastErr)
			t.Fatalf("hit timeout of %s after %d tries", timeout, tries)
		case <-backoffTimer.C:
		}

		// Keep doubling it until a maximum. With the start at 100ms,
		// we'll do: 100ms, 200ms, 400ms, 800ms, 1.6s, and 2s forever.
		backoff *= 2
		if max := 2 * time.Second; backoff > max {
			backoff = max
		}
	}
}

type colorMismatch struct {
	x, y            int
	wantRGB, gotRGB [3]uint32
}

func (m colorMismatch) String() string {
	return fmt.Sprintf("%3d,%-3d got 0x%04x%04x%04x, want 0x%04x%04x%04x",
		m.x, m.y,
		m.gotRGB[0], m.gotRGB[1], m.gotRGB[2],
		m.wantRGB[0], m.wantRGB[1], m.wantRGB[2],
	)
}

func checkImageCorners(img image.Image, topLeft, topRight, botLeft, botRight color.Color) error {
	// The colors are split in four rectangular sections. Check the corners
	// of each of the sections. We check the corners left to right, top to
	// bottom, like when reading left-to-right text.

	size := img.Bounds().Size()
	var mismatches []colorMismatch

	checkColor := func(x, y int, want color.Color) {
		r, g, b, _ := want.RGBA()
		got := img.At(x, y)
		r_, g_, b_, _ := got.RGBA()
		if r_ != r || g_ != g || b_ != b {
			mismatches = append(mismatches, colorMismatch{
				x:       x,
				y:       y,
				wantRGB: [3]uint32{r, g, b},
				gotRGB:  [3]uint32{r_, g_, b_},
			})
		}
	}

	{
		minX, minY := 5, 5
		maxX, maxY := (size.X/2)-5, (size.Y/2)-5
		checkColor(minX, minY, topLeft)
		checkColor(maxX, minY, topLeft)
		checkColor(minX, maxY, topLeft)
		checkColor(maxX, maxY, topLeft)
	}
	{
		minX, minY := (size.X/2)+5, 5
		maxX, maxY := size.X-5, (size.Y/2)-5
		checkColor(minX, minY, topRight)
		checkColor(maxX, minY, topRight)
		checkColor(minX, maxY, topRight)
		checkColor(maxX, maxY, topRight)
	}
	{
		minX, minY := 5, (size.Y/2)+5
		maxX, maxY := (size.X/2)-5, size.Y-5
		checkColor(minX, minY, botLeft)
		checkColor(maxX, minY, botLeft)
		checkColor(minX, maxY, botLeft)
		checkColor(maxX, maxY, botLeft)
	}
	{
		minX, minY := (size.X/2)+5, (size.Y/2)+5
		maxX, maxY := size.X-5, size.Y-5
		checkColor(minX, minY, botRight)
		checkColor(maxX, minY, botRight)
		checkColor(minX, maxY, botRight)
		checkColor(maxX, maxY, botRight)
	}
	if n := len(mismatches); n > 0 {
		b := new(strings.Builder)
		fmt.Fprintf(b, "encountered %d color mismatches:\n", n)
		for _, m := range mismatches {
			fmt.Fprintf(b, "%s\n", m)
		}
		return errors.New(b.String())
	}
	return nil
}

func (d *driverBase) waitForFrame() {
	d.Helper()

	if d.frameNotifs == nil {
		// Start the goroutine that reads output lines and notifies of
		// new frames via frameNotifs. The test doesn't wait for this
		// goroutine to finish; it will naturally end when the output
		// reader reaches an error like EOF.
		d.frameNotifs = make(chan bool, 1)
		if d.output == nil {
			d.Fatal("need an output reader to be notified of frames")
		}
		go func() {
			scanner := bufio.NewScanner(d.output)
			for scanner.Scan() {
				line := scanner.Text()
				d.Log(line)
				if strings.Contains(line, "gio frame ready") {
					d.frameNotifs <- true
				}
			}
			// Since we're only interested in the output while the
			// app runs, and we don't know when it finishes here,
			// ignore "already closed" pipe errors.
			if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
				d.Errorf("reading app output: %v", err)
			}
		}()
	}

	// Unfortunately, there isn't a way to select on a test failing, since
	// testing.T doesn't have anything like a context or a "done" channel.
	//
	// We can't let selects block forever, since the default -test.timeout
	// is ten minutes - far too long for tests that take seconds.
	//
	// For now, a static short timeout is better than nothing. 5s is plenty
	// for our simple test app to render on any device.
	select {
	case <-d.frameNotifs:
	case <-time.After(5 * time.Second):
		d.Fatalf("timed out waiting for a frame to be ready")
	}
}

func (d *driverBase) needPrograms(names ...string) {
	d.Helper()
	for _, name := range names {
		if _, err := exec.LookPath(name); err != nil {
			d.Skipf("%s needed to run", name)
		}
	}
}

func (d *driverBase) tempDir(name string) string {
	d.Helper()
	dir, err := ioutil.TempDir("", name)
	if err != nil {
		d.Fatal(err)
	}
	d.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func (d *driverBase) gogio(args ...string) {
	d.Helper()
	prog, err := os.Executable()
	if err != nil {
		d.Fatal(err)
	}
	cmd := exec.Command(prog, args...)
	cmd.Env = append(os.Environ(), "RUN_GOGIO=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		d.Fatalf("gogio error: %s:\n%s", err, out)
	}
}
