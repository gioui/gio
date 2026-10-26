// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"testing"

	"gioui.org/io/event"
)

func TestFileDropEvent(t *testing.T) {
	var e event.Event = FileDropEvent{Paths: []string{"/tmp/a.pdf"}}
	fd, ok := e.(FileDropEvent)
	if !ok {
		t.Fatal("FileDropEvent must implement event.Event")
	}
	if len(fd.Paths) != 1 || fd.Paths[0] != "/tmp/a.pdf" {
		t.Fatalf("paths lost: %#v", fd.Paths)
	}
	var empty event.Event = FileDropEvent{}
	if len(empty.(FileDropEvent).Paths) != 0 {
		t.Fatal("empty drop must stay empty")
	}
}
