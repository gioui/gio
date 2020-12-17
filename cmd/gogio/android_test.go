// SPDX-License-Identifier: Unlicense OR MIT

package main_test

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

type AndroidTestDriver struct {
	driverBase

	sdkDir  string
	adbPath string
}

var rxAdbDevice = regexp.MustCompile(`(.*)\s+device$`)

func (d *AndroidTestDriver) Start(path string) {
	d.sdkDir = os.Getenv("ANDROID_SDK_ROOT")
	if d.sdkDir == "" {
		d.Skipf("Android SDK is required; set $ANDROID_SDK_ROOT")
	}
	d.adbPath = filepath.Join(d.sdkDir, "platform-tools", "adb")
	if _, err := os.Stat(d.adbPath); os.IsNotExist(err) {
		d.Skipf("adb not found")
	}

	devOut := bytes.TrimSpace(d.adb("devices"))
	devices := rxAdbDevice.FindAllSubmatch(devOut, -1)
	switch len(devices) {
	case 0:
		d.Skipf("no Android devices attached via adb; skipping")
	case 1:
	default:
		d.Skipf("multiple Android devices attached via adb; skipping")
	}

	// If the device is attached but asleep, it's probably just charging.
	// Don't use it; the screen needs to be on and unlocked for the test to
	// work.
	if !bytes.Contains(
		d.adb("shell", "dumpsys", "power"),
		[]byte(" mWakefulness=Awake"),
	) {
		d.Skipf("Android device isn't awake; skipping")
	}

	// First, build the app.
	apk := filepath.Join(d.tempDir("gio-endtoend-android"), "e2e.apk")
	d.gogio("-target=android", "-appid="+appid, "-o="+apk, path)

	// Make sure the app isn't installed already, and try to uninstall it
	// when we finish. Previous failed test runs might have left the app.
	d.tryUninstall()
	d.adb("install", apk)
	d.Cleanup(d.tryUninstall)

	// Force our e2e app to be fullscreen, so that the android system bar at
	// the top doesn't mess with our screenshots.
	// TODO(mvdan): is there a way to do this via gio, so that we don't need
	// to set up a global Android setting via the shell?
	d.adb("shell", "settings", "put", "global", "policy_control", "immersive.full="+appid)

	// Make sure the app isn't already running.
	d.adb("shell", "pm", "clear", appid)

	// Start listening for log messages.
	{
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, d.adbPath,
			"logcat",
			"-s",       // suppress other logs
			"-T1",      // don't show previous log messages
			appid+":*", // show all logs from our gio app ID
		)
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
	}

	// Start the app.
	d.adb("shell", "monkey", "-p", appid, "1")

	// Wait for the gio app to render.
	d.waitForFrame()
}

func (d *AndroidTestDriver) Screenshot() image.Image {
	out := d.adb("shell", "screencap", "-p")
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		d.Fatal(err)
	}
	return img
}

func (d *AndroidTestDriver) tryUninstall() {
	cmd := exec.Command(d.adbPath, "shell", "pm", "uninstall", appid)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if bytes.Contains(out, []byte("Unknown package")) {
			// The package is not installed. Don't log anything.
			return
		}
		d.Logf("could not uninstall: %v\n%s", err, out)
	}
}

func (d *AndroidTestDriver) adb(args ...interface{}) []byte {
	strs := []string{}
	for _, arg := range args {
		strs = append(strs, fmt.Sprint(arg))
	}
	cmd := exec.Command(d.adbPath, strs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		d.Errorf("%s", out)
		d.Fatal(err)
	}
	return out
}

func (d *AndroidTestDriver) Click(x, y int) {
	d.adb("shell", "input", "tap", x, y)

	// Wait for the gio app to render after this click.
	d.waitForFrame()
}
