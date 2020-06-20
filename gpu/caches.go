// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"fmt"

	"gioui.org/f32"
	"gioui.org/internal/ops"
)

type resourceCache struct {
	res    map[interface{}]resource
	newRes map[interface{}]resource
}

// opCache is like a resourceCache but using concrete types.
type opCache struct {
	res    map[ops.Key]opCacheValue
	newRes map[ops.Key]opCacheValue
}

type opCacheValue struct {
	data   *pathData
	bounds f32.Rectangle
}

func newResourceCache() *resourceCache {
	return &resourceCache{
		res:    make(map[interface{}]resource),
		newRes: make(map[interface{}]resource),
	}
}

func (r *resourceCache) get(key interface{}) (resource, bool) {
	v, exists := r.res[key]
	if exists {
		r.newRes[key] = v
	}
	return v, exists
}

func (r *resourceCache) put(key interface{}, val resource) {
	if _, exists := r.newRes[key]; exists {
		panic(fmt.Errorf("key exists, %p", key))
	}
	r.res[key] = val
	r.newRes[key] = val
}

func (r *resourceCache) frame() {
	for k, v := range r.res {
		if _, exists := r.newRes[k]; !exists {
			delete(r.res, k)
			v.release()
		}
	}
	for k, v := range r.newRes {
		delete(r.newRes, k)
		r.res[k] = v
	}
}

func (r *resourceCache) release() {
	for _, v := range r.newRes {
		v.release()
	}
	r.newRes = nil
	r.res = nil
}

func newOpCache() *opCache {
	return &opCache{
		res:    make(map[ops.Key]opCacheValue),
		newRes: make(map[ops.Key]opCacheValue),
	}
}

func (r *opCache) get(key ops.Key) (opCacheValue, bool) {
	v, exists := r.res[key]
	if exists {
		r.newRes[key] = v
	}
	return v, exists
}

func (r *opCache) put(key ops.Key, val opCacheValue) {
	if _, exists := r.newRes[key]; exists {
		panic(fmt.Errorf("key exists, %#v", key))
	}
	r.res[key] = val
	r.newRes[key] = val
}

func (r *opCache) frame() {
	for k, v := range r.res {
		if _, exists := r.newRes[k]; !exists {
			delete(r.res, k)
			v.data.release()
		}
	}
	for k, v := range r.newRes {
		delete(r.newRes, k)
		r.res[k] = v
	}
}

func (r *opCache) release() {
	for _, v := range r.newRes {
		v.data.release()
	}
	r.newRes = nil
	r.res = nil
}
