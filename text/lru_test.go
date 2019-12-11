// SPDX-License-Identifier: Unlicense OR MIT

package text

import (
	"strconv"
	"testing"

	"gioui.org/op"
)

func TestLayoutLRU(t *testing.T) {
	c := new(layoutCache)
	put := func(i int) {
		c.Put(layoutKey{str: strconv.Itoa(i)}, nil)
	}
	get := func(i int) bool {
		_, ok := c.Get(layoutKey{str: strconv.Itoa(i)})
		return ok
	}
	testLRU(t, put, get)
}

func TestPathLRU(t *testing.T) {
	c := new(pathCache)
	put := func(i int) {
		c.Put(pathKey{str: strconv.Itoa(i)}, op.CallOp{})
	}
	get := func(i int) bool {
		_, ok := c.Get(pathKey{str: strconv.Itoa(i)})
		return ok
	}
	testLRU(t, put, get)
}

func testLRU(t *testing.T, put func(i int), get func(i int) bool) {
	for i := 0; i < maxSize; i++ {
		put(i)
	}
	for i := 0; i < maxSize; i++ {
		if !get(i) {
			t.Fatalf("key %d was evicted", i)
		}
	}
	put(maxSize)
	for i := 1; i < maxSize+1; i++ {
		if !get(i) {
			t.Fatalf("key %d was evicted", i)
		}
	}
	if i := 0; get(i) {
		t.Fatalf("key %d was not evicted", i)
	}
}
