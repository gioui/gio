// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"archive/zip"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

func buildIOS(tmpDir, target string, bi *buildInfo) error {
	appName := bi.name
	switch *buildMode {
	case "archive":
		framework := *destPath
		if framework == "" {
			framework = fmt.Sprintf("%s.framework", strings.Title(appName))
		}
		return archiveIOS(tmpDir, target, framework, bi)
	case "exe":
		out := *destPath
		if out == "" {
			out = appName + ".ipa"
		}
		forDevice := strings.HasSuffix(out, ".ipa")
		// Filter out unsupported architectures.
		for i := len(bi.archs) - 1; i >= 0; i-- {
			switch bi.archs[i] {
			case "arm", "arm64":
				if forDevice {
					continue
				}
			case "386", "amd64":
				if !forDevice {
					continue
				}
			}

			bi.archs = append(bi.archs[:i], bi.archs[i+1:]...)
		}
		tmpFramework := filepath.Join(tmpDir, "Gio.framework")
		if err := archiveIOS(tmpDir, target, tmpFramework, bi); err != nil {
			return err
		}
		if !forDevice && !strings.HasSuffix(out, ".app") {
			return fmt.Errorf("the specified output directory %q does not end in .app or .ipa", out)
		}
		if !forDevice {
			return exeIOS(tmpDir, target, out, bi)
		}
		payload := filepath.Join(tmpDir, "Payload")
		appDir := filepath.Join(payload, appName+".app")
		if err := os.MkdirAll(appDir, 0755); err != nil {
			return err
		}
		if err := exeIOS(tmpDir, target, appDir, bi); err != nil {
			return err
		}
		if err := signIOS(tmpDir, appDir, out); err != nil {
			return err
		}
		return zipDir(out, tmpDir, "Payload")
	default:
		panic("unreachable")
	}
}

func signIOS(tmpDir, app, ipa string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	provPattern := filepath.Join(home, "Library", "MobileDevice", "Provisioning Profiles", "*.mobileprovision")
	provisions, err := filepath.Glob(provPattern)
	if err != nil {
		return err
	}
	provInfo := filepath.Join(tmpDir, "provision.plist")
	for _, prov := range provisions {
		// Decode the provision file to a plist.
		_, err := runCmd(exec.Command("security", "cms", "-D", "-i", prov, "-o", provInfo))
		if err != nil {
			return err
		}
		expUnix, err := runCmd(exec.Command("/usr/libexec/PlistBuddy", "-c", "Print:ExpirationDate", provInfo))
		if err != nil {
			return err
		}
		exp, err := time.Parse(time.UnixDate, expUnix)
		if err != nil {
			return fmt.Errorf("sign: failed to parse expiration date from %q: %v", prov, err)
		}
		if exp.Before(time.Now()) {
			continue
		}
		appIDPrefix, err := runCmd(exec.Command("/usr/libexec/PlistBuddy", "-c", "Print:ApplicationIdentifierPrefix:0", provInfo))
		if err != nil {
			return err
		}
		provAppID, err := runCmd(exec.Command("/usr/libexec/PlistBuddy", "-c", "Print:Entitlements:application-identifier", provInfo))
		if err != nil {
			return err
		}
		expAppID := fmt.Sprintf("%s.%s", appIDPrefix, *appID)
		if expAppID != provAppID {
			continue
		}
		// Copy provisioning file.
		embedded := filepath.Join(app, "embedded.mobileprovision")
		if err := copyFile(embedded, prov); err != nil {
			return err
		}
		certDER, err := runCmdRaw(exec.Command("/usr/libexec/PlistBuddy", "-c", "Print:DeveloperCertificates:0", provInfo))
		if err != nil {
			return err
		}
		// Omit trailing newline.
		certDER = certDER[:len(certDER)-1]
		cert, err := x509.ParseCertificate(certDER)
		if err != nil {
			return fmt.Errorf("sign: failed to parse developer certificate from %q: %v", prov, err)
		}
		entitlements, err := runCmd(exec.Command("/usr/libexec/PlistBuddy", "-x", "-c", "Print:Entitlements", provInfo))
		if err != nil {
			return err
		}
		entFile := filepath.Join(tmpDir, "entitlements.plist")
		if err := ioutil.WriteFile(entFile, []byte(entitlements), 0660); err != nil {
			return err
		}
		signIdentity := cert.Subject.CommonName
		_, err = runCmd(exec.Command("codesign", "-s", signIdentity, "--entitlements", entFile, app))
		return err
	}
	return fmt.Errorf("sign: no valid provisioning profile found for bundle id %q", *appID)
}

