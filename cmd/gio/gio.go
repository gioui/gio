// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// zip.Writer with a sticky error.
type zipWriter struct {
	err error
	w   *zip.Writer
}

// Writer that saves any errors.
type errWriter struct {
	w   io.Writer
	err *error
}

var (
	target    = flag.String("target", "", "specify target (ios or android)")
	archNames = flag.String("arch", "", "specify architecture(s) to include")
	destPath  = flag.String("o", "", "output file (for Android .aar) or directory (for iOS .framework)")
	verbose   = flag.Bool("v", false, "verbose output")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <pkg>\n", os.Args[0])
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
	// Expand relative package paths.
	out, err := exec.Command("go", "list", pkg).Output()
	if err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			errorf("gio: %s", bytes.TrimSpace(err.Stderr))
		}
		errorf("gio: failed to run the go tool: %v", err)
	}
	pkg = string(bytes.TrimSpace(out))
	appArgs := flag.Args()[1:]
	if err := run(pkg, appArgs); err != nil {
		errorf("gio: %v", err)
	}
}

func run(pkg string, appArgs []string) error {
	tmpDir, err := ioutil.TempDir("", "gio-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	var ldflags string
	if len(appArgs) > 0 {
		// Pass along arguments to the app.
		ldflags = fmt.Sprintf("-X gioui.org/ui/app.extraArgs=%s", strings.Join(appArgs, "|"))
	}
	var archs []string
	switch *target {
	case "tvos":
		// Only 64-bit support.
		archs = []string{"arm64", "amd64"}
	default:
		archs = []string{"arm", "arm64", "386", "amd64"}
	}
	if *archNames != "" {
		archs = strings.Split(*archNames, ",")
	}
	switch *target {
	case "ios", "tvos":
		return runIOS(tmpDir, *target, pkg, archs, ldflags)
	case "android":
		return runAndroid(tmpDir, pkg, archs, ldflags)
	default:
		return fmt.Errorf("invalid -target %s\n", *target)
	}
}

func runIOS(tmpDir, target, pkg string, archs []string, ldflags string) error {
	frameworkRoot := *destPath
	if frameworkRoot == "" {
		appName := filepath.Base(pkg)
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
	for _, a := range archs {
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
			"-ldflags=-s -w "+ldflags,
			"-buildmode=c-archive",
			"-o", lib,
			"-tags", "ios",
			pkg,
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

func runAndroid(tmpDir, pkg string, archs []string, ldflags string) (err error) {
	androidHome := os.Getenv("ANDROID_HOME")
	if androidHome == "" {
		return errors.New("ANDROID_HOME is not set. Please point it to the root of the Android SDK.")
	}
	ndkRoot := filepath.Join(androidHome, "ndk-bundle")
	if _, err := os.Stat(ndkRoot); err != nil {
		return fmt.Errorf("No NDK found in $ANDROID_HOME/ndk-bundle (%s). Use `sdkmanager ndk-bundle` to install it.", ndkRoot)
	}
	tcRoot := filepath.Join(ndkRoot, "toolchains", "llvm", "prebuilt", archNDK())
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		return errors.New("Please set ANDROID_HOME to the Android SDK path")
	}
	if _, err := os.Stat(sdk); err != nil {
		return err
	}
	platform, err := latestPlatform(sdk)
	if err != nil {
		return err
	}
	var builds errgroup.Group
	for _, a := range archs {
		arch := allArchs[a]
		clang := filepath.Join(tcRoot, "bin", arch.clang)
		if _, err := os.Stat(clang); err != nil {
			return fmt.Errorf("No NDK compiler found. Please make sure you have NDK >= r19c installed. Use the command `sdkmanager ndk-bundle` to install it. Path %s", clang)
		}
		if runtime.GOOS == "windows" {
			// Because of https://github.com/android-ndk/ndk/issues/920,
			// we need NDK r19c, not just r19b. Check for the presence of
			// clang++.cmd which is only available in r19c.
			clangpp := filepath.Join(tcRoot, "bin", arch.clang+"++.cmd")
			if _, err := os.Stat(clangpp); err != nil {
				return fmt.Errorf("NDK version r19b detected, but >= r19c is required. Use the command `sdkmanager ndk-bundle` to install it.")
			}
		}
		archDir := filepath.Join(tmpDir, "jni", arch.jniArch)
		if err := os.MkdirAll(archDir, 0755); err != nil {
			return fmt.Errorf("failed to create %q: %v", archDir, err)
		}
		libFile := filepath.Join(archDir, "libgio.so")
		cmd := exec.Command(
			"go",
			"build",
			"-ldflags=-w -s "+ldflags,
			"-buildmode=c-shared",
			"-o", libFile,
			pkg,
		)
		cmd.Env = append(
			os.Environ(),
			"GOOS=android",
			"GOARCH="+a,
			"CGO_ENABLED=1",
			"CC="+clang,
			"CGO_CFLAGS=-Werror",
		)
		builds.Go(func() error {
			_, err := runCmd(cmd)
			return err
		})
	}
	if err := builds.Wait(); err != nil {
		return err
	}
	aarFile := *destPath
	if aarFile == "" {
		aarFile = fmt.Sprintf("%s.aar", filepath.Base(pkg))
	}
	if filepath.Ext(aarFile) != ".aar" {
		return fmt.Errorf("the specified output %q does not end in '.aar'", aarFile)
	}
	aar, err := os.Create(aarFile)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := aar.Close(); err == nil {
			err = cerr
		}
	}()
	aarw := newZipWriter(aar)
	defer aarw.Close()
	aarw.Create("R.txt")
	aarw.Create("res/")
	manifest := aarw.Create("AndroidManifest.xml")
	manifest.Write([]byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="org.gioui.app">
	<uses-sdk android:minSdkVersion="16"/>
	<uses-feature android:glEsVersion="0x00030000" android:required="true" />
</manifest>`))
	proguard := aarw.Create("proguard.txt")
	proguard.Write([]byte(`-keep class org.gioui.** { *; }`))

	for _, a := range archs {
		arch := allArchs[a]
		libFile := filepath.Join("jni", arch.jniArch, "libgio.so")
		aarw.Add(filepath.ToSlash(libFile), filepath.Join(tmpDir, libFile))
	}
	appDir, err := appDir()
	if err != nil {
		return err
	}
	javaFiles, err := filepath.Glob(filepath.Join(appDir, "*.java"))
	if err != nil {
		return err
	}
	if len(javaFiles) > 0 {
		clsPath := filepath.Join(platform, "android.jar")
		classes := filepath.Join(tmpDir, "classes")
		if err := os.MkdirAll(classes, 0755); err != nil {
			return err
		}
		javac := exec.Command(
			"javac",
			"-target", "1.8",
			"-source", "1.8",
			"-sourcepath", appDir,
			"-bootclasspath", clsPath,
			"-d", classes,
		)
		javac.Args = append(javac.Args, javaFiles...)
		if _, err := runCmd(javac); err != nil {
			return err
		}
		jarFile := filepath.Join(tmpDir, "classes.jar")
		if err := writeJar(jarFile, classes); err != nil {
			return err
		}
		aarw.Add("classes.jar", jarFile)
	}
	return aarw.Close()
}

func newZipWriter(w io.Writer) *zipWriter {
	return &zipWriter{
		w: zip.NewWriter(w),
	}
}

func (z *zipWriter) Close() error {
	err := z.w.Close()
	if z.err == nil {
		z.err = err
	}
	return z.err
}

func (z *zipWriter) Create(name string) io.Writer {
	if z.err != nil {
		return ioutil.Discard
	}
	w, err := z.w.Create(name)
	if err != nil {
		z.err = err
		return ioutil.Discard
	}
	return &errWriter{w: w, err: &z.err}
}

func (z *zipWriter) Add(name, file string) {
	if z.err != nil {
		return
	}
	w := z.Create(name)
	f, err := os.Open(file)
	if err != nil {
		z.err = err
		return
	}
	defer f.Close()
	if _, err := io.Copy(w, f); err != nil {
		z.err = err
		return
	}
}

func (w *errWriter) Write(p []byte) (n int, err error) {
	if err := *w.err; err != nil {
		return 0, err
	}
	n, err = w.w.Write(p)
	*w.err = err
	return
}

func errorf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(2)
}

func runCmd(cmd *exec.Cmd) ([]byte, error) {
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
	cmd.Env = append(
		os.Environ(),
		"GOOS=android",
	)
	out, err := runCmd(cmd)
	if err != nil {
		return "", err
	}
	appDir := string(bytes.TrimSpace(out))
	return appDir, nil
}

func writeJar(jarFile, dir string) (err error) {
	jar, err := os.Create(jarFile)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := jar.Close(); err == nil {
			err = cerr
		}
	}()
	jarw := newZipWriter(jar)
	const manifestHeader = `Manifest-Version: 1.0
Created-By: 1.0 (Go)

`
	jarw.Create("META-INF/MANIFEST.MF").Write([]byte(manifestHeader))
	err = filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".class" {
			rel := filepath.ToSlash(path[len(dir)+1:])
			jarw.Add(rel, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return jarw.Close()
}

type arch struct {
	iosArch string
	jniArch string
	clang   string
	// TODO: Remove when https://github.com/android-ndk/ndk/issues/920
	// is solved and released.
	r19bWindowsClangArgs string
}

var allArchs = map[string]arch{
	"arm": arch{
		iosArch:              "armv7",
		jniArch:              "armeabi-v7a",
		clang:                "armv7a-linux-androideabi16-clang",
		r19bWindowsClangArgs: "--target=armv7a-linux-androideabi16 -fno-addrsig",
	},
	"arm64": arch{
		iosArch:              "arm64",
		jniArch:              "arm64-v8a",
		clang:                "aarch64-linux-android21-clang",
		r19bWindowsClangArgs: "--target=aarch64-linux-androideabi21 -fno-addrsig",
	},
	"386": arch{
		iosArch:              "i386",
		jniArch:              "x86",
		clang:                "i686-linux-android16-clang",
		r19bWindowsClangArgs: "--target=i686-linux-androideabi16 -fno-addrsig",
	},
	"amd64": arch{
		iosArch:              "x86_64",
		jniArch:              "x86_64",
		clang:                "x86_64-linux-android21-clang",
		r19bWindowsClangArgs: "--target=x86_64-linux-androideabi21 -fno-addrsig",
	},
}

func archNDK() string {
	if runtime.GOOS == "windows" && runtime.GOARCH == "386" {
		return "windows"
	} else {
		var arch string
		switch runtime.GOARCH {
		case "386":
			arch = "x86"
		case "amd64":
			arch = "x86_64"
		default:
			panic("unsupported GOARCH: " + runtime.GOARCH)
		}
		return runtime.GOOS + "-" + arch
	}
}

func latestPlatform(sdk string) (string, error) {
	allPlats, err := filepath.Glob(filepath.Join(sdk, "platforms", "android-*"))
	if err != nil {
		return "", err
	}
	var bestVer int
	var bestPlat string
	for _, platform := range allPlats {
		_, name := filepath.Split(platform)
		// The glob above guarantees the "android-" prefix.
		verStr := name[len("android-"):]
		ver, err := strconv.Atoi(verStr)
		if err != nil {
			continue
		}
		if ver < bestVer {
			continue
		}
		bestVer = ver
		bestPlat = platform
	}
	if bestPlat == "" {
		return "", fmt.Errorf("no platforms found in %q", sdk)
	}
	return bestPlat, nil
}
