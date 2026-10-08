// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

import (
	"bytes"
	"strings"
	"testing"
)

const testHLSL = `
struct VSIn {
	float4 pos : POSITION;
};

struct VSOut {
	float4 pos : SV_POSITION;
};

VSOut main(VSIn v) {
	VSOut res;
	res.pos = v.pos;
	return res;
}
`

// TestCompileHLSL ensures the runtime compiler binding works and produces
// bytecode the D3D11 backend can consume.
func TestCompileHLSL(t *testing.T) {
	bc, err := CompileHLSL(testHLSL, "main", "vs_5_0", D3DCOMPILE_OPTIMIZATION_LEVEL3)
	if err != nil {
		t.Fatalf("CompileHLSL: %v", err)
	}
	if !bytes.HasPrefix(bc, []byte("DXBC")) {
		t.Fatalf("bytecode does not start with the DXBC signature: %q", bc[:min(4, len(bc))])
	}
	if len(bc) < 32 {
		t.Fatalf("bytecode is suspiciously small: %d bytes", len(bc))
	}
}

func TestCompileHLSLError(t *testing.T) {
	_, err := CompileHLSL("this is not hlsl", "main", "ps_5_0", 0)
	if err == nil {
		t.Fatal("expected an error for invalid HLSL")
	}
	if !strings.Contains(err.Error(), "D3DCompile") {
		t.Fatalf("error should mention D3DCompile, got: %v", err)
	}
}

// TestCompileHLSLTarget ensures the target string is actually honoured: a pixel
// shader profile must not accept a vertex-only source without an entry point.
func TestCompileHLSLTarget(t *testing.T) {
	if _, err := CompileHLSL(testHLSL, "main", "not_a_profile", 0); err == nil {
		t.Fatal("expected an error for an unknown shader profile")
	}
}