func exeIOS(tmpDir, target, app string, bi *buildInfo) error {
	if *appID == "" {
		return errors.New("app id is empty; use -appid to set it")
	}
	if err := os.RemoveAll(app); err != nil {
		return err
	}
	if err := os.Mkdir(app, 0755); err != nil {
		return err
	}
	mainm := filepath.Join(tmpDir, "main.m")
	const mainmSrc = `@import UIKit;
@import Gio;

int main(int argc, char * argv[]) {
	@autoreleasepool {
		return UIApplicationMain(argc, argv, nil, NSStringFromClass([GioAppDelegate class]));
	}
}`
	if err := ioutil.WriteFile(mainm, []byte(mainmSrc), 0660); err != nil {
		return err
	}
	exe := filepath.Join(app, "app")
	lipo := exec.Command("xcrun", "lipo", "-o", exe, "-create")
	var builds errgroup.Group
	for _, a := range bi.archs {
		clang, cflags, err := iosCompilerFor(target, a)
		if err != nil {
			return err
		}
		exeSlice := filepath.Join(tmpDir, "app-"+a)
		lipo.Args = append(lipo.Args, exeSlice)
		compile := exec.Command(clang, cflags...)
		compile.Args = append(compile.Args,
			"-F", tmpDir,
			"-o", exeSlice,
			mainm,
		)
		builds.Go(func() error {
			_, err := runCmd(compile)
			return err
		})
	}
	if err := builds.Wait(); err != nil {
		return err
	}
	if _, err := runCmd(lipo); err != nil {
		return err
	}
	infoPlist := buildInfoPlist(bi)
	plistFile := filepath.Join(app, "Info.plist")
	if err := ioutil.WriteFile(plistFile, []byte(infoPlist), 0660); err != nil {
		return err
	}
	if _, err := runCmd(exec.Command("plutil", "-convert", "binary1", plistFile)); err != nil {
		return err
	}
	return nil
}

func buildInfoPlist(bi *buildInfo) string {
	appName := strings.Title(bi.name)
	var platform string
	switch bi.target {
	case "ios":
		platform = "iphoneos"
	case "tvos":
		platform = "appletvos"
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleDevelopmentRegion</key>
	<string>en</string>
	<key>CFBundleExecutable</key>
	<string>app</string>
	<key>CFBundleIdentifier</key>
	<string>%s</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleName</key>
	<string>%s</string>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
	<key>CFBundleShortVersionString</key>
	<string>1.0</string>
	<key>CFBundleVersion</key>
	<string>1</string>
	<key>UILaunchStoryboardName</key>
	<string>LaunchScreen</string>
	<key>UIRequiredDeviceCapabilities</key>
	<array><string>arm64</string></array>
	<key>DTPlatformName</key>
	<string>%s</string>
	<key>DTPlatformVersion</key>
	<string>12.4</string>
	<key>MinimumOSVersion</key>
	<string>9.0</string>
	<key>UIDeviceFamily</key>
	<array>
		<integer>1</integer>
	</array>
</dict>
</plist>`, bi.appID, appName, platform)
}

func archiveIOS(tmpDir, target, frameworkRoot string, bi *buildInfo) error {
	framework := filepath.Base(frameworkRoot)
	const suf = ".framework"
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
		clang, cflags, err := iosCompilerFor(target, a)
		if err != nil {
			return err
		}
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
		cflagsLine := strings.Join(cflags, " ")
		cmd.Env = append(
			os.Environ(),
			"GOOS=darwin",
			"GOARCH="+a,
			"CGO_ENABLED=1",
			"CC="+clang,
			"CGO_CFLAGS="+cflagsLine,
			"CGO_LDFLAGS="+cflagsLine,
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

func iosCompilerFor(target, arch string) (string, []string, error) {
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
	switch arch {
	case "arm", "arm64":
		platformSDK += "os"
	case "386", "amd64":
		platformOS += "-simulator"
		platformSDK += "simulator"
	default:
		return "", nil, fmt.Errorf("unsupported -arch: %s", arch)
	}
	sdkPath, err := runCmd(exec.Command("xcrun", "--sdk", platformSDK, "--show-sdk-path"))
	if err != nil {
		return "", nil, err
	}
	clang, err := runCmd(exec.Command("xcrun", "--sdk", platformSDK, "--find", "clang"))
	if err != nil {
		return "", nil, err
	}
	cflags := []string{
		"-fmodules",
		"-fobjc-arc",
		"-fembed-bitcode",
		"-Werror",
		"-arch", allArchs[arch].iosArch,
		"-isysroot", sdkPath,
		"-m" + platformOS + "-version-min=9.0",
	}
	return clang, cflags, nil
}

func zipDir(dst, base, dir string) (err error) {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()
	zipf := zip.NewWriter(f)
	err = filepath.Walk(filepath.Join(base, dir), func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		rel := filepath.ToSlash(path[len(base)+1:])
		entry, err := zipf.Create(rel)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(entry, src)
		return err
	})
	if err != nil {
		return err
	}
	return zipf.Close()
}
