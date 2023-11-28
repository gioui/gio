// SPDX-License-Identifier: Unlicense OR MIT

package input

import (
	"fmt"
	"image"
	"reflect"
	"testing"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/io/semantic"
	"gioui.org/op"
	"gioui.org/op/clip"
)

func TestEmptySemantics(t *testing.T) {
	var r Router
	tree := r.AppendSemantics(nil)
	if len(tree) != 1 {
		t.Errorf("expected 1 semantic node for empty tree, got %d", len(tree))
	}
}

func TestSemanticTree(t *testing.T) {
	var (
		ops op.Ops
		r   Router
	)
	t1 := clip.Rect(image.Rect(0, 0, 75, 75)).Push(&ops)
	semantic.DescriptionOp("child1").Add(&ops)
	t1.Pop()
	t2 := clip.Rect(image.Rect(25, 25, 100, 100)).Push(&ops)
	semantic.DescriptionOp("child2").Add(&ops)
	t2.Pop()
	r.Frame(&ops)
	tests := []struct {
		x, y float32
		desc string
	}{
		{24, 24, "child1"},
		{50, 50, "child2"},
		{100, 100, ""},
	}
	tree := r.AppendSemantics(nil)
	verifyTree(t, 0, tree[0])
	for _, test := range tests {
		p := f32.Pt(test.x, test.y)
		id, found := r.SemanticAt(p)
		if !found {
			t.Errorf("no semantic node at %v", p)
		}
		n, found := lookupNode(tree, id)
		if !found {
			t.Errorf("no id %d in semantic tree", id)
		}
		if got := n.Desc.Description; got != test.desc {
			t.Errorf("got semantic description %s at %v, expected %s", got, p, test.desc)
		}
	}

	// Verify stable IDs.
	r.Frame(&ops)
	tree2 := r.AppendSemantics(nil)
	if !reflect.DeepEqual(tree, tree2) {
		fmt.Println("First tree:")
		printTree(0, tree[0])
		fmt.Println("Second tree:")
		printTree(0, tree2[0])
		t.Error("same semantic description lead to differing trees")
	}
}

func TestSemanticDescription(t *testing.T) {
	var ops op.Ops

	h := new(int)
	event.Op(&ops, h)
	semantic.DescriptionOp("description").Add(&ops)
	semantic.LabelOp("label").Add(&ops)
	semantic.Button.Add(&ops)
	semantic.EnabledOp(false).Add(&ops)
	semantic.SelectedOp(true).Add(&ops)
	var r Router
	events(&r, -1, pointer.Filter{
		Target: h,
		Kinds:  pointer.Press | pointer.Release,
	})
	r.Frame(&ops)
	tree := r.AppendSemantics(nil)
	got := tree[0].Desc
	exp := SemanticDesc{
		Class:       1,
		Description: "description",
		Label:       "label",
		Selected:    true,
		Disabled:    true,
		Gestures:    ClickGesture,
		Bounds:      image.Rectangle{Min: image.Point{X: -1e+06, Y: -1e+06}, Max: image.Point{X: 1e+06, Y: 1e+06}},
	}
	if got != exp {
		t.Errorf("semantic description mismatch:\nGot:  %+v\nWant: %+v", got, exp)
	}
}

func lookupNode(tree []SemanticNode, id SemanticID) (SemanticNode, bool) {
	for _, n := range tree {
		if id == n.ID {
			return n, true
		}
	}
	return SemanticNode{}, false
}

func verifyTree(t *testing.T, parent SemanticID, n SemanticNode) {
	t.Helper()
	if n.ParentID != parent {
		t.Errorf("node %d: got parent %d, want %d", n.ID, n.ParentID, parent)
	}
	for _, c := range n.Children {
		verifyTree(t, n.ID, c)
	}
}

func printTree(indent int, n SemanticNode) {
	for i := 0; i < indent; i++ {
		fmt.Print("\t")
	}
	fmt.Printf("%d: %+v\n", n.ID, n.Desc)
	for _, c := range n.Children {
		printTree(indent+1, c)
	}
}
