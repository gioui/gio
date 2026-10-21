// SPDX-License-Identifier: Unlicense OR MIT

package app

import "sync"

// ForegroundOptions describes the notification StartForeground shows.
type ForegroundOptions struct {
	Title string
	Text  string
}

var foreground struct {
	mu    sync.Mutex
	users int
}

// StartForeground keeps the program running while its window is not in front,
// until every returned stop function has been called. On Android that takes a
// foreground service showing a notification that opens the app, and the app
// must import package gioui.org/app/permission/foreground. Elsewhere a program
// keeps running anyway, and it does nothing.
func StartForeground(opts ForegroundOptions) (stop func(), err error) {
	foreground.mu.Lock()
	defer foreground.mu.Unlock()
	if err := startForeground(opts); err != nil {
		return nil, err
	}
	foreground.users++
	var once sync.Once
	return func() {
		once.Do(func() {
			foreground.mu.Lock()
			defer foreground.mu.Unlock()
			foreground.users--
			if foreground.users == 0 {
				stopForeground()
			}
		})
	}, nil
}
