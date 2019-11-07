// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bufio"
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

	frameNotifs chan bool

	display string
}

func (d *X11TestDriver) Start(t_ *testing.T, path string, width, height int) {
	d.frameNotifs = make(chan bool, 1)
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
	d.t.Cleanup(func() { os.RemoveAll(dir) })

	bin := filepath.Join(dir, "red")
	flags := []string{"build", "-tags", "nowayland", "-o=" + bin}
	if raceEnabled {
		flags = append(flags, "-race")
	}
	flags = append(flags, path)
	cmd := exec.Command("go", flags...)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.t.Fatalf("could not build app: %s:\n%s", err, out)
	}

	var wg sync.WaitGroup
	d.t.Cleanup(wg.Wait)

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
		d.t.Cleanup(cancel)
		d.t.Cleanup(func() {
			// Give it a chance to exit gracefully, cleaning up
			// after itself. After 10ms, the deferred cancel above
			// will signal an os.Kill.
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
		cmd.Env = []string{"DISPLAY=" + d.display}
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			d.t.Fatal(err)
		}
		stderr := &bytes.Buffer{}
		cmd.Stderr = stderr

		if err := cmd.Start(); err != nil {
			d.t.Fatal(err)
		}
		d.t.Cleanup(cancel)
		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				// Print stderr and error.
				io.Copy(os.Stdout, stderr)
				d.t.Error(err)
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
	<-d.frameNotifs
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

	// Wait for the gio app to render after this click.
	<-d.frameNotifs
}
