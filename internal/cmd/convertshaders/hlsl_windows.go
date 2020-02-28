// SPDX-License-Identifier: Unlicense OR MIT

package main

import "gioui.org/internal/d3dcompile"

func compileHLSL(src, entry, profile string) ([]byte, error) {
	return d3dcompile.D3DCompile([]byte(src), entry, profile)
}
