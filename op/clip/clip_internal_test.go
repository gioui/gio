// SPDX-License-Identifier: Unlicense OR MIT

package clip

import (
	"gioui.org/f32"
	"gioui.org/op"
	"math"
	"testing"
)

func TestPath_MoveTo_LineTo(t *testing.T) {
	var ops op.Ops
	p := Path{}
	p.Begin(&ops)
	startPoint := f32.Pt(32, 32)
	endPoint := f32.Pt(64, 64)
	p.MoveTo(startPoint)
	p.LineTo(endPoint)
	pathSpec := p.End()

	minPoint := f32.Pt(32, 32)
	maxPoint := f32.Pt(64, 64)
	if pathSpec.bounds.Min == pathSpec.bounds.Max {
		t.Errorf("zero path")
	}
	if pathSpec.bounds.Min != minPoint.Round() {
		t.Errorf("pathSpec.bounds.Min = %v, want %v", pathSpec.bounds.Min, minPoint)
	}
	if pathSpec.bounds.Max != maxPoint.Round() {
		t.Errorf("pathSpec.bounds.Max = %v, want %v", pathSpec.bounds.Max, maxPoint)
	}
}

func TestPath_MoveTo_QuadTo(t *testing.T) {
	var ops op.Ops
	p := Path{}
	p.Begin(&ops)
	startPoint := f32.Pt(32, 32)
	midPoint := f32.Pt(60, 60)
	p.MoveTo(startPoint)
	p.QuadTo(midPoint.Sub(f32.Pt(-4, 0)), midPoint.Sub(f32.Pt(0, -4)))
	pathSpec := p.End()

	minPoint := f32.Pt(32, 32)
	maxPoint := f32.Pt(64, 64)
	if pathSpec.bounds.Min == pathSpec.bounds.Max {
		t.Errorf("zero path")
	}
	if pathSpec.bounds.Min != minPoint.Round() {
		t.Errorf("pathSpec.bounds.Min = %v, want %v", pathSpec.bounds.Min, minPoint)
	}
	if pathSpec.bounds.Max != maxPoint.Round() {
		t.Errorf("pathSpec.bounds.Max = %v, want %v", pathSpec.bounds.Max, maxPoint)
	}
}

func TestPath_MoveTo_ArcTo(t *testing.T) {
	// We need a tolerance here because of rounding errors.
	tolerance := f32.Pt(1, 1)

	var ops op.Ops
	p := Path{}
	p.Begin(&ops)
	arcStartPoint := f32.Pt(48, 32)
	arcCenterPoint := f32.Pt(48, 48)
	p.MoveTo(arcStartPoint)
	p.ArcTo(arcCenterPoint, arcCenterPoint, math.Pi*2)
	pathSpec := p.End()

	minPoint := f32.Pt(32, 32).Sub(tolerance).Round()
	maxPoint := f32.Pt(64, 64).Add(tolerance).Round()
	if pathSpec.bounds.Min == pathSpec.bounds.Max {
		t.Errorf("zero path")
	}
	if pathSpec.bounds.Min.X < minPoint.X || pathSpec.bounds.Min.Y < minPoint.Y {
		t.Errorf("pathSpec.bounds.Min = %v, want %v", pathSpec.bounds.Min, minPoint)
	}
	if pathSpec.bounds.Max.X > maxPoint.X || pathSpec.bounds.Max.Y > maxPoint.Y {
		t.Errorf("pathSpec.bounds.Max = %v, want %v", pathSpec.bounds.Max, maxPoint)
	}
}

func TestPath_MoveTo_CubeTo(t *testing.T) {
	var ops op.Ops
	p := Path{}
	p.Begin(&ops)
	startPoint := f32.Pt(32, 32)
	midPoint := f32.Pt(48, 48)
	endPoint := f32.Pt(64, 64)
	p.MoveTo(startPoint)
	p.CubeTo(midPoint.Sub(f32.Pt(-4, 0)), midPoint.Sub(f32.Pt(0, -4)), endPoint)
	pathSpec := p.End()

	minPoint := f32.Pt(32, 32)
	maxPoint := f32.Pt(64, 64)
	if pathSpec.bounds.Min == pathSpec.bounds.Max {
		t.Errorf("zero path")
	}
	if pathSpec.bounds.Min != minPoint.Round() {
		t.Errorf("pathSpec.bounds.Min = %v, want %v", pathSpec.bounds.Min, minPoint)
	}
	if pathSpec.bounds.Max != maxPoint.Round() {
		t.Errorf("pathSpec.bounds.Max = %v, want %v", pathSpec.bounds.Max, maxPoint)
	}
}
