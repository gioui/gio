// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import (
	"errors"
	"sync"
	"sync/atomic"
)

// OHOSPermission names used by the runtime permission bridge. Callers may
// also pass a raw OHOS permission string to OHOSRequestPermission.
const (
	OHOSPermissionCamera     = "ohos.permission.CAMERA"
	OHOSPermissionMicrophone = "ohos.permission.MICROPHONE"
	OHOSPermissionStorage    = "ohos.permission.READ_MEDIA"
	OHOSPermissionBluetooth  = "ohos.permission.ACCESS_BLUETOOTH"
	OHOSPermissionNetwork    = "ohos.permission.GET_NETWORK_INFO"
	OHOSPermissionWakeLock   = "ohos.permission.RUNNING_LOCK"
)

var errOHOSPermissionUnsupported = errors.New("ohos: runtime permission request is not supported by the shim")
var errOHOSPermissionNoWindow = errors.New("ohos: no active window for permission request")

var ohosPermissions = struct {
	mu      sync.Mutex
	next    atomic.Int64
	pending map[int]func(granted bool, err error)
}{pending: make(map[int]func(granted bool, err error))}

// requestOHOSPermission asks host for permission and registers callback.
// It reports whether the request was dispatched.
func requestOHOSPermission(host ohosHost, name string, callback func(granted bool, err error)) bool {
	if host == nil || name == "" {
		if callback != nil {
			callback(false, errOHOSPermissionUnsupported)
		}
		return false
	}
	id := int(ohosPermissions.next.Add(1))
	ohosPermissions.mu.Lock()
	ohosPermissions.pending[id] = callback
	ohosPermissions.mu.Unlock()
	if !host.RequestPermission(name, id) {
		ohosPermissions.mu.Lock()
		delete(ohosPermissions.pending, id)
		ohosPermissions.mu.Unlock()
		if callback != nil {
			callback(false, errOHOSPermissionUnsupported)
		}
		return false
	}
	return true
}

// deliverOHOSPermission resolves a pending request. It is called on the
// loop goroutine; callbacks run without the registry lock held.
func deliverOHOSPermission(id int, granted bool, errMsg string) {
	ohosPermissions.mu.Lock()
	cb := ohosPermissions.pending[id]
	delete(ohosPermissions.pending, id)
	ohosPermissions.mu.Unlock()
	if cb == nil {
		return
	}
	var err error
	if errMsg != "" {
		err = errors.New(errMsg)
	}
	cb(granted, err)
}
