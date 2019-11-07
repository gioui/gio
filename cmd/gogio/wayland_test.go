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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"text/template"
	"time"
)

type WaylandTestDriver struct {
	t *testing.T

	frameNotifs chan bool

	runtimeDir string
	socket     string
	display    string
}

// No bars or anything fancy. Just a white background with our dimensions.
var tmplSwayConfig = template.Must(template.New("").Parse(`
output * bg #FFFFFF solid_color
output * mode {{.Width}}x{{.Height}}
default_border none
`))

var rxSwayReady = regexp.MustCompile(`Running compositor on wayland display '(.*)'`)

func (d *WaylandTestDriver) Start(t_ *testing.T, path string, width, height int) {
	d.frameNotifs = make(chan bool, 1)
	d.t = t_

	// We want os.Environ, so that it can e.g. find $DISPLAY to run within
	// X11. wlroots env vars are documented at:
	// https://github.com/swaywm/wlroots/blob/master/docs/env_vars.md
	env := os.Environ()
	if *headless {
		env = append(env, "WLR_BACKENDS=headless")
	}

	for _, prog := range []string{
		"sway",    // to run a wayland compositor
		"grim",    // to take screenshots
		"swaymsg", // to send input
	} {
		if _, err := exec.LookPath(prog); err != nil {
			d.t.Skipf("%s needed to run", prog)
		}
	}

	// First, build the app.
	dir, err := ioutil.TempDir("", "gio-endtoend-wayland")
	if err != nil {
		d.t.Fatal(err)
	}
	d.t.Cleanup(func() { os.RemoveAll(dir) })

	bin := filepath.Join(dir, "red")
	flags := []string{"build", "-tags", "nox11", "-o=" + bin}
	if raceEnabled {
		flags = append(flags, "-race")
	}
	flags = append(flags, path)
	cmd := exec.Command("go", flags...)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.t.Fatalf("could not build app: %s:\n%s", err, out)
	}

	conf := filepath.Join(dir, "config")
	f, err := os.Create(conf)
	if err != nil {
		d.t.Fatal(err)
	}
	defer f.Close()
	if err := tmplSwayConfig.Execute(f, struct{ Width, Height int }{
		width, height,
	}); err != nil {
		d.t.Fatal(err)
	}

	d.socket = filepath.Join(dir, "socket")
	env = append(env, "SWAYSOCK="+d.socket)
	d.runtimeDir = dir
	env = append(env, "XDG_RUNTIME_DIR="+d.runtimeDir)

	var wg sync.WaitGroup
	d.t.Cleanup(wg.Wait)

	// First, start sway.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, "sway", "--config", conf, "--verbose")
		cmd.Env = env
		stderr, err := cmd.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}
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

		// Wait for sway to be ready. We probably don't need a deadline
		// here.
		br := bufio.NewReader(stderr)
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				d.t.Fatal(err)
			}
			if m := rxSwayReady.FindStringSubmatch(line); m != nil {
				d.display = m[1]
				break
			}
		}

		wg.Add(1)
		go func() {
			if err := cmd.Wait(); err != nil && ctx.Err() == nil && !strings.Contains(err.Error(), "interrupt") {
				// Don't print all stderr, since we use --verbose.
				// TODO(mvdan): if it's useful, probably filter
				// errors and show them.
				d.t.Error(err)
			}
			wg.Done()
		}()
	}

	// Then, start our program on the sway compositor above.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, bin)
		cmd.Env = []string{"XDG_RUNTIME_DIR=" + d.runtimeDir, "WAYLAND_DISPLAY=" + d.display}
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

func (d *WaylandTestDriver) Screenshot() image.Image {
	cmd := exec.Command("grim", "/dev/stdout")
	cmd.Env = []string{"XDG_RUNTIME_DIR=" + d.runtimeDir, "WAYLAND_DISPLAY=" + d.display}
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

func (d *WaylandTestDriver) swaymsg(args ...interface{}) {
	strs := []string{
		"--socket", d.socket,
	}
	for _, arg := range args {
		strs = append(strs, fmt.Sprint(arg))
	}
	cmd := exec.Command("swaymsg", strs...)
	if out, err := cmd.CombinedOutput(); err != nil {
		d.t.Errorf("%s", out)
		d.t.Fatal(err)
	}
}

func (d *WaylandTestDriver) Click(x, y int) {
	d.swaymsg("-t", "get_seats")
	d.swaymsg("seat", "-", "cursor", "set", x, y)
	d.swaymsg("seat", "-", "cursor", "press", "button1")
	d.swaymsg("seat", "-", "cursor", "release", "button1")

	// Wait for the gio app to render after this click.
	<-d.frameNotifs
}
