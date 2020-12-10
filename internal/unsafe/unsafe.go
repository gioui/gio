// SPDX-License-Identifier: Unlicense OR MIT

package unsafe

import (
	"reflect"
	"unsafe"
)

// BytesView returns a byte slice view of a slice.
func BytesView(s interface{}) []byte {
	v := reflect.ValueOf(s)
	first := v.Index(0)
	sz := int(first.Type().Size())
	var res []byte
	h := (*reflect.SliceHeader)(unsafe.Pointer(&res))
	h.Data = first.UnsafeAddr()
	h.Cap = v.Cap() * sz
	h.Len = v.Len() * sz
	return res
}

// SliceOf returns a slice from a (native) pointer.
func SliceOf(s uintptr) []byte {
	if s == 0 {
		return nil
	}
	var res []byte
	h := (*reflect.SliceHeader)(unsafe.Pointer(&res))
	h.Data = s
	h.Cap = 1 << 30
	return res
}

// GoString convert a NUL-terminated C string
// to a Go string.
func GoString(s []byte) string {
	for i, v := range s {
		if v == 0 {
			return string(s[:i])
		}
	}
	return string(s)
}
