// SPDX-License-Identifier: Unlicense OR MIT

package d3dcompile

import (
	"fmt"
	"syscall"
	"unsafe"

	gunsafe "gioui.org/internal/unsafe"

	"golang.org/x/sys/windows"
)

var (
	d3dcompiler_47 = windows.NewLazySystemDLL("d3dcompiler_47.dll")

	__D3DCompile = d3dcompiler_47.NewProc("D3DCompile")
)

type _IUnknownVTbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

type _ID3DBlob struct {
	vtbl *struct {
		_IUnknownVTbl
		GetBufferPointer uintptr
		GetBufferSize    uintptr
	}
}

func D3DCompile(src []byte, entryPoint, target string) ([]byte, error) {
	var (
		code   *_ID3DBlob
		errors *_ID3DBlob
	)
	entryPoint0 := []byte(entryPoint + "\x00")
	target0 := []byte(target + "\x00")
	r, _, _ := __D3DCompile.Call(
		uintptr(unsafe.Pointer(&src[0])),
		uintptr(len(src)),
		0, // pSourceName
		0, // pDefines
		0, // pInclude
		uintptr(unsafe.Pointer(&entryPoint0[0])),
		uintptr(unsafe.Pointer(&target0[0])),
		0, // Flags1
		0, // Flags2
		uintptr(unsafe.Pointer(&code)),
		uintptr(unsafe.Pointer(&errors)),
	)
	var compileErr string
	if errors != nil {
		compileErr = string(errors.data())
		_IUnknownRelease(unsafe.Pointer(errors), errors.vtbl.Release)
	}
	if r != 0 {
		return nil, fmt.Errorf("D3D11Compile: %#x: %s", r, compileErr)
	}
	bytecode := code.data()
	cp := make([]byte, len(bytecode))
	copy(cp, bytecode)
	_IUnknownRelease(unsafe.Pointer(code), code.vtbl.Release)
	return cp, nil
}

func (b *_ID3DBlob) GetBufferPointer() uintptr {
	ptr, _, _ := syscall.Syscall(
		b.vtbl.GetBufferPointer,
		1,
		uintptr(unsafe.Pointer(b)),
		0,
		0,
	)
	return ptr
}

func (b *_ID3DBlob) GetBufferSize() uintptr {
	sz, _, _ := syscall.Syscall(
		b.vtbl.GetBufferSize,
		1,
		uintptr(unsafe.Pointer(b)),
		0,
		0,
	)
	return sz
}

func (b *_ID3DBlob) data() []byte {
	data := gunsafe.SliceOf(b.GetBufferPointer())
	n := int(b.GetBufferSize())
	return data[:n:n]
}

func _IUnknownRelease(obj unsafe.Pointer, releaseMethod uintptr) {
	syscall.Syscall(
		releaseMethod,
		1,
		uintptr(obj),
		0,
		0,
	)
}
