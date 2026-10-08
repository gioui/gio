// SPDX-License-Identifier: Unlicense OR MIT

package d3d11

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var d3dcompiler = windows.NewLazySystemDLL("d3dcompiler_47.dll")

// Compilation flags, as in d3dcompiler.h.
const (
	D3DCOMPILE_DEBUG                      = 0x00000001
	D3DCOMPILE_SKIP_OPTIMIZATION          = 0x00000002
	D3DCOMPILE_PACK_MATRIX_COLUMN_MAJOR   = 0x00000004
	D3DCOMPILE_PACK_MATRIX_ROW_MAJOR      = 0x00000008
	D3DCOMPILE_PACK_OUTPUT_RSGS           = 0x00000010
	D3DCOMPILE_IEEE_STRICTNESS            = 0x00000020
	D3DCOMPILE_OPTIMIZATION_LEVEL0        = 0x00000000
	D3DCOMPILE_OPTIMIZATION_LEVEL1        = 0x00004000
	D3DCOMPILE_OPTIMIZATION_LEVEL2        = 0x00008000
	D3DCOMPILE_OPTIMIZATION_LEVEL3        = 0x0000c000
	D3DCOMPILE_PREFER_FIRST_MIN_PRECISION = 0x00008000
	D3DCOMPILE_PREFER_HIGHEST_PRECISION   = 0x00004000
)

// Blob is an ID3DBlob, the container D3DCompile returns bytecode in.
type Blob struct {
	Vtbl *struct {
		_IUnknownVTbl
		GetBufferPointer uintptr
		GetBufferSize    uintptr
	}
}

// MacroShader is the legacy IMacroShader interface. D3DCompile only hands one
// back so that callers have something to release.
type MacroShader struct {
	Vtbl *struct {
		_IUnknownVTbl
	}
}

// bytes returns a view of the blob's contents. The view is only valid until
// the blob is released, so callers that keep the data must copy it first.
func (b *Blob) bytes() []byte {
	if b == nil {
		return nil
	}
	ptr, _, _ := syscall.Syscall(
		b.Vtbl.GetBufferPointer,
		1,
		uintptr(unsafe.Pointer(b)),
		0,
		0,
	)
	size, _, _ := syscall.Syscall(
		b.Vtbl.GetBufferSize,
		1,
		uintptr(unsafe.Pointer(b)),
		0,
		0,
	)
	if ptr == 0 || size == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(ptr)), int(size))
}

var _D3DCompile = d3dcompiler.NewProc("D3DCompile")

// CompileHLSL compiles HLSL source to DXBC bytecode using the shader compiler
// that ships with Windows, the same one fxc.exe drives. This lets callers build
// shaders at runtime without the Windows SDK and without cgo.
//
// The returned bytecode can be handed to CreateVertexShader,
// CreatePixelShader or CreateComputeShader.
func CompileHLSL(source, entry, target string, flags uint32) ([]byte, error) {
	var (
		code *Blob
		errs *Blob
	)
	src := append([]byte(source), 0)
	srcName, err := windows.BytePtrFromString(source)
	if err != nil {
		return nil, err
	}
	entryName, err := windows.BytePtrFromString(entry)
	if err != nil {
		return nil, err
	}
	targetName, err := windows.BytePtrFromString(target)
	if err != nil {
		return nil, err
	}
	r, _, _ := syscall.SyscallN(
		_D3DCompile.Addr(),
		uintptr(unsafe.Pointer(&src[0])),
		uintptr(len(src)-1),
		uintptr(unsafe.Pointer(srcName)),
		0,
		0,
		uintptr(unsafe.Pointer(entryName)),
		uintptr(unsafe.Pointer(targetName)),
		uintptr(flags),
		0,
		uintptr(unsafe.Pointer(&code)),
		uintptr(unsafe.Pointer(&errs)),
	)
	if errs != nil {
		msg := string(errs.bytes())
		IUnknownRelease(unsafe.Pointer(errs), errs.Vtbl.Release)
		if msg != "" {
			return nil, fmt.Errorf("d3d11: D3DCompile: %s", msg)
		}
	}
	if r != 0 {
		return nil, ErrorCode{Name: "D3DCompile", Code: uint32(r)}
	}
	// The bytecode lives in the blob, which dies with the release below, so
	// copy it out before returning.
	bc := code.bytes()
	if len(bc) == 0 {
		IUnknownRelease(unsafe.Pointer(code), code.Vtbl.Release)
		return nil, fmt.Errorf("d3d11: D3DCompile: empty bytecode")
	}
	out := make([]byte, len(bc))
	copy(out, bc)
	IUnknownRelease(unsafe.Pointer(code), code.Vtbl.Release)
	return out, nil
}
