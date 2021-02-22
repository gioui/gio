package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

type buildInfo struct {
	appID    string
	archs    []string
	ldflags  string
	minsdk   int
	name     string
	pkgDir   string
	pkgPath  string
	iconPath string
	tags     string
	target   string
	version  int
	key      string
	password string
}

func newBuildInfo(pkgPath string) (*buildInfo, error) {
	pkgMetadata, err := getPkgMetadata(pkgPath)
	if err != nil {
		return nil, err
	}
	appID := getAppID(pkgMetadata)
	appIcon := filepath.Join(pkgMetadata.Dir, "appicon.png")
	if *iconPath != "" {
		appIcon = *iconPath
	}
	bi := &buildInfo{
		appID:    appID,
		archs:    getArchs(),
		ldflags:  getLdFlags(appID),
		minsdk:   *minsdk,
		name:     getPkgName(pkgMetadata),
		pkgDir:   pkgMetadata.Dir,
		pkgPath:  pkgPath,
		iconPath: appIcon,
		tags:     *extraTags,
		target:   *target,
		version:  *version,
		key:      *signKey,
		password: *signPass,
	}
	return bi, nil
}

func getArchs() []string {
	if *archNames != "" {
		return strings.Split(*archNames, ",")
	}
	switch *target {
	case "js":
		return []string{"wasm"}
	case "ios", "tvos":
		// Only 64-bit support.
		return []string{"arm64", "amd64"}
	case "android":
		return []string{"arm", "arm64", "386", "amd64"}
	case "windows":
		goarch := os.Getenv("GOARCH")
		if goarch == "" {
			goarch = runtime.GOARCH
		}
		return []string{goarch}
	default:
		// TODO: Add flag tests.
		panic("The target value has already been validated, this will never execute.")
	}
}

func getLdFlags(appID string) string {
	var ldflags []string
	if extra := *extraLdflags; extra != "" {
		ldflags = append(ldflags, strings.Split(extra, " ")...)
	}
	// Pass appID along, to be used for logging on platforms like Android.
	ldflags = append(ldflags, fmt.Sprintf("-X gioui.org/app/internal/log.appID=%s", appID))
	// Pass along all remaining arguments to the app.
	if appArgs := flag.Args()[1:]; len(appArgs) > 0 {
		ldflags = append(ldflags, fmt.Sprintf("-X gioui.org/app.extraArgs=%s", strings.Join(appArgs, "|")))
	}
	if m := *linkMode; m != "" {
		ldflags = append(ldflags, "-linkmode="+m)
	}
	return strings.Join(ldflags, " ")
}

type packageMetadata struct {
	PkgPath string
	Dir     string
}

func getPkgMetadata(pkgPath string) (*packageMetadata, error) {
	pkgImportPath, err := runCmd(exec.Command("go", "list", "-f", "{{.ImportPath}}", pkgPath))
	if err != nil {
		return nil, err
	}
	pkgDir, err := runCmd(exec.Command("go", "list", "-f", "{{.Dir}}", pkgPath))
	if err != nil {
		return nil, err
	}
	return &packageMetadata{
		PkgPath: pkgImportPath,
		Dir:     pkgDir,
	}, nil
}

func getAppID(pkgMetadata *packageMetadata) string {
	if *appID != "" {
		return *appID
	}
	elems := strings.Split(pkgMetadata.PkgPath, "/")
	domain := strings.Split(elems[0], ".")
	name := ""
	if len(elems) > 1 {
		name = "." + elems[len(elems)-1]
	}
	if len(elems) < 2 && len(domain) < 2 {
		name = "." + domain[0]
		domain[0] = "localhost"
	} else {
		for i := 0; i < len(domain)/2; i++ {
			opp := len(domain) - 1 - i
			domain[i], domain[opp] = domain[opp], domain[i]
		}
	}

	pkgDomain := strings.Join(domain, ".")
	appid := []rune(pkgDomain + name)

	// a Java-language-style package name may contain upper- and lower-case
	// letters and underscores with individual parts separated by '.'.
	// https://developer.android.com/guide/topics/manifest/manifest-element
	for i, c := range appid {
		if !('a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' ||
			c == '_' || c == '.') {
			appid[i] = '_'
		}
	}
	return string(appid)
}

func getPkgName(pkgMetadata *packageMetadata) string {
	return path.Base(pkgMetadata.PkgPath)
}
