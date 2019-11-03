// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/template"

	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/packages"
)

type androidTools struct {
	buildtools string
	androidjar string
}

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

var exeSuffix string

type manifestData struct {
	AppID       string
	Version     int
	MinSDK      int
	TargetSDK   int
	Permissions []string
	Features    []string
	IconSnip    string
	AppName     string
}

func init() {
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
}

func buildAndroid(tmpDir string, bi *buildInfo) error {
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		return errors.New("please set ANDROID_HOME to the Android SDK path")
	}
	if _, err := os.Stat(sdk); err != nil {
		return err
	}
	platform, err := latestPlatform(sdk)
	if err != nil {
		return err
	}
	buildtools, err := latestTools(sdk)
	if err != nil {
		return err
	}

	tools := &androidTools{
		buildtools: buildtools,
		androidjar: filepath.Join(platform, "android.jar"),
	}

	perms := []string{"default"}
	const permPref = "gioui.org/app/permission/"
	cfg := &packages.Config{
		Mode: packages.NeedName +
			packages.NeedFiles +
			packages.NeedImports +
			packages.NeedDeps,
		Env: append(
			os.Environ(),
			"GOOS=android",
			"CGO_ENABLED=1",
		),
	}
	pkgs, err := packages.Load(cfg, bi.pkg)
	if err != nil {
		return err
	}
	var extraJars []string
	visitedPkgs := make(map[string]bool)
	var visitPkg func(*packages.Package) error
	visitPkg = func(p *packages.Package) error {
		if len(p.GoFiles) == 0 {
			return nil
		}
		dir := path.Dir(p.GoFiles[0])
		jars, err := filepath.Glob(filepath.Join(dir, "*.jar"))
		if err != nil {
			return err
		}
		extraJars = append(extraJars, jars...)
		switch {
		case p.PkgPath == "net":
			perms = append(perms, "network")
		case strings.HasPrefix(p.PkgPath, permPref):
			perms = append(perms, p.PkgPath[len(permPref):])
		}

		for _, imp := range p.Imports {
			if !visitedPkgs[imp.ID] {
				visitPkg(imp)
				visitedPkgs[imp.ID] = true
			}
		}
		return nil
	}
	if err := visitPkg(pkgs[0]); err != nil {
		return err
	}

	if err := compileAndroid(tmpDir, tools, bi); err != nil {
		return err
	}
	switch *buildMode {
	case "archive":
		return archiveAndroid(tmpDir, bi, perms)
	case "exe":
		if err := exeAndroid(tmpDir, tools, bi, extraJars, perms); err != nil {
			return err
		}
		return signAPK(tmpDir, tools, bi)
	default:
		panic("unreachable")
	}
}

