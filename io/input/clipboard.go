// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
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

func (q *clipboardQueue) ClipboardRequested(state clipboardState) bool {
	req := len(state.receivers) > 0 && q.requested
	q.requested = false
	return req
}

func (q *clipboardQueue) Push(state clipboardState, e transfer.DataEvent) (clipboardState, []taggedEvent) {
	var evts []taggedEvent
	var remaining []clipboard.ReadCmd
	for _, request := range state.receivers {
		if !containsAllTypes(e.Types, request.Types) {
			remaining = append(remaining, request)
			continue
		}
		reply := transfer.DataEvent{Types: request.Types, Open: e.Open}
		evts = append(evts, taggedEvent{tag: request.Tag, event: reply})
	}
	state.receivers = remaining
	return state, evts
}

func containsAllTypes(types, requested []string) bool {
	for _, mime := range requested {
		if !slices.Contains(types, mime) {
			return false
		}
	}
	return true
}

func (q *clipboardQueue) ProcessWriteClipboard(req clipboard.WriteCmd) {
	q.contents = make([]clipboard.Content, 0, len(req.Contents))
	q.sensitive = req.Sensitive
	for _, c := range req.Contents {
		if c.Type == "" {
			continue
		}
		c.Data = slices.Clone(c.Data)
		q.contents = append(q.contents, c)
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
	var types []string
	for _, mime := range req.Types {
		if mime != "" && !slices.Contains(types, mime) {
			types = append(types, mime)
		}
	}
	if len(types) == 0 {
		types = []string{"application/text"}
	}
	req.Types = types
	for _, pending := range state.receivers {
		if pending.Tag == req.Tag && slices.Equal(pending.Types, types) {
			return state
		}
	}
	n := len(state.receivers)
	state.receivers = append(state.receivers[:n:n], req)
	q.requested = true
	return state
}
