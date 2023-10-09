// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"io"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
)

type clipboardQueue struct {
	receivers map[event.Tag]struct{}
	// request avoid read clipboard every frame while waiting.
	requested bool
	mime      string
	text      []byte
}

// WriteClipboard returns the most recent data to be copied
// to the clipboard, if any.
func (q *clipboardQueue) WriteClipboard() (mime string, content []byte, ok bool) {
	if q.text == nil {
		return "", nil, false
	}
	content = q.text
	q.text = nil
	return q.mime, content, true
}

// ReadClipboard reports if any new handler is waiting
// to read the clipboard.
func (q *clipboardQueue) ReadClipboard() bool {
	if len(q.receivers) == 0 || q.requested {
		return false
	}
	q.requested = true
	return true
}

func (q *clipboardQueue) Push(e event.Event, events *handlerEvents) {
	for r := range q.receivers {
		events.Add(r, e)
		delete(q.receivers, r)
	}
}

func (q *clipboardQueue) ProcessWriteClipboard(req clipboard.WriteCmd) {
	defer req.Data.Close()
	content, err := io.ReadAll(req.Data)
	if err != nil {
		return
	}
	q.mime = req.Type
	q.text = content
}

func (q *clipboardQueue) ProcessReadClipboard(refs []interface{}) {
	if q.receivers == nil {
		q.receivers = make(map[event.Tag]struct{})
	}
	tag := refs[0].(event.Tag)
	if _, ok := q.receivers[tag]; !ok {
		q.receivers[tag] = struct{}{}
		q.requested = false
	}
}
