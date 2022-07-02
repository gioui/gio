// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import "testing"

func BenchmarkResourceCache(b *testing.B) {
	offset := 0
	const N = 100

	cache := newResourceCache()
	for i := 0; i < b.N; i++ {
		// half are the same and half updated
		for k := 0; k < N; k++ {
			cache.put(offset+k, nullResource{})
		}
		cache.frame()
		offset += N / 2
	}
}

type nullResource struct{}

func (nullResource) release() {}
