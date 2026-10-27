// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos || openharmony || gio_ohos_test

package app

import (
	"encoding/json"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
)

// ohosSemanticNode is the wire shape of one semantic node. It is
// deliberately small: the shim only needs labels, bounds and actions.
type ohosSemanticNode struct {
	ID       uint     `json:"id"`
	Parent   uint     `json:"parent"`
	Class    string   `json:"class,omitempty"`
	Label    string   `json:"label,omitempty"`
	Desc     string   `json:"desc,omitempty"`
	Selected bool     `json:"selected,omitempty"`
	Disabled bool     `json:"disabled,omitempty"`
	Bounds   [4]int   `json:"bounds"`
	Actions  []string `json:"actions,omitempty"`
}

type ohosSemanticPayload struct {
	Root    uint               `json:"root"`
	Nodes   []ohosSemanticNode `json:"nodes"`
	Removed []uint             `json:"removed,omitempty"`
}

// appendOHOSSemanticNodes appends n and its descendants in pre-order.
func appendOHOSSemanticNodes(dst []input.SemanticNode, n input.SemanticNode) []input.SemanticNode {
	dst = append(dst, n)
	for _, ch := range n.Children {
		dst = appendOHOSSemanticNodes(dst, ch)
	}
	return dst
}

func ohosSemanticClassName(c semantic.ClassOp) string {
	switch c {
	case semantic.Button:
		return "button"
	case semantic.CheckBox:
		return "checkbox"
	case semantic.Editor:
		return "editor"
	case semantic.RadioButton:
		return "radio"
	case semantic.Switch:
		return "switch"
	default:
		return ""
	}
}

func ohosSemanticsJSON(root input.SemanticID, nodes []input.SemanticNode, removed []input.SemanticID) []byte {
	payload := ohosSemanticPayload{Root: uint(root)}
	for _, n := range nodes {
		d := n.Desc
		entry := ohosSemanticNode{
			ID:       uint(n.ID),
			Parent:   uint(n.ParentID),
			Class:    ohosSemanticClassName(d.Class),
			Label:    d.Label,
			Desc:     d.Description,
			Selected: d.Selected,
			Disabled: d.Disabled,
			Bounds:   [4]int{d.Bounds.Min.X, d.Bounds.Min.Y, d.Bounds.Max.X, d.Bounds.Max.Y},
		}
		if d.Gestures&input.ClickGesture != 0 {
			entry.Actions = append(entry.Actions, "click")
		}
		if d.Gestures&input.ScrollGesture != 0 {
			entry.Actions = append(entry.Actions, "scroll")
		}
		payload.Nodes = append(payload.Nodes, entry)
	}
	for _, id := range removed {
		payload.Removed = append(payload.Removed, uint(id))
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return b
}
