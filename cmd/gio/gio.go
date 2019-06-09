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
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
)

var (
	target    = flag.String("target", "", "specify target (ios, tvos, android)")
	archNames = flag.String("arch", "", "specify architecture(s) to include")
	buildMode = flag.String("buildmode", "archive", "specify buildmode: archive or exe")
	destPath  = flag.String("o", "", "output file (Android .aar or .apk file) or directory (iOS/tvOS .framework)")
	verbose   = flag.Bool("v", false, "verbose output")
)

type buildInfo struct {
	pkg     string
	ldflags string
	archs   []string
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Gio is a tool for building and running gio programs.\n\n")
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
	case "ios", "tvos", "android":
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
	case "ios", "tvos":
		return archiveIOS(tmpDir, *target, bi)
	case "android":
		return buildAndroid(tmpDir, bi)
	default:
		panic("unreachable")
	}
}

func archiveIOS(tmpDir, target string, bi *buildInfo) error {
	frameworkRoot := *destPath
	if frameworkRoot == "" {
		appName := filepath.Base(bi.pkg)
		frameworkRoot = fmt.Sprintf("%s.framework", strings.Title(appName))
	}
	framework := filepath.Base(frameworkRoot)
	suf := ".framework"
	if !strings.HasSuffix(framework, suf) {
		return fmt.Errorf("the specified output %q does not end in '.framework'", frameworkRoot)
	}
	framework = framework[:len(framework)-len(suf)]
	if err := os.RemoveAll(frameworkRoot); err != nil {
		return err
	}
	frameworkDir := filepath.Join(frameworkRoot, "Versions", "A")
	for _, dir := range []string{"Headers", "Modules"} {
		p := filepath.Join(frameworkDir, dir)
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
	}
	symlinks := [][2]string{
		{"Versions/Current/Headers", "Headers"},
		{"Versions/Current/Modules", "Modules"},
		{"Versions/Current/" + framework, framework},
		{"A", filepath.Join("Versions", "Current")},
	}
	for _, l := range symlinks {
		if err := os.Symlink(l[0], filepath.Join(frameworkRoot, l[1])); err != nil && !os.IsExist(err) {
			return err
		}
	}
	exe := filepath.Join(frameworkDir, framework)
	lipo := exec.Command("xcrun", "lipo", "-o", exe, "-create")
	var builds errgroup.Group
	for _, a := range bi.archs {
		arch := allArchs[a]
		var platformSDK string
		var platformOS string
		switch target {
		case "ios":
			platformOS = "ios"
			platformSDK = "iphone"
		case "tvos":
			platformOS = "tvos"
			platformSDK = "appletv"
		}
		switch a {
		case "arm", "arm64":
			platformSDK += "os"
		case "386", "amd64":
			platformOS += "-simulator"
			platformSDK += "simulator"
		default:
			return fmt.Errorf("unsupported -arch: %s", a)
		}
		sdkPathOut, err := runCmd(exec.Command("xcrun", "--sdk", platformSDK, "--show-sdk-path"))
		if err != nil {
			return err
		}
		sdkPath := string(bytes.TrimSpace(sdkPathOut))
		clangOut, err := runCmd(exec.Command("xcrun", "--sdk", platformSDK, "--find", "clang"))
		if err != nil {
			return err
		}
		clang := string(bytes.TrimSpace(clangOut))
		cflags := fmt.Sprintf("-fmodules -fobjc-arc -fembed-bitcode -Werror -arch %s -isysroot %s -m%s-version-min=9.0", arch.iosArch, sdkPath, platformOS)
		lib := filepath.Join(tmpDir, "gio-"+a)
		cmd := exec.Command(
			"go",
			"build",
			"-ldflags=-s -w "+bi.ldflags,
			"-buildmode=c-archive",
			"-o", lib,
			"-tags", "ios",
			bi.pkg,
		)
		lipo.Args = append(lipo.Args, lib)
		cmd.Env = append(
			os.Environ(),
			"GOOS=darwin",
			"GOARCH="+a,
			"CGO_ENABLED=1",
			"CC="+clang,
			"CGO_CFLAGS="+cflags,
			"CGO_LDFLAGS="+cflags,
		)
		builds.Go(func() error {
			_, err := runCmd(cmd)
			return err
		})
	}
	if err := builds.Wait(); err != nil {
		return err
	}
	if _, err := runCmd(lipo); err != nil {
		return err
	}
	appDir, err := appDir()
	if err != nil {
		return err
	}
	headerDst := filepath.Join(frameworkDir, "Headers", framework+".h")
	headerSrc := filepath.Join(appDir, "framework_ios.h")
	if err := copyFile(headerDst, headerSrc); err != nil {
		return err
	}
	module := fmt.Sprintf(`framework module "%s" {
    header "%[1]s.h"

    export *
}`, framework)
	moduleFile := filepath.Join(frameworkDir, "Modules", "module.modulemap")
	return ioutil.WriteFile(moduleFile, []byte(module), 0644)
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

func runCmd(cmd *exec.Cmd) (string, error) {
	if *verbose {
		fmt.Printf("%s\n", strings.Join(cmd.Args, " "))
	}
	out, err := cmd.Output()
	if err == nil {
		return string(bytes.TrimSpace(out)), nil
	}
	if err, ok := err.(*exec.ExitError); ok {
		return "", fmt.Errorf("%s failed: %s%s", strings.Join(cmd.Args, " "), out, err.Stderr)
	}
	return "", err
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
