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
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"
	"golang.org/x/sync/errgroup"
)

var (
	target        = flag.String("target", "", "specify target (ios, tvos, android, js).\n")
	archNames     = flag.String("arch", "", "specify architecture(s) to include (arm, arm64, amd64).")
	minsdk        = flag.Int("minsdk", 16, "specify minimum supported Android platform sdk version (e.g. 28 for android28 a.k.a. Android 9 Pie).")
	buildMode     = flag.String("buildmode", "exe", "specify buildmode (archive, exe)")
	destPath      = flag.String("o", "", "output file or directory.\nFor -target ios or tvos, use the .app suffix to target simulators.")
	appID         = flag.String("appid", "", "app identifier (for -buildmode=exe)")
	version       = flag.Int("version", 1, "app version (for -buildmode=exe)")
	printCommands = flag.Bool("x", false, "print the commands")
	keepWorkdir   = flag.Bool("work", false, "print the name of the temporary work directory and do not delete it when exiting.")
	linkMode      = flag.String("linkmode", "", "set the -linkmode flag of the go tool")
	extraLdflags  = flag.String("ldflags", "", "extra flags to the Go linker")
	extraTags     = flag.String("tags", "", "extra tags to the Go tool")
	iconPath      = flag.String("icon", "", "Specify an icon for iOS and Android")
)

func main() {
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, mainUsage)
	}
	flag.Parse()
	if err := flagValidate(); err != nil {
		fmt.Fprintf(os.Stderr, "gogio: %v\n", err)
		os.Exit(1)
	}
	buildInfo, err := newBuildInfo(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "gogio: %v\n", err)
		os.Exit(1)
	}
	if err := build(buildInfo); err != nil {
		fmt.Fprintf(os.Stderr, "gogio: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func flagValidate() error {
	pkgPathArg := flag.Arg(0)
	if pkgPathArg == "" {
		return errors.New("specify a package")
	}
	if *target == "" {
		return errors.New("please specify -target")
	}
	switch *target {
	case "ios", "tvos", "android", "js":
	default:
		return fmt.Errorf("invalid -target %s", *target)
	}
	switch *buildMode {
	case "archive", "exe":
	default:
		return fmt.Errorf("invalid -buildmode %s", *buildMode)
	}
	return nil
}

func build(bi *buildInfo) error {
	tmpDir, err := ioutil.TempDir("", "gogio-")
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
	iosArch   string
	jniArch   string
	clangArch string
}

var allArchs = map[string]arch{
	"arm": {
		iosArch:   "armv7",
		jniArch:   "armeabi-v7a",
		clangArch: "armv7a-linux-androideabi",
	},
	"arm64": {
		iosArch:   "arm64",
		jniArch:   "arm64-v8a",
		clangArch: "aarch64-linux-android",
	},
	"386": {
		iosArch:   "i386",
		jniArch:   "x86",
		clangArch: "i686-linux-android",
	},
	"amd64": {
		iosArch:   "x86_64",
		jniArch:   "x86_64",
		clangArch: "x86_64-linux-android",
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
