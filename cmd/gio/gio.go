// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/sync/errgroup"
)

var (
	target        = flag.String("target", "", "specify target (ios, tvos, android, js).\n")
	archNames     = flag.String("arch", "", "specify architecture(s) to include (arm, arm64, amd64).")
	buildMode     = flag.String("buildmode", "exe", "specify buildmode (archive, exe)")
	destPath      = flag.String("o", "", "output file or directory.\nFor -target ios or tvos, use the .app suffix to target simulators.")
	appID         = flag.String("appid", "org.gioui.app", "app identifier (for -buildmode=exe)")
	version       = flag.Int("version", 1, "app version (for -buildmode=exe)")
	printCommands = flag.Bool("x", false, "print the commands")
	keepWorkdir   = flag.Bool("work", false, "print the name of the temporary work directory and do not delete it when exiting.")
)

type buildInfo struct {
	name    string
	pkg     string
	ldflags string
	target  string
	appID   string
	version int
	dir     string
	archs   []string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, mainUsage)
	}
	flag.Parse()
	if err := mainErr(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func mainErr() error {
	pkg := flag.Arg(0)
	if pkg == "" {
		return errors.New("specify a package")
	}
	if *target == "" {
		return errors.New("please specify -target")
	}
	switch *target {
	case "ios", "tvos", "android", "js":
	default:
		return fmt.Errorf("invalid -target %s\n", *target)
	}
	switch *buildMode {
	case "archive", "exe":
	default:
		return fmt.Errorf("invalid -buildmode %s\n", *buildMode)
	}
	// Find package name.
	name, err := runCmd(exec.Command("go", "list", "-f", "{{.ImportPath}}", pkg))
	if err != nil {
		return fmt.Errorf("gio: %v", err)
	}
	name = path.Base(name)
	dir, err := runCmd(exec.Command("go", "list", "-f", "{{.Dir}}", pkg))
	if err != nil {
		return fmt.Errorf("gio: %v", err)
	}
	bi := &buildInfo{
		name:    name,
		pkg:     pkg,
		target:  *target,
		appID:   *appID,
		dir:     dir,
		version: *version,
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
		return fmt.Errorf("gio: %v", err)
	}
	return nil
}

func build(bi *buildInfo) error {
	tmpDir, err := ioutil.TempDir("", "gio-")
	if err != nil {
		return err
	}
	if *keepWorkdir {
		fmt.Fprintf(os.Stderr, "WORKDIR=%s\n", tmpDir)
	} else {
		defer os.RemoveAll(tmpDir)
	}
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

func runCmdRaw(cmd *exec.Cmd) ([]byte, error) {
	if *printCommands {
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

type iconVariant struct {
	path string
	size int
	fill bool
}

func buildIcons(baseDir, icon string, variants []iconVariant) error {
	f, err := os.Open(icon)
	if err != nil {
		return err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	var resizes errgroup.Group
	for _, v := range variants {
		v := v
		resizes.Go(func() (err error) {
			scaled := image.NewNRGBA(image.Rectangle{Max: image.Point{X: v.size, Y: v.size}})
			op := draw.Src
			if v.fill {
				op = draw.Over
				draw.Draw(scaled, scaled.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
			}
			draw.CatmullRom.Scale(scaled, scaled.Bounds(), img, img.Bounds(), op, nil)
			path := filepath.Join(baseDir, v.path)
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return err
			}
			f, err := os.Create(path)
			if err != nil {
				return err
			}
			defer func() {
				if cerr := f.Close(); err == nil {
					err = cerr
				}
			}()
			return png.Encode(f, scaled)
		})
	}
	return resizes.Wait()
}
