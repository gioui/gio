// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"io"
	"slices"

	"gioui.org/io/clipboard"
	"gioui.org/io/transfer"
)

// clipboardState contains the state for clipboard event routing.
type clipboardState struct {
	receivers []clipboard.ReadCmd
}

type clipboardQueue struct {
	requested bool
	sensitive bool
	contents  []clipboard.Content
}

func (q *clipboardQueue) WriteClipboard() (mime string, content []byte, ok bool) {
	contents, ok := q.WriteContents()
	if !ok || len(contents) == 0 {
		return "", nil, false
	}
	return contents[0].Type, contents[0].Data, true
}

func (q *clipboardQueue) ClipboardRequested(state clipboardState) bool {
	req := len(state.receivers) > 0 && q.requested
	q.requested = false
	return req
}

func (q *clipboardQueue) Push(state clipboardState, e transfer.DataEvent) (clipboardState, []taggedEvent) {
	var evts []taggedEvent
	var remaining []clipboard.ReadCmd
	for _, r := range state.receivers {
		if r.Type == e.Type {
			evts = append(evts, taggedEvent{tag: r.Tag, event: e})
		} else {
			remaining = append(remaining, r)
		}
	}
	state.receivers = remaining
	return state, evts
}

func (q *clipboardQueue) ProcessWriteClipboard(req clipboard.WriteCmd) {
	defer req.Data.Close()
	content, err := io.ReadAll(req.Data)
	if err != nil {
		return
	}
	q.sensitive = req.Sensitive
	q.contents = []clipboard.Content{{Type: req.Type, Data: content}}
}

func (q *clipboardQueue) ProcessWriteContents(req clipboard.WriteContentsCmd) {
	q.contents = make([]clipboard.Content, 0, len(req.Contents))
	q.sensitive = req.Sensitive
	for _, c := range req.Contents {
		if c.Type == "" {
			continue
		}
		data := append([]byte(nil), c.Data...)
		q.contents = append(q.contents, clipboard.Content{Type: c.Type, Data: data})
	}
}

func (q *clipboardQueue) WriteContents() ([]clipboard.Content, bool) {
	if q.contents == nil {
		return nil, false
	}
	contents := q.contents
	q.contents = nil
	return contents, true
}

func (q *clipboardQueue) ProcessReadClipboard(state clipboardState, req clipboard.ReadCmd) clipboardState {
	if req.Type == "" {
		req.Type = "application/text"
	}
	if slices.Contains(state.receivers, req) {
		return state
	}
	n := len(state.receivers)
	state.receivers = append(state.receivers[:n:n], req)
	q.requested = true
	return state
}

func (q *clipboardQueue) ProcessReadContents(state clipboardState, req clipboard.ReadContentsCmd) clipboardState {
	if len(req.Types) == 0 {
		return q.ProcessReadClipboard(state, clipboard.ReadCmd{Tag: req.Tag})
	}
	for _, typ := range req.Types {
		state = q.ProcessReadClipboard(state, clipboard.ReadCmd{Tag: req.Tag, Type: typ})
	}
	return state
}
