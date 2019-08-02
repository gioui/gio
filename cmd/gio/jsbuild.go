// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
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
		"-o", filepath.Join(out, "main.wasm"),
		bi.pkg,
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
	const indexhtml = `<!doctype html>
<html>
	<head>
		<meta charset="utf-8">
		<meta name="viewport" content="width=device-width, user-scalable=no">
		<meta name="mobile-web-app-capable" content="yes">
		<script src="wasm_exec.js"></script>
		<script>
			if (!WebAssembly.instantiateStreaming) { // polyfill
				WebAssembly.instantiateStreaming = async (resp, importObject) => {
					const source = await (await resp).arrayBuffer();
					return await WebAssembly.instantiate(source, importObject);
				};
			}

			const go = new Go();
			WebAssembly.instantiateStreaming(fetch("main.wasm"), go.importObject).then((result) => {
				go.run(result.instance);
			});
		</script>
		<style>
			body,pre { margin:0;padding:0; }
		</style>
	</head>
	<body>
	</body>
</html>`
	if err := ioutil.WriteFile(filepath.Join(out, "index.html"), []byte(indexhtml), 0600); err != nil {
		return err
	}
	goroot, err := runCmd(exec.Command("go", "env", "GOROOT"))
	if err != nil {
		return err
	}
	wasmjs := filepath.Join(goroot, "misc", "wasm", "wasm_exec.js")
	if _, err := os.Stat(wasmjs); err != nil {
		return fmt.Errorf("failed to find $GOROOT/misc/wasm/wasm_exec.js driver: %v", err)
	}
	return copyFile(filepath.Join(out, "wasm_exec.js"), wasmjs)
}
