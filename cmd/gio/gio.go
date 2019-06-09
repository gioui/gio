// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

var (
	target    = flag.String("target", "", "specify target (ios, tvos, android, js)")
	archNames = flag.String("arch", "", "specify architecture(s) to include")
	buildMode = flag.String("buildmode", "archive", "specify buildmode: archive or exe")
	destPath  = flag.String("o", "", "output file (Android .aar or .apk file) or directory (iOS/tvOS .framework or webassembly files)")
	appID     = flag.String("appid", "org.gioui.app", "app identifier (for -buildmode=exe)")
	verbose   = flag.Bool("v", false, "verbose output")
)

type buildInfo struct {
	pkg     string
	ldflags string
	archs   []string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Gio is a tool for building gio programs.\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n\n\tgio [flags] <pkg>\n\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	pkg := flag.Arg(0)
	if pkg == "" {
		flag.Usage()
	}
	if *target == "" {
		fmt.Fprintf(os.Stderr, "Please specify -target\n\n")
		flag.PrintDefaults()
		os.Exit(2)
	}
	switch *target {
	case "ios", "tvos", "android", "js":
	default:
		errorf("invalid -target %s\n", *target)
	}
	switch *buildMode {
	case "archive", "exe":
	default:
		errorf("invalid -buildmode %s\n", *buildMode)
	}
	// Expand relative package paths.
	pkg, err := runCmd(exec.Command("go", "list", pkg))
	if err != nil {
		errorf("gio: %v", err)
	}
	bi := &buildInfo{
		pkg: pkg,
	}
	switch *target {
	case "js":
		bi.archs = []string{"wasm"}
	case "ios", "tvos":
		// Only 64-bit support.
		bi.archs = []string{"arm64", "amd64"}
	case "android":
		bi.archs = []string{"arm", "arm64", "386", "amd64"}
	}
	if *archNames != "" {
		bi.archs = strings.Split(*archNames, ",")
	}
	if appArgs := flag.Args()[1:]; len(appArgs) > 0 {
		// Pass along arguments to the app.
		bi.ldflags = fmt.Sprintf("-X gioui.org/ui/app.extraArgs=%s", strings.Join(appArgs, "|"))
	}
	if err := build(bi); err != nil {
		errorf("gio: %v", err)
	}
}

func build(bi *buildInfo) error {
	tmpDir, err := ioutil.TempDir("", "gio-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	switch *target {
	case "js":
		return buildJS(bi)
	case "ios", "tvos":
		return buildIOS(tmpDir, *target, bi)
	case "android":
		return buildAndroid(tmpDir, bi)
	default:
		panic("unreachable")
	}
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

func runCmdRaw(cmd *exec.Cmd) ([]byte, error) {
	if *verbose {
		fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
	}
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if err, ok := err.(*exec.ExitError); ok {
		return nil, fmt.Errorf("%s failed: %s%s", strings.Join(cmd.Args, " "), out, err.Stderr)
	}
	return nil, err
}

func runCmd(cmd *exec.Cmd) (string, error) {
	out, err := runCmdRaw(cmd)
	return string(bytes.TrimSpace(out)), err
}

func copyFile(dst, src string) (err error) {
	r, err := os.Open(src)
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := w.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(w, r)
	return err
}

func appDir() (string, error) {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "gioui.org/ui/app")
	return runCmd(cmd)
}

type arch struct {
	iosArch string
	jniArch string
	clang   string
}

var allArchs = map[string]arch{
	"arm": arch{
		iosArch: "armv7",
		jniArch: "armeabi-v7a",
		clang:   "armv7a-linux-androideabi16-clang",
	},
	"arm64": arch{
		iosArch: "arm64",
		jniArch: "arm64-v8a",
		clang:   "aarch64-linux-android21-clang",
	},
	"386": arch{
		iosArch: "i386",
		jniArch: "x86",
		clang:   "i686-linux-android16-clang",
	},
	"amd64": arch{
		iosArch: "x86_64",
		jniArch: "x86_64",
		clang:   "x86_64-linux-android21-clang",
	},
}
