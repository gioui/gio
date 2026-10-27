// SPDX-License-Identifier: Unlicense OR MIT

//go:build gio_ohos_test

package app

import (
	"encoding/json"
	"image"
	"testing"

	"gioui.org/io/input"
	"gioui.org/io/semantic"
)

func TestOHOSSemanticsJSON(t *testing.T) {
	root := input.SemanticID(1)
	nodes := []input.SemanticNode{
		{
			ID:       2,
			ParentID: 1,
			Desc: input.SemanticDesc{
				Class:    semantic.Button,
				Label:    "OK",
				Bounds:   image.Rect(10, 20, 90, 50),
				Gestures: input.ClickGesture | input.ScrollGesture,
			},
		},
	}
	removed := []input.SemanticID{9}
	b := ohosSemanticsJSON(root, nodes, removed)
	if len(b) == 0 {
		t.Fatal("empty payload")
	}
	var got ohosSemanticPayload
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Root != 1 || len(got.Nodes) != 1 || len(got.Removed) != 1 || got.Removed[0] != 9 {
		t.Fatalf("payload = %+v", got)
	}
	n := got.Nodes[0]
	if n.ID != 2 || n.Parent != 1 || n.Class != "button" || n.Label != "OK" {
		t.Fatalf("node = %+v", n)
	}
	if n.Bounds != [4]int{10, 20, 90, 50} {
		t.Fatalf("bounds = %v", n.Bounds)
	}
	if len(n.Actions) != 2 || n.Actions[0] != "click" || n.Actions[1] != "scroll" {
		t.Fatalf("actions = %v", n.Actions)
	}
}

func TestOHOSAppendSemanticNodes(t *testing.T) {
	root := input.SemanticNode{
		ID: 1,
		Children: []input.SemanticNode{
			{ID: 2},
			{ID: 3, Children: []input.SemanticNode{{ID: 4}}},
		},
	}
	got := appendOHOSSemanticNodes(nil, root)
	if len(got) != 4 || got[0].ID != 1 || got[1].ID != 2 || got[2].ID != 3 || got[3].ID != 4 {
		t.Fatalf("order = %v", got)
	}
}

func TestOHOSPumpSemanticAction(t *testing.T) {
	p, f := newTestPump()
	p.Handle(ohosEvent{kind: ohosEvSemanticAction, a: 7, b: 1})
	if len(f.semActs) != 1 || f.semActs[0] != [2]int{7, 1} {
		t.Fatalf("actions = %v", f.semActs)
	}
}
