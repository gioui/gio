// SPDX-License-Identifier: Unlicense OR MIT

// Package byteslice provides byte slice views of other Go values such as
// slices and structs.
package byteslice

import (
	"unsafe"
)

// View returns a byte slice view of a value.
func View[T any](s *T) []byte {
	sz := unsafe.Sizeof(*s)
	return unsafe.Slice((*byte)(unsafe.Pointer(s)), sz)
}

// Slice returns a byte slice view of a slice.
func Slice[T any](s []T) []byte {
	sz := unsafe.Sizeof(s[0])
	res := unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(s))), sz*uintptr(cap(s)))
	return res[:sz*uintptr(len(s))]
}
