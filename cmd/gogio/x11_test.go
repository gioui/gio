// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type X11TestDriver struct {
	driverBase

	display string
}

func (d *X11TestDriver) Start(path string) {
	// First, build the app.
	bin := filepath.Join(d.tempDir("gio-endtoend-x11"), "red")
	flags := []string{"build", "-tags", "nowayland", "-o=" + bin}
	if raceEnabled {
		flags = append(flags, "-race")
	}
	flags = append(flags, path)
	cmd := exec.Command("go", flags...)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.Fatalf("could not build app: %s:\n%s", err, out)
	}

	var wg sync.WaitGroup
	d.Cleanup(wg.Wait)

	d.startServer(&wg, d.width, d.height)

	// Then, start our program on the X server above.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, bin)
		cmd.Env = []string{"DISPLAY=" + d.display}
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
}

func (d *X11TestDriver) startServer(wg *sync.WaitGroup, width, height int) {
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

	d.needPrograms(
		xprog,     // to run the X server
		"scrot",   // to take screenshots
		"xdotool", // to send input
	)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, xprog, xflags...)
	combined := &bytes.Buffer{}
	cmd.Stdout = combined
	cmd.Stderr = combined
	if err := cmd.Start(); err != nil {
		d.Fatal(err)
	}
	d.Cleanup(cancel)
	d.Cleanup(func() {
		// Give it a chance to exit gracefully, cleaning up
		// after itself. After 10ms, the deferred cancel above
		// will signal an os.Kill.
		cmd.Process.Signal(os.Interrupt)
		time.Sleep(10 * time.Millisecond)
	})

	// Wait for the X server to be ready. The socket path isn't
	// terribly portable, but that's okay for now.
	withRetries(d.T, time.Second, func() error {
		socket := fmt.Sprintf("/tmp/.X11-unix/X%s", d.display[1:])
		_, err := os.Stat(socket)
		return err
	})

	wg.Add(1)
	go func() {
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			// Print all output and error.
			io.Copy(os.Stdout, combined)
			d.Error(err)
		}
		wg.Done()
	}()
}

func (d *X11TestDriver) Screenshot() image.Image {
	cmd := exec.Command("scrot", "--silent", "--overwrite", "/dev/stdout")
	cmd.Env = []string{"DISPLAY=" + d.display}
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.Errorf("%s", out)
		d.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		d.Fatal(err)
	}
	return img
}

func (d *X11TestDriver) xdotool(args ...interface{}) string {
	d.Helper()
	strs := make([]string, len(args))
	for i, arg := range args {
		strs[i] = fmt.Sprint(arg)
	}
	cmd := exec.Command("xdotool", strs...)
	cmd.Env = []string{"DISPLAY=" + d.display}
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.Errorf("%s", out)
		d.Fatal(err)
	}
	return string(bytes.TrimSpace(out))
}

func (d *X11TestDriver) Click(x, y int) {
	d.xdotool("mousemove", "--sync", x, y)
	d.xdotool("click", "1")

	// Wait for the gio app to render after this click.
	d.waitForFrame()
}
