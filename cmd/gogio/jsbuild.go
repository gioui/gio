// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"golang.org/x/tools/go/packages"
)

func buildJS(bi *buildInfo) error {
	out := *destPath
	if out == "" {
		out = bi.name
	}
	if err := os.MkdirAll(out, 0700); err != nil {
		return err
	}
	cmd := exec.Command(
		"go",
		"build",
		"-ldflags="+bi.ldflags,
		"-tags="+bi.tags,
		"-o", filepath.Join(out, "main.wasm"),
		bi.pkgPath,
	)
	cmd.Env = append(
		os.Environ(),
		"GOOS=js",
		"GOARCH=wasm",
	)
	_, err := runCmd(cmd)
	if err != nil {
		return err
	}

	var faviconPath string
	if _, err := os.Stat(bi.iconPath); err == nil {
		// Copy icon to the output folder
		icon, err := ioutil.ReadFile(bi.iconPath)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(filepath.Join(out, filepath.Base(bi.iconPath)), icon, 0600); err != nil {
			return err
		}
		faviconPath = filepath.Base(bi.iconPath)
	}

	indexTemplate, err := template.New("").Parse(jsIndex)
	if err != nil {
		return err
	}

	var b bytes.Buffer
	if err := indexTemplate.Execute(&b, struct {
		Name string
		Icon string
	}{
		Name: bi.name,
		Icon: faviconPath,
	}); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(out, "index.html"), b.Bytes(), 0600); err != nil {
		return err
	}

	goroot, err := runCmd(exec.Command("go", "env", "GOROOT"))
	if err != nil {
		return err
	}
	wasmJS := filepath.Join(goroot, "misc", "wasm", "wasm_exec.js")
	if _, err := os.Stat(wasmJS); err != nil {
		return fmt.Errorf("failed to find $GOROOT/misc/wasm/wasm_exec.js driver: %v", err)
	}
	pkgs, err := packages.Load(&packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps,
		Env:  append(os.Environ(), "GOOS=js", "GOARCH=wasm"),
	}, bi.pkgPath)
	if err != nil {
		return err
	}
	extraJS, err := findPackagesJS(pkgs[0], make(map[string]bool))
	if err != nil {
		return err
	}

	return mergeJSFiles(filepath.Join(out, "wasm.js"), append([]string{wasmJS}, extraJS...)...)
}

func findPackagesJS(p *packages.Package, visited map[string]bool) (extraJS []string, err error) {
	if len(p.GoFiles) == 0 {
		return nil, nil
	}
	js, err := filepath.Glob(filepath.Join(filepath.Dir(p.GoFiles[0]), "*_js.js"))
	if err != nil {
		return nil, err
	}
	extraJS = append(extraJS, js...)
	for _, imp := range p.Imports {
		if !visited[imp.ID] {
			extra, err := findPackagesJS(imp, visited)
			if err != nil {
				return nil, err
			}
			extraJS = append(extraJS, extra...)
			visited[imp.ID] = true
		}
	}
	return extraJS, nil
}

// mergeJSFiles will merge all files into a single `wasm.js`. It will prepend the jsSetGo
// and append the jsStartGo.
func mergeJSFiles(dst string, files ...string) (err error) {
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := w.Close(); err != nil {
			err = cerr
		}
	}()
	_, err = io.Copy(w, strings.NewReader(jsSetGo))
	if err != nil {
		return err
	}
	for i := range files {
		r, err := os.Open(files[i])
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		r.Close()
		if err != nil {
			return err
		}
	}
	_, err = io.Copy(w, strings.NewReader(jsStartGo))
	return err
}

const (
	jsIndex = `<!doctype html>
<html>
	<head>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, user-scalable=no">
		<meta name="mobile-web-app-capable" content="yes">
		{{ if .Icon }}<link rel="icon" href="{{.Icon}}" type="image/x-icon" />{{ end }}
		{{ if .Name }}<title>{{.Name}}</title>{{ end }}
		<script src="wasm.js"></script>
		<style>
			body,pre { margin:0;padding:0; }
		</style>
	</head>
	<body>
	</body>
</html>`
	// jsSetGo sets the `window.go` variable.
	jsSetGo = `(() => {
    window.go = {argv: [], env: {}, importObject: {go: {}}};
	const argv = new URLSearchParams(location.search).get("argv");
	if (argv) {
		window.go["argv"] = argv.split(" ");
	}
})();`
	// jsStartGo initializes the main.wasm.
	jsStartGo = `(() => {
	defaultGo = new Go();
	Object.assign(defaultGo["argv"], defaultGo["argv"].concat(go["argv"]));
	Object.assign(defaultGo["env"], go["env"]);
	for (let key in go["importObject"]) {
		if (typeof defaultGo["importObject"][key] === "undefined") {
			defaultGo["importObject"][key] = {};
		}
		Object.assign(defaultGo["importObject"][key], go["importObject"][key]);
	}
	window.go = defaultGo;
    if (!WebAssembly.instantiateStreaming) { // polyfill
        WebAssembly.instantiateStreaming = async (resp, importObject) => {
            const source = await (await resp).arrayBuffer();
            return await WebAssembly.instantiate(source, importObject);
        };
    }
    WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject).then((result) => {
        go.run(result.instance);
    });
})();`
)