func compileAndroid(tmpDir string, tools *androidTools, bi *buildInfo) (err error) {
	androidHome := os.Getenv("ANDROID_HOME")
	if androidHome == "" {
		return errors.New("ANDROID_HOME is not set. Please point it to the root of the Android SDK")
	}
	javac, err := findJavaC()
	if err != nil {
		return fmt.Errorf("could not find javac: %v", err)
	}
	ndkRoot := filepath.Join(androidHome, "ndk-bundle")
	if _, err := os.Stat(ndkRoot); err != nil {
		return fmt.Errorf("no NDK found in $ANDROID_HOME/ndk-bundle (%s). Use `sdkmanager ndk-bundle` to install it", ndkRoot)
	}
	tcRoot := filepath.Join(ndkRoot, "toolchains", "llvm", "prebuilt", archNDK())
	var builds errgroup.Group
	for _, a := range bi.archs {
		arch := allArchs[a]
		clang, err := latestCompiler(tcRoot, a, bi.minsdk)
		if err != nil {
			return fmt.Errorf("%s. Please make sure you have NDK >= r19c installed. Use the command `sdkmanager ndk-bundle` to install it.", err)
		}
		if runtime.GOOS == "windows" {
			// Because of https://github.com/android-ndk/ndk/issues/920,
			// we need NDK r19c, not just r19b. Check for the presence of
			// clang++.cmd which is only available in r19c.
			clangpp := clang + "++.cmd"
			if _, err := os.Stat(clangpp); err != nil {
				return fmt.Errorf("NDK version r19b detected, but >= r19c is required. Use the command `sdkmanager ndk-bundle` to install it")
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
			"-ldflags=-w -s "+bi.ldflags,
			"-buildmode=c-shared",
			"-o", libFile,
			bi.pkg,
		)
		cmd.Env = append(
			os.Environ(),
			"GOOS=android",
			"GOARCH="+a,
			"CGO_ENABLED=1",
			"CC="+clang,
		)
		builds.Go(func() error {
			_, err := runCmd(cmd)
			return err
		})
	}
	appDir, err := runCmd(exec.Command("go", "list", "-f", "{{.Dir}}", "gioui.org/app/internal/window"))
	if err != nil {
		return err
	}
	javaFiles, err := filepath.Glob(filepath.Join(appDir, "*.java"))
	if err != nil {
		return err
	}
	if len(javaFiles) > 0 {
		classes := filepath.Join(tmpDir, "classes")
		if err := os.MkdirAll(classes, 0755); err != nil {
			return err
		}
		javac := exec.Command(
			javac,
			"-target", "1.8",
			"-source", "1.8",
			"-sourcepath", appDir,
			"-bootclasspath", tools.androidjar,
			"-d", classes,
		)
		javac.Args = append(javac.Args, javaFiles...)
		builds.Go(func() error {
			_, err := runCmd(javac)
			return err
		})
	}
	return builds.Wait()
}

func archiveAndroid(tmpDir string, bi *buildInfo, perms []string) (err error) {
	aarFile := *destPath
	if aarFile == "" {
		aarFile = fmt.Sprintf("%s.aar", bi.name)
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
	permissions, features := getPermissions(perms)
	manifest := aarw.Create("AndroidManifest.xml")
	manifestSrc := manifestData{
		AppID:       bi.appID,
		MinSDK:      bi.minsdk,
		Permissions: permissions,
		Features:    features,
	}
	tmpl, err := template.New("manifest").Parse(
		`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="{{.AppID}}">
        <uses-sdk android:minSdkVersion="{{.MinSDK}}"/>
{{range .Permissions}}	<uses-permission android:name="{{.}}"/>
{{end}}{{range .Features}}	<uses-feature android:{{.}} android:required="false"/>
{{end}}</manifest>
`)
	if err != nil {
		panic(err)
	}
	err = tmpl.Execute(manifest, manifestSrc)
	proguard := aarw.Create("proguard.txt")
	proguard.Write([]byte(`-keep class org.gioui.** { *; }`))

	for _, a := range bi.archs {
		arch := allArchs[a]
		libFile := filepath.Join("jni", arch.jniArch, "libgio.so")
		aarw.Add(filepath.ToSlash(libFile), filepath.Join(tmpDir, libFile))
	}
	classes := filepath.Join(tmpDir, "classes")
	if _, err := os.Stat(classes); err == nil {
		jarFile := filepath.Join(tmpDir, "classes.jar")
		if err := writeJar(jarFile, classes); err != nil {
			return err
		}
		aarw.Add("classes.jar", jarFile)
	}
	return aarw.Close()
}

func exeAndroid(tmpDir string, tools *androidTools, bi *buildInfo, extraJars, perms []string) (err error) {
	classes := filepath.Join(tmpDir, "classes")
	var classFiles []string
	err = filepath.Walk(classes, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Ext(path) == ".class" {
			classFiles = append(classFiles, path)
		}
		return nil
	})
	classFiles = append(classFiles, extraJars...)
	apkDir := filepath.Join(tmpDir, "apk")
	if err := os.MkdirAll(apkDir, 0755); err != nil {
		return err
	}
	if len(classFiles) > 0 {
		d8 := exec.Command(
			filepath.Join(tools.buildtools, "d8"),
			"--classpath", tools.androidjar,
			"--output", apkDir,
		)
		d8.Args = append(d8.Args, classFiles...)
		if _, err := runCmd(d8); err != nil {
			return err
		}
	}

	// Compile resources.
	resDir := filepath.Join(tmpDir, "res")
	valDir := filepath.Join(resDir, "values")
	v21Dir := filepath.Join(resDir, "values-v21")
	for _, dir := range []string{valDir, v21Dir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	icon := filepath.Join(bi.dir, "appicon.png")
	iconSnip := ""
	if _, err := os.Stat(icon); err == nil {
		err := buildIcons(resDir, icon, []iconVariant{
			{path: filepath.Join("mipmap-hdpi", "ic_launcher.png"), size: 72},
			{path: filepath.Join("mipmap-xhdpi", "ic_launcher.png"), size: 96},
			{path: filepath.Join("mipmap-xxhdpi", "ic_launcher.png"), size: 144},
			{path: filepath.Join("mipmap-xxxhdpi", "ic_launcher.png"), size: 192},
		})
		if err != nil {
			return err
		}
		iconSnip = `android:icon="@mipmap/ic_launcher"`
	}
	themes := `<?xml version="1.0" encoding="utf-8"?>
<resources>
	<style name="Theme.GioApp" parent="android:style/Theme.NoTitleBar">
	</style>
</resources>`
	err = ioutil.WriteFile(filepath.Join(valDir, "themes.xml"), []byte(themes), 0660)
	if err != nil {
		return err
	}
	themesV21 := `<?xml version="1.0" encoding="utf-8"?>
<resources>
	<style name="Theme.GioApp" parent="android:style/Theme.NoTitleBar">
		<item name="android:windowDrawsSystemBarBackgrounds">true</item>
		<item name="android:navigationBarColor">#40000000</item>
		<item name="android:statusBarColor">#40000000</item>
	</style>
</resources>`
	err = ioutil.WriteFile(filepath.Join(v21Dir, "themes.xml"), []byte(themesV21), 0660)
	if err != nil {
		return err
	}
	resZip := filepath.Join(tmpDir, "resources.zip")
	aapt2 := filepath.Join(tools.buildtools, "aapt2")
	_, err = runCmd(exec.Command(
		aapt2,
		"compile",
		"-o", resZip,
		"--dir", resDir))
	if err != nil {
		return err
	}

	// Link APK.
	// Currently, new apps must have a target SDK version of at least 28.
	// https://developer.android.com/distribute/best-practices/develop/target-sdk
	targetSDK := 28
	if bi.minsdk > targetSDK {
		targetSDK = bi.minsdk
	}
	permissions, features := getPermissions(perms)
	appName := strings.Title(bi.name)
	manifestSrc := manifestData{
		AppID:       bi.appID,
		Version:     bi.version,
		MinSDK:      bi.minsdk,
		TargetSDK:   targetSDK,
		Permissions: permissions,
		Features:    features,
		IconSnip:    iconSnip,
		AppName:     appName,
	}
	tmpl, err := template.New("test").Parse(
		`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
	package="{{.AppID}}"
	android:versionCode="{{.Version}}"
	android:versionName="1.0.{{.Version}}">
	<uses-sdk android:minSdkVersion="{{.MinSDK}}" android:targetSdkVersion="{{.TargetSDK}}" />
{{range .Permissions}}	<uses-permission android:name="{{.}}"/>
{{end}}{{range .Features}}	<uses-feature android:{{.}} android:required="false"/>
{{end}}	<application {{.IconSnip}} android:label="{{.AppName}}">
		<activity android:name="org.gioui.GioActivity"
			android:label="{{.AppName}}"
			android:theme="@style/Theme.GioApp"
			android:configChanges="orientation|keyboardHidden"
			android:windowSoftInputMode="adjustResize">
			<intent-filter>
				<action android:name="android.intent.action.MAIN" />
				<category android:name="android.intent.category.LAUNCHER" />
			</intent-filter>
		</activity>
	</application>
</manifest>`)
	var manifestBuffer bytes.Buffer
	if err := tmpl.Execute(&manifestBuffer, manifestSrc); err != nil {
		return err
	}
	manifest := filepath.Join(tmpDir, "AndroidManifest.xml")
	if err := ioutil.WriteFile(manifest, manifestBuffer.Bytes(), 0660); err != nil {
		return err
	}

	tmpapk := filepath.Join(tmpDir, "link.apk")
	link := exec.Command(
		aapt2,
		"link",
		"--manifest", manifest,
		"-I", tools.androidjar,
		"-o", tmpapk,
		resZip,
	)
	if _, err := runCmd(link); err != nil {
		return err
	}
	// The Go standard library archive/zip doesn't support appending to zip
	// files. Unpack the apk from aapt2 and re-zip its contents along with
	// classes.dex and the Go libraries.
	if err := unzip(apkDir, tmpapk); err != nil {
		return err
	}
	tmpApk := filepath.Join(tmpDir, "app.ap_")
	ap_, err := os.Create(tmpApk)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := ap_.Close(); err == nil {
			err = cerr
		}
	}()
	apkw := newZipWriter(ap_)
	defer apkw.Close()
	err = filepath.Walk(apkDir, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		zpath := path[len(apkDir)+1:]
		if filepath.Base(path) == "resources.arsc" {
			apkw.Store(zpath, path)
		} else {
			apkw.Add(zpath, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, a := range bi.archs {
		arch := allArchs[a]
		libFile := filepath.Join(arch.jniArch, "libgio.so")
		apkw.Add(filepath.ToSlash(filepath.Join("lib", libFile)), filepath.Join(tmpDir, "jni", libFile))
	}
	return apkw.Close()
}

func signAPK(tmpDir string, tools *androidTools, bi *buildInfo) error {
	apkFile := *destPath
	if apkFile == "" {
		apkFile = fmt.Sprintf("%s.apk", bi.name)
	}
	if filepath.Ext(apkFile) != ".apk" {
		return fmt.Errorf("the specified output %q does not end in '.apk'", apkFile)
	}
	_, err := runCmd(exec.Command(
		filepath.Join(tools.buildtools, "zipalign"),
		"-f",
		"4", // 32-bit alignment.
		filepath.Join(tmpDir, "app.ap_"),
		apkFile,
	))
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	keystore := filepath.Join(home, ".android", "debug.keystore")
	if _, err := os.Stat(keystore); err != nil {
		keystore = filepath.Join(tmpDir, "sign.keystore")
		keytool, err := findKeytool()
		if err != nil {
			return err
		}
		_, err = runCmd(exec.Command(
			keytool,
			"-genkey",
			"-keystore", keystore,
			"-storepass", "android",
			"-alias", "android",
			"-keyalg", "RSA", "-keysize", "2048",
			"-validity", "10000",
			"-noprompt",
			"-dname", "CN=android",
		))
		if err != nil {
			return err
		}
	}
	_, err = runCmd(exec.Command(
		filepath.Join(tools.buildtools, "apksigner"),
		"sign",
		"--ks-pass", "pass:android",
		"--ks", keystore,
		apkFile,
	))
	if err != nil {
		return err
	}
	return nil
}

func unzip(dir, zipfile string) (err error) {
	zipr, err := zip.OpenReader(zipfile)
	if err != nil {
		return err
	}
	defer zipr.Close()
	for _, f := range zipr.File {
		path := filepath.Join(dir, f.Name)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		out, err := os.Create(path)
		if err != nil {
			return err
		}
		defer func() {
			if cerr := out.Close(); err == nil {
				err = cerr
			}
		}()
		in, err := f.Open()
		if err != nil {
			return err
		}
		defer in.Close()
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
	}
	return nil
}

func findKeytool() (string, error) {
	keytool, err := exec.LookPath("keytool")
	if err == nil {
		return keytool, err
	}
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome == "" {
		return "", err
	}
	keytool = filepath.Join(javaHome, "jre", "bin", "keytool"+exeSuffix)
	if _, serr := os.Stat(keytool); serr == nil {
		return keytool, nil
	}
	return "", err
}

func findJavaC() (string, error) {
	javac, err := exec.LookPath("javac")
	if err == nil {
		return javac, err
	}
	javaHome := os.Getenv("JAVA_HOME")
	if javaHome == "" {
		return "", err
	}
	javac = filepath.Join(javaHome, "bin", "javac"+exeSuffix)
	if _, serr := os.Stat(javac); serr == nil {
		return javac, nil
	}
	return "", err
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

func archNDK() string {
	if runtime.GOOS == "windows" {
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

func getPermissions(ps []string) ([]string, []string) {
	var permissions, features []string
	seenPermissions := make(map[string]bool)
	seenFeatures := make(map[string]bool)
	for _, perm := range ps {
		for _, x := range AndroidPermissions[perm] {
			if !seenPermissions[x] {
				permissions = append(permissions, x)
				seenPermissions[x] = true
			}
		}
		for _, x := range AndroidFeatures[perm] {
			if !seenFeatures[x] {
				features = append(features, x)
				seenFeatures[x] = true
			}
		}
	}
	return permissions, features
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

func latestCompiler(tcRoot, a string, minsdk int) (string, error) {
	arch := allArchs[a]
	allComps, err := filepath.Glob(filepath.Join(tcRoot, "bin", arch.clangArch+"*-clang"))
	if err != nil {
		return "", err
	}
	var bestVer int
	var firstVer int
	var bestCompiler string
	var firstCompiler string
	for _, compiler := range allComps {
		var ver int
		pattern := filepath.Join(tcRoot, "bin", arch.clangArch) + "%d-clang"
		if n, err := fmt.Sscanf(compiler, pattern, &ver); n < 1 || err != nil {
			continue
		}
		if firstCompiler == "" || ver < firstVer {
			firstVer = ver
			firstCompiler = compiler
		}
		if ver < bestVer {
			continue
		}
		if ver > minsdk {
			continue
		}
		bestVer = ver
		bestCompiler = compiler
	}
	if bestCompiler == "" {
		bestCompiler = firstCompiler
	}
	if bestCompiler == "" {
		return "", fmt.Errorf("no NDK compiler found for architecture %s", a)
	}
	return bestCompiler, nil
}

func latestTools(sdk string) (string, error) {
	allTools, err := filepath.Glob(filepath.Join(sdk, "build-tools", "*"))
	if err != nil {
		return "", err
	}
	var bestVer [3]int
	var bestTools string
loop:
	for _, tools := range allTools {
		_, name := filepath.Split(tools)
		s := strings.SplitN(name, ".", 3)
		if len(s) != len(bestVer) {
			continue
		}
		var version [3]int
		for i, v := range s {
			v, err := strconv.Atoi(v)
			if err != nil {
				continue loop
			}
			if v < bestVer[i] {
				continue loop
			}
			if v > bestVer[i] {
				break
			}
			version[i] = v
		}
		bestVer = version
		bestTools = tools
	}
	if bestTools == "" {
		return "", fmt.Errorf("no build-tools found in %q", sdk)
	}
	return bestTools, nil
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

func (z *zipWriter) Store(name, file string) {
	z.add(name, file, false)
}

func (z *zipWriter) Add(name, file string) {
	z.add(name, file, true)
}

func (z *zipWriter) add(name, file string, compressed bool) {
	if z.err != nil {
		return
	}
	f, err := os.Open(file)
	if err != nil {
		z.err = err
		return
	}
	defer f.Close()
	fh := &zip.FileHeader{
		Name: name,
	}
	if compressed {
		fh.Method = zip.Deflate
	}
	w, err := z.w.CreateHeader(fh)
	if err != nil {
		z.err = err
		return
	}
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
