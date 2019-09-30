// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"fmt"

	"gioui.org/internal/ops"
)

type resourceCache struct {
	res    map[interface{}]resource
	newRes map[interface{}]resource
}

// opCache is like a resourceCache using the concrete Key
// key type to avoid allocations.
type opCache struct {
	res    map[ops.Key]resource
	newRes map[ops.Key]resource
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

func (r *resourceCache) frame(ctx *context) {
	for k, v := range r.res {
		if _, exists := r.newRes[k]; !exists {
			delete(r.res, k)
			v.release(ctx)
		}
	}
	for k, v := range r.newRes {
		delete(r.newRes, k)
		r.res[k] = v
	}
}

func (r *resourceCache) release(ctx *context) {
	for _, v := range r.newRes {
		v.release(ctx)
	}
	r.newRes = nil
	r.res = nil
}

func newOpCache() *opCache {
	return &opCache{
		res:    make(map[ops.Key]resource),
		newRes: make(map[ops.Key]resource),
	}
}

func (r *opCache) get(key ops.Key) (resource, bool) {
	v, exists := r.res[key]
	if exists {
		r.newRes[key] = v
	}
	return v, exists
}

func (r *opCache) put(key ops.Key, val resource) {
	if _, exists := r.newRes[key]; exists {
		panic(fmt.Errorf("key exists, %#v", key))
	}
	r.res[key] = val
	r.newRes[key] = val
}

func (r *opCache) frame(ctx *context) {
	for k, v := range r.res {
		if _, exists := r.newRes[k]; !exists {
			delete(r.res, k)
			v.release(ctx)
		}
	}
	for k, v := range r.newRes {
		delete(r.newRes, k)
		r.res[k] = v
	}
}

func (r *opCache) release(ctx *context) {
	for _, v := range r.newRes {
		v.release(ctx)
	}
	r.newRes = nil
	r.res = nil
}
