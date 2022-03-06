// SPDX-License-Identifier: Unlicense OR MIT

package widget

import (
	"image"
	"image/draw"
	"image/gif"
	"io"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// Image is a widget that displays an image.
type Image struct {
	// Src is the image to display.
	Src paint.ImageOp
	// Fit specifies how to scale the image to the constraints.
	// By default it does not do any scaling.
	Fit Fit
	// Position specifies where to position the image within
	// the constraints.
	Position layout.Direction
	// Scale is the ratio of image pixels to
	// dps. If Scale is zero Image falls back to
	// a scale that match a standard 72 DPI.
	Scale float32
}

const (
	defaultScale = float32(160.0 / 72.0)
	defaultDelay = 16 * time.Millisecond
)

func (im Image) Layout(gtx layout.Context) layout.Dimensions {
	scale := im.Scale
	if scale == 0 {
		scale = defaultScale
	}

	size := im.Src.Size()
	wf, hf := float32(size.X), float32(size.Y)
	w, h := gtx.Px(unit.Dp(wf*scale)), gtx.Px(unit.Dp(hf*scale))

	dims, trans := im.Fit.scale(gtx.Constraints, im.Position, layout.Dimensions{Size: image.Pt(w, h)})
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()

	pixelScale := scale * gtx.Metric.PxPerDp
	trans = trans.Mul(f32.Affine2D{}.Scale(f32.Point{}, f32.Pt(pixelScale, pixelScale)))
	defer op.Affine(trans).Push(gtx.Ops).Pop()

	im.Src.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)

	return dims
}

type ImageSequences struct {
	// Fit specifies how to scale the image to the constraints.
	// By default it does not do any scaling.
	Fit Fit
	// Position specifies where to position the image within
	// the constraints.
	Position layout.Direction
	// Frames is the sequence images, each image must be independent
	// from the previous one and only one will be drawed per frame
	Frames []paint.ImageOp
	// FramesEnds control when each frame ends. The FramesEnds must
	// have the lenght of Frames.
	FramesEnds []time.Duration
	// LoopCount controls the number of times an animation will be
	// restarted during display.
	// A LoopCount of 0 means to loop forever.
	// A LoopCount of -1 means to show each frame only once.
	// Otherwise, the animation is looped LoopCount+1 times.
	LoopCount int

	last    time.Time
	elapsed time.Duration
	loops   int
	frame   int
}

// NewGIF returns an ImageSequences from GIF.
// It's expected that the img is a GIF file.
func NewGIF(img io.Reader) (*ImageSequences, error) {
	gifs, err := gif.DecodeAll(img)
	if err != nil {
		return nil, err
	}

	sequence := &ImageSequences{
		Frames:     make([]paint.ImageOp, len(gifs.Image)),
		FramesEnds: make([]time.Duration, len(gifs.Delay)),
		LoopCount:  gifs.LoopCount,
		last:       time.Now(),
	}

	var (
		lastFrame     *image.RGBA
		lastNoDispose *image.RGBA
	)

	for i, img := range gifs.Image {
		merge := image.NewRGBA(gifs.Image[0].Bounds())

		switch gifs.Disposal[i] {
		case gif.DisposalNone:
			// The current frame must draw on top of the the previous frame.
			if lastFrame != nil {
				draw.Draw(merge, merge.Bounds(), lastFrame, merge.Bounds().Min, draw.Src)
			}
			lastNoDispose = merge
		case gif.DisposalPrevious:
			// The current frame must draw over the last gif.NoDisposal.
			if lastNoDispose != nil {
				draw.Draw(merge, merge.Bounds(), lastNoDispose, merge.Bounds().Min, draw.Src)
			}
		case gif.DisposalBackground:
			// The current frame must draw using the background (ignoring any previous frame).
		}

		// This Draw call would also occur inside paint.NewImageOp, so there's no
		// additional cost for DisposalBackground.
		draw.Draw(merge, merge.Bounds(), img, merge.Bounds().Min, draw.Over)
		sequence.Frames[i] = paint.NewImageOp(merge)
		lastFrame = merge
	}

	for i, delay := range gifs.Delay {
		delay := time.Duration(delay * int(time.Second/100))
		if defaultDelay > delay {
			// Most browsers enforces a minimum delay, usually 100ms.
			delay = defaultDelay
		}
		sequence.FramesEnds[i] = delay
		if i > 0 {
			sequence.FramesEnds[i] += sequence.FramesEnds[i-1]
		}
	}

	return sequence, err
}

func (im *ImageSequences) Layout(gtx layout.Context) layout.Dimensions {
	now := gtx.Now
	im.elapsed += now.Sub(im.last)
	im.last = now

	frame := im.frame

	if len(im.FramesEnds) != len(im.Frames) {
		// That is to prevent crash due to misuses, it's documented that both
		// need to have the same lenght. If it doesn't happen, it will apply
		// the default delay (or the first delay) to every frame.
		delay := defaultDelay
		if len(im.FramesEnds) > 0 {
			delay = im.FramesEnds[0]
		}
		im.FramesEnds = make([]time.Duration, len(im.Frames))
		for i := range im.FramesEnds {
			im.FramesEnds[i] = delay
			if i > 0 {
				im.FramesEnds[i] += im.FramesEnds[i-1]
			}
		}
	}

	// Find next frame (maybe skip if necessary)
	for ; frame < len(im.Frames) && im.elapsed > im.FramesEnds[frame]; frame++ {
	}

	if frame == im.frame {
		op.InvalidateOp{At: now.Add(im.FramesEnds[im.frame] - im.elapsed)}.Add(gtx.Ops)
	}

	// If the animation ends, it must reset while the LoopCount
	// is lower than loops.
	if frame >= len(im.Frames) {
		count := im.loops + 1
		if im.LoopCount == 0 || (im.LoopCount+1) > count {
			im.elapsed = 0
			im.loops = count
			frame = 0
		} else {
			frame = im.frame // Reach maximum number of loops, stop on the last frame.
		}
	}

	if frame != im.frame {
		im.frame = frame
		op.InvalidateOp{now.Add(im.FramesEnds[frame] - im.elapsed)}.Add(gtx.Ops)
	}

	return Image{Src: im.Frames[frame], Fit: im.Fit, Position: im.Position}.Layout(gtx)
}
