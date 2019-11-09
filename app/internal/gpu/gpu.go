// SPDX-License-Identifier: Unlicense OR MIT

package gpu

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"math"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"gioui.org/app/internal/gl"
	"gioui.org/f32"
	"gioui.org/internal/opconst"
	"gioui.org/internal/ops"
	"gioui.org/internal/path"
	"gioui.org/op"
	"gioui.org/op/paint"
)

type GPU struct {
	drawing bool
	summary string
	err     error

	pathCache *opCache
	cache     *resourceCache

	frames     chan frame
	results    chan frameResult
	refresh    chan struct{}
	refreshErr chan error
	ack        chan struct{}
	stop       chan struct{}
	stopped    chan struct{}
}

type frame struct {
	collectStats bool
	viewport     image.Point
	ops          *op.Ops
}

type frameResult struct {
	summary string
	err     error
}

type renderer struct {
	ctx           *context
	blitter       *blitter
	pather        *pather
	packer        packer
	intersections packer
}

type drawOps struct {
	reader     ops.Reader
	cache      *resourceCache
	viewport   image.Point
	clearColor [3]float32
	imageOps   []imageOp
	// zimageOps are the rectangle clipped opaque images
	// that can use fast front-to-back rendering with z-test
	// and no blending.
	zimageOps   []imageOp
	pathOps     []*pathOp
	pathOpCache []pathOp
}

type drawState struct {
	clip  f32.Rectangle
	t     op.TransformOp
	cpath *pathOp
	rect  bool
	z     int

	matType materialType
	// Current paint.ImageOp
	image imageOpData
	// Current paint.ColorOp, if any.
	color color.RGBA
}

type pathOp struct {
	off f32.Point
	// clip is the union of all
	// later clip rectangles.
	clip      image.Rectangle
	pathKey   ops.Key
	path      bool
	pathVerts []byte
	parent    *pathOp
	place     placement
}

type imageOp struct {
	z        float32
	path     *pathOp
	off      f32.Point
	clip     image.Rectangle
	material material
	clipType clipType
	place    placement
}

type material struct {
	material materialType
	opaque   bool
	// For materialTypeColor.
	color [4]float32
	// For materialTypeTexture.
	texture  *texture
	uvScale  f32.Point
	uvOffset f32.Point
}

// clipOp is the shadow of clip.Op.
type clipOp struct {
	bounds f32.Rectangle
}

// imageOpData is the shadow of paint.ImageOp.
type imageOpData struct {
	src    *image.RGBA
	handle interface{}
}

func (op *clipOp) decode(data []byte) {
	if opconst.OpType(data[0]) != opconst.TypeClip {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	r := f32.Rectangle{
		Min: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[1:])),
			Y: math.Float32frombits(bo.Uint32(data[5:])),
		},
		Max: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[9:])),
			Y: math.Float32frombits(bo.Uint32(data[13:])),
		},
	}
	*op = clipOp{
		bounds: r,
	}
}

func decodeImageOp(data []byte, refs []interface{}) imageOpData {
	if opconst.OpType(data[0]) != opconst.TypeImage {
		panic("invalid op")
	}
	handle := refs[1]
	if handle == nil {
		panic("nil handle")
	}
	return imageOpData{
		src:    refs[0].(*image.RGBA),
		handle: handle,
	}
}

func decodeColorOp(data []byte) color.RGBA {
	if opconst.OpType(data[0]) != opconst.TypeColor {
		panic("invalid op")
	}
	return color.RGBA{
		R: data[1],
		G: data[2],
		B: data[3],
		A: data[4],
	}
}

func decodePaintOp(data []byte) paint.PaintOp {
	bo := binary.LittleEndian
	if opconst.OpType(data[0]) != opconst.TypePaint {
		panic("invalid op")
	}
	r := f32.Rectangle{
		Min: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[1:])),
			Y: math.Float32frombits(bo.Uint32(data[5:])),
		},
		Max: f32.Point{
			X: math.Float32frombits(bo.Uint32(data[9:])),
			Y: math.Float32frombits(bo.Uint32(data[13:])),
		},
	}
	return paint.PaintOp{
		Rect: r,
	}
}

type clipType uint8

type resource interface {
	release(ctx *context)
}

type texture struct {
	src *image.RGBA
	id  gl.Texture
}

type blitter struct {
	ctx      *context
	viewport image.Point
	prog     [2]gl.Program
	vars     [2]struct {
		z                   gl.Uniform
		uScale, uOffset     gl.Uniform
		uUVScale, uUVOffset gl.Uniform
		uColor              gl.Uniform
	}
	quadVerts gl.Buffer
}

type materialType uint8

const (
	clipTypeNone clipType = iota
	clipTypePath
	clipTypeIntersection
)

const (
	materialTexture materialType = iota
	materialColor
)

var (
	blitAttribs           = []string{"pos", "uv"}
	attribPos   gl.Attrib = 0
	attribUV    gl.Attrib = 1
)

func NewGPU(ctx gl.Context) (*GPU, error) {
	g := &GPU{
		frames:     make(chan frame),
		results:    make(chan frameResult),
		refresh:    make(chan struct{}),
		refreshErr: make(chan error),
		// Ack is buffered so GPU commands can be issued after
		// ack'ing the frame.
		ack:       make(chan struct{}, 1),
		stop:      make(chan struct{}),
		stopped:   make(chan struct{}),
		pathCache: newOpCache(),
		cache:     newResourceCache(),
	}
	if err := g.renderLoop(ctx); err != nil {
		return nil, err
	}
	return g, nil
}

func (g *GPU) renderLoop(glctx gl.Context) error {
	// GL Operations must happen on a single OS thread, so
	// pass initialization result through a channel.
	initErr := make(chan error)
	go func() {
		runtime.LockOSThread()
		// Don't UnlockOSThread to avoid reuse by the Go runtime.
		defer close(g.stopped)
		defer glctx.Release()

		if err := glctx.MakeCurrent(); err != nil {
			initErr <- err
			return
		}
		ctx, err := newContext(glctx)
		if err != nil {
			initErr <- err
			return
		}
		defer g.cache.release(ctx)
		defer g.pathCache.release(ctx)
		r := newRenderer(ctx)
		defer r.release()
		var timers *timers
		var zopsTimer, stencilTimer, coverTimer, cleanupTimer *timer
		initErr <- nil
		var drawOps drawOps
	loop:
		for {
			select {
			case <-g.refresh:
				g.refreshErr <- glctx.MakeCurrent()
			case frame := <-g.frames:
				drawOps.reset(g.cache, frame.viewport)
				drawOps.collect(g.cache, frame.ops, frame.viewport)
				glctx.Lock()
				frameStart := time.Now()
				if frame.collectStats && timers == nil && ctx.caps.EXT_disjoint_timer_query {
					timers = newTimers(ctx)
					zopsTimer = timers.newTimer()
					stencilTimer = timers.newTimer()
					coverTimer = timers.newTimer()
					cleanupTimer = timers.newTimer()
					defer timers.release()
				}
				// Upload path data to GPU before ack'ing the frame
				// ops for re-use.
				for _, p := range drawOps.pathOps {
					if _, exists := g.pathCache.get(p.pathKey); !exists {
						data := buildPath(r.ctx, p.pathVerts)
						g.pathCache.put(p.pathKey, data)
					}
					p.pathVerts = nil
				}
				// Signal that we're done with the frame ops.
				g.ack <- struct{}{}
				r.blitter.viewport = frame.viewport
				r.pather.viewport = frame.viewport
				for _, img := range drawOps.imageOps {
					expandPathOp(img.path, img.clip)
				}
				if frame.collectStats {
					zopsTimer.begin()
				}
				ctx.DepthFunc(gl.GREATER)
				ctx.ClearColor(drawOps.clearColor[0], drawOps.clearColor[1], drawOps.clearColor[2], 1.0)
				ctx.ClearDepthf(0.0)
				ctx.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
				ctx.Viewport(0, 0, frame.viewport.X, frame.viewport.Y)
				r.drawZOps(drawOps.zimageOps)
				zopsTimer.end()
				stencilTimer.begin()
				ctx.Enable(gl.BLEND)
				r.packStencils(&drawOps.pathOps)
				r.stencilClips(g.pathCache, drawOps.pathOps)
				r.packIntersections(drawOps.imageOps)
				r.intersect(drawOps.imageOps)
				stencilTimer.end()
				coverTimer.begin()
				ctx.Viewport(0, 0, frame.viewport.X, frame.viewport.Y)
				r.drawOps(drawOps.imageOps)
				ctx.Disable(gl.BLEND)
				r.pather.stenciler.invalidateFBO()
				coverTimer.end()
				err := glctx.Present()
				cleanupTimer.begin()
				g.cache.frame(ctx)
				g.pathCache.frame(ctx)
				cleanupTimer.end()
				var res frameResult
				if frame.collectStats && timers.ready() {
					zt, st, covt, cleant := zopsTimer.Elapsed, stencilTimer.Elapsed, coverTimer.Elapsed, cleanupTimer.Elapsed
					ft := zt + st + covt + cleant
					q := 100 * time.Microsecond
					zt, st, covt = zt.Round(q), st.Round(q), covt.Round(q)
					frameDur := time.Since(frameStart).Round(q)
					ft = ft.Round(q)
					res.summary = fmt.Sprintf("draw:%7s gpu:%7s zt:%7s st:%7s cov:%7s", frameDur, ft, zt, st, covt)
				}
				res.err = err
				glctx.Unlock()
				g.results <- res
			case <-g.stop:
				break loop
			}
		}
	}()
	return <-initErr
}

func (g *GPU) Release() {
	// Flush error.
	g.Flush()
	close(g.stop)
	<-g.stopped
	g.stop = nil
}

func (g *GPU) Flush() error {
	if g.drawing {
		st := <-g.results
		g.setErr(st.err)
		if st.summary != "" {
			g.summary = st.summary
		}
		g.drawing = false
	}
	return g.err
}

func (g *GPU) Timings() string {
	return g.summary
}

func (g *GPU) Refresh() {
	if g.err != nil {
		return
	}
	// Make sure any pending frame is complete.
	g.Flush()
	g.refresh <- struct{}{}
	g.setErr(<-g.refreshErr)
}

// Draw initiates a draw of a frame. It returns a channel
// than signals when the frame is no longer being accessed.
func (g *GPU) Draw(profile bool, viewport image.Point, frameOps *op.Ops) <-chan struct{} {
	if g.err != nil {
		g.ack <- struct{}{}
		return g.ack
	}
	g.Flush()
	g.frames <- frame{profile, viewport, frameOps}
	g.drawing = true
	return g.ack
}

func (g *GPU) setErr(err error) {
	if g.err == nil {
		g.err = err
	}
}

func (r *renderer) texHandle(t *texture) gl.Texture {
	if t.id.Valid() {
		return t.id
	}
	t.id = createTexture(r.ctx)
	r.ctx.BindTexture(gl.TEXTURE_2D, t.id)
	r.uploadTexture(t.src)
	return t.id
}

func (t *texture) release(ctx *context) {
	if t.id.Valid() {
		ctx.DeleteTexture(t.id)
	}
}

func newRenderer(ctx *context) *renderer {
	r := &renderer{
		ctx:     ctx,
		blitter: newBlitter(ctx),
		pather:  newPather(ctx),
	}
	r.packer.maxDim = ctx.GetInteger(gl.MAX_TEXTURE_SIZE)
	r.intersections.maxDim = r.packer.maxDim
	return r
}

func (r *renderer) release() {
	r.pather.release()
	r.blitter.release()
}

func newBlitter(ctx *context) *blitter {
	prog, err := createColorPrograms(ctx, blitVSrc, blitFSrc)
	if err != nil {
		panic(err)
	}
	quadVerts := ctx.CreateBuffer()
	ctx.BindBuffer(gl.ARRAY_BUFFER, quadVerts)
	ctx.BufferData(gl.ARRAY_BUFFER,
		gl.BytesView([]float32{
			-1, +1, 0, 0,
			+1, +1, 1, 0,
			-1, -1, 0, 1,
			+1, -1, 1, 1,
		}),
		gl.STATIC_DRAW)
	b := &blitter{
		ctx:       ctx,
		prog:      prog,
		quadVerts: quadVerts,
	}
	for i, prog := range prog {
		ctx.UseProgram(prog)
		switch materialType(i) {
		case materialTexture:
			uTex := gl.GetUniformLocation(ctx.Functions, prog, "tex")
			ctx.Uniform1i(uTex, 0)
			b.vars[i].uUVScale = gl.GetUniformLocation(ctx.Functions, prog, "uvScale")
			b.vars[i].uUVOffset = gl.GetUniformLocation(ctx.Functions, prog, "uvOffset")
		case materialColor:
			b.vars[i].uColor = gl.GetUniformLocation(ctx.Functions, prog, "color")
		}
		b.vars[i].z = gl.GetUniformLocation(ctx.Functions, prog, "z")
		b.vars[i].uScale = gl.GetUniformLocation(ctx.Functions, prog, "scale")
		b.vars[i].uOffset = gl.GetUniformLocation(ctx.Functions, prog, "offset")
	}
	return b
}

func (b *blitter) release() {
	b.ctx.DeleteBuffer(b.quadVerts)
	for _, p := range b.prog {
		b.ctx.DeleteProgram(p)
	}
}

func createColorPrograms(ctx *context, vsSrc, fsSrc string) ([2]gl.Program, error) {
	var prog [2]gl.Program
	frep := strings.NewReplacer(
		"HEADER", `
uniform sampler2D tex;
`,
		"GET_COLOR", `texture2D(tex, vUV)`,
	)
	fsSrcTex := frep.Replace(fsSrc)
	var err error
	prog[materialTexture], err = gl.CreateProgram(ctx.Functions, vsSrc, fsSrcTex, blitAttribs)
	if err != nil {
		return prog, err
	}
	frep = strings.NewReplacer(
		"HEADER", `
uniform vec4 color;
`,
		"GET_COLOR", `color`,
	)
	fsSrcCol := frep.Replace(fsSrc)
	prog[materialColor], err = gl.CreateProgram(ctx.Functions, vsSrc, fsSrcCol, blitAttribs)
	if err != nil {
		ctx.DeleteProgram(prog[materialTexture])
		return prog, err
	}
	return prog, nil
}

func (r *renderer) stencilClips(pathCache *opCache, ops []*pathOp) {
	if len(r.packer.sizes) == 0 {
		return
	}
	fbo := -1
	r.pather.begin(r.packer.sizes)
	for _, p := range ops {
		if fbo != p.place.Idx {
			fbo = p.place.Idx
			f := r.pather.stenciler.cover(fbo)
			bindFramebuffer(r.ctx, f.fbo)
			r.ctx.Clear(gl.COLOR_BUFFER_BIT)
		}
		data, _ := pathCache.get(p.pathKey)
		r.pather.stencilPath(p.clip, p.off, p.place.Pos, data.(*pathData))
	}
	r.pather.end()
}

func (r *renderer) intersect(ops []imageOp) {
	if len(r.intersections.sizes) == 0 {
		return
	}
	fbo := -1
	r.pather.stenciler.beginIntersect(r.intersections.sizes)
	r.ctx.BindBuffer(gl.ARRAY_BUFFER, r.blitter.quadVerts)
	r.ctx.VertexAttribPointer(attribPos, 2, gl.FLOAT, false, 4*4, 0)
	r.ctx.VertexAttribPointer(attribUV, 2, gl.FLOAT, false, 4*4, 4*2)
	r.ctx.EnableVertexAttribArray(attribPos)
	r.ctx.EnableVertexAttribArray(attribUV)
	for _, img := range ops {
		if img.clipType != clipTypeIntersection {
			continue
		}
		if fbo != img.place.Idx {
			fbo = img.place.Idx
			f := r.pather.stenciler.intersections.fbos[fbo]
			bindFramebuffer(r.ctx, f.fbo)
			r.ctx.Clear(gl.COLOR_BUFFER_BIT)
		}
		r.ctx.Viewport(img.place.Pos.X, img.place.Pos.Y, img.clip.Dx(), img.clip.Dy())
		r.intersectPath(img.path, img.clip)
	}
	r.ctx.DisableVertexAttribArray(attribPos)
	r.ctx.DisableVertexAttribArray(attribUV)
	r.pather.stenciler.endIntersect()
}

func (r *renderer) intersectPath(p *pathOp, clip image.Rectangle) {
	if p.parent != nil {
		r.intersectPath(p.parent, clip)
	}
	if !p.path {
		return
	}
	o := p.place.Pos.Add(clip.Min).Sub(p.clip.Min)
	uv := image.Rectangle{
		Min: o,
		Max: o.Add(clip.Size()),
	}
	fbo := r.pather.stenciler.cover(p.place.Idx)
	r.ctx.BindTexture(gl.TEXTURE_2D, fbo.tex)
	coverScale, coverOff := texSpaceTransform(toRectF(uv), fbo.size)
	r.ctx.Uniform2f(r.pather.stenciler.uIntersectUVScale, coverScale.X, coverScale.Y)
	r.ctx.Uniform2f(r.pather.stenciler.uIntersectUVOffset, coverOff.X, coverOff.Y)
	r.ctx.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
}

func (r *renderer) packIntersections(ops []imageOp) {
	r.intersections.clear()
	for i, img := range ops {
		var npaths int
		var onePath *pathOp
		for p := img.path; p != nil; p = p.parent {
			if p.path {
				onePath = p
				npaths++
			}
		}
		switch npaths {
		case 0:
		case 1:
			place := onePath.place
			place.Pos = place.Pos.Sub(onePath.clip.Min).Add(img.clip.Min)
			ops[i].place = place
			ops[i].clipType = clipTypePath
		default:
			sz := image.Point{X: img.clip.Dx(), Y: img.clip.Dy()}
			place, ok := r.intersections.add(sz)
			if !ok {
				panic("internal error: if the intersection fit, the intersection should fit as well")
			}
			ops[i].clipType = clipTypeIntersection
			ops[i].place = place
		}
	}
}

func (r *renderer) packStencils(pops *[]*pathOp) {
	r.packer.clear()
	ops := *pops
	// Allocate atlas space for cover textures.
	var i int
	for i < len(ops) {
		p := ops[i]
		if p.clip.Empty() {
			ops[i] = ops[len(ops)-1]
			ops = ops[:len(ops)-1]
			continue
		}
		sz := image.Point{X: p.clip.Dx(), Y: p.clip.Dy()}
		place, ok := r.packer.add(sz)
		if !ok {
			// The clip area is at most the entire screen. Hopefully no
			// screen is larger than GL_MAX_TEXTURE_SIZE.
			panic(fmt.Errorf("clip area %v is larger than maximum texture size %dx%d", p.clip, r.packer.maxDim, r.packer.maxDim))
		}
		p.place = place
		i++
	}
	*pops = ops
}

// intersects intersects clip and b where b is offset by off.
// ceilRect returns a bounding image.Rectangle for a f32.Rectangle.
func boundRectF(r f32.Rectangle) image.Rectangle {
	return image.Rectangle{
		Min: image.Point{
			X: int(floor(r.Min.X)),
			Y: int(floor(r.Min.Y)),
		},
		Max: image.Point{
			X: int(ceil(r.Max.X)),
			Y: int(ceil(r.Max.Y)),
		},
	}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{
			X: float32(r.Min.X),
			Y: float32(r.Min.Y),
		},
		Max: f32.Point{
			X: float32(r.Max.X),
			Y: float32(r.Max.Y),
		},
	}
}

func ceil(v float32) int {
	return int(math.Ceil(float64(v)))
}

func floor(v float32) int {
	return int(math.Floor(float64(v)))
}

func (d *drawOps) reset(cache *resourceCache, viewport image.Point) {
	d.clearColor = [3]float32{1.0, 1.0, 1.0}
	d.cache = cache
	d.viewport = viewport
	d.imageOps = d.imageOps[:0]
	d.zimageOps = d.zimageOps[:0]
	d.pathOps = d.pathOps[:0]
	d.pathOpCache = d.pathOpCache[:0]
}

func (d *drawOps) collect(cache *resourceCache, root *op.Ops, viewport image.Point) {
	d.reset(cache, viewport)
	clip := f32.Rectangle{
		Max: f32.Point{X: float32(viewport.X), Y: float32(viewport.Y)},
	}
	d.reader.Reset(root)
	state := drawState{
		clip:  clip,
		rect:  true,
		color: color.RGBA{A: 0xff},
	}
	d.collectOps(&d.reader, state)
}

func (d *drawOps) newPathOp() *pathOp {
	d.pathOpCache = append(d.pathOpCache, pathOp{})
	return &d.pathOpCache[len(d.pathOpCache)-1]
}

func (d *drawOps) collectOps(r *ops.Reader, state drawState) int {
	var aux []byte
	var auxKey ops.Key
loop:
	for encOp, ok := r.Decode(); ok; encOp, ok = r.Decode() {
		switch opconst.OpType(encOp.Data[0]) {
		case opconst.TypeTransform:
			dop := ops.DecodeTransformOp(encOp.Data)
			state.t = state.t.Multiply(op.TransformOp(dop))
		case opconst.TypeAux:
			aux = encOp.Data[opconst.TypeAuxLen:]
			// The first data byte stores whether the MaxY
			// fields have been initialized.
			maxyFilled := aux[0] == 1
			aux[0] = 1
			aux = aux[1:]
			if !maxyFilled {
				fillMaxY(aux)
			}
			auxKey = encOp.Key
		case opconst.TypeClip:
			var op clipOp
			op.decode(encOp.Data)
			off := state.t.Transform(f32.Point{})
			state.clip = state.clip.Intersect(op.bounds.Add(off))
			if state.clip.Empty() {
				continue
			}
			npath := d.newPathOp()
			*npath = pathOp{
				parent: state.cpath,
				off:    off,
			}
			state.cpath = npath
			if len(aux) > 0 {
				state.rect = false
				state.cpath.pathKey = auxKey
				state.cpath.path = true
				state.cpath.pathVerts = aux
				d.pathOps = append(d.pathOps, state.cpath)
			}
			aux = nil
			auxKey = ops.Key{}
		case opconst.TypeColor:
			state.matType = materialColor
			state.color = decodeColorOp(encOp.Data)
		case opconst.TypeImage:
			state.matType = materialTexture
			state.image = decodeImageOp(encOp.Data, encOp.Refs)
		case opconst.TypePaint:
			op := decodePaintOp(encOp.Data)
			off := state.t.Transform(f32.Point{})
			clip := state.clip.Intersect(op.Rect.Add(off))
			if clip.Empty() {
				continue
			}
			bounds := boundRectF(clip)
			mat := state.materialFor(d.cache, op.Rect, off, bounds)
			if bounds.Min == (image.Point{}) && bounds.Max == d.viewport && state.rect && mat.opaque && mat.material == materialColor {
				// The image is a uniform opaque color and takes up the whole screen.
				// Scrap images up to and including this image and set clear color.
				d.zimageOps = d.zimageOps[:0]
				d.imageOps = d.imageOps[:0]
				state.z = 0
				copy(d.clearColor[:], mat.color[:3])
				continue
			}
			state.z++
			// Assume 16-bit depth buffer.
			const zdepth = 1 << 16
			// Convert z to window-space, assuming depth range [0;1].
			zf := float32(state.z)*2/zdepth - 1.0
			img := imageOp{
				z:        zf,
				path:     state.cpath,
				off:      off,
				clip:     bounds,
				material: mat,
			}
			if state.rect && img.material.opaque {
				d.zimageOps = append(d.zimageOps, img)
			} else {
				d.imageOps = append(d.imageOps, img)
			}
		case opconst.TypePush:
			state.z = d.collectOps(r, state)
		case opconst.TypePop:
			break loop
		}
	}
	return state.z
}

func expandPathOp(p *pathOp, clip image.Rectangle) {
	for p != nil {
		pclip := p.clip
		if !pclip.Empty() {
			clip = clip.Union(pclip)
		}
		p.clip = clip
		p = p.parent
	}
}

func (d *drawState) materialFor(cache *resourceCache, rect f32.Rectangle, off f32.Point, clip image.Rectangle) material {
	var m material
	switch d.matType {
	case materialColor:
		m.material = materialColor
		m.color = gamma(d.color.RGBA())
		m.opaque = m.color[3] == 1.0
	case materialTexture:
		m.material = materialTexture
		dr := boundRectF(rect.Add(off))
		sz := d.image.src.Bounds().Size()
		sr := f32.Rectangle{
			Max: f32.Point{
				X: float32(sz.X),
				Y: float32(sz.Y),
			},
		}
		if dx := float32(dr.Dx()); dx != 0 {
			// Don't clip 1 px width sources.
			if sdx := sr.Dx(); sdx > 1 {
				sr.Min.X += (float32(clip.Min.X-dr.Min.X)*sdx + dx/2) / dx
				sr.Max.X -= (float32(dr.Max.X-clip.Max.X)*sdx + dx/2) / dx
			}
		}
		if dy := float32(dr.Dy()); dy != 0 {
			// Don't clip 1 px height sources.
			if sdy := sr.Dy(); sdy > 1 {
				sr.Min.Y += (float32(clip.Min.Y-dr.Min.Y)*sdy + dy/2) / dy
				sr.Max.Y -= (float32(dr.Max.Y-clip.Max.Y)*sdy + dy/2) / dy
			}
		}
		tex, exists := cache.get(d.image.handle)
		if !exists {
			t := &texture{
				src: d.image.src,
			}
			cache.put(d.image.handle, t)
			tex = t
		}
		m.texture = tex.(*texture)
		m.uvScale, m.uvOffset = texSpaceTransform(sr, sz)
	}
	return m
}

func (r *renderer) drawZOps(ops []imageOp) {
	r.ctx.Enable(gl.DEPTH_TEST)
	r.ctx.BindBuffer(gl.ARRAY_BUFFER, r.blitter.quadVerts)
	r.ctx.VertexAttribPointer(attribPos, 2, gl.FLOAT, false, 4*4, 0)
	r.ctx.VertexAttribPointer(attribUV, 2, gl.FLOAT, false, 4*4, 4*2)
	r.ctx.EnableVertexAttribArray(attribPos)
	r.ctx.EnableVertexAttribArray(attribUV)
	// Render front to back.
	for i := len(ops) - 1; i >= 0; i-- {
		img := ops[i]
		m := img.material
		switch m.material {
		case materialTexture:
			r.ctx.BindTexture(gl.TEXTURE_2D, r.texHandle(m.texture))
		}
		drc := img.clip
		scale, off := clipSpaceTransform(drc, r.blitter.viewport)
		r.blitter.blit(img.z, m.material, m.color, scale, off, m.uvScale, m.uvOffset)
	}
	r.ctx.DisableVertexAttribArray(attribPos)
	r.ctx.DisableVertexAttribArray(attribUV)
	r.ctx.Disable(gl.DEPTH_TEST)
}

func (r *renderer) drawOps(ops []imageOp) {
	r.ctx.Enable(gl.DEPTH_TEST)
	r.ctx.DepthMask(false)
	r.ctx.BlendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA)
	r.ctx.BindBuffer(gl.ARRAY_BUFFER, r.blitter.quadVerts)
	r.ctx.VertexAttribPointer(attribPos, 2, gl.FLOAT, false, 4*4, 0)
	r.ctx.VertexAttribPointer(attribUV, 2, gl.FLOAT, false, 4*4, 4*2)
	r.ctx.EnableVertexAttribArray(attribPos)
	r.ctx.EnableVertexAttribArray(attribUV)
	var coverTex gl.Texture
	for _, img := range ops {
		m := img.material
		switch m.material {
		case materialTexture:
			r.ctx.BindTexture(gl.TEXTURE_2D, r.texHandle(m.texture))
		}
		drc := img.clip
		scale, off := clipSpaceTransform(drc, r.blitter.viewport)
		var fbo stencilFBO
		switch img.clipType {
		case clipTypeNone:
			r.blitter.blit(img.z, m.material, m.color, scale, off, m.uvScale, m.uvOffset)
			continue
		case clipTypePath:
			fbo = r.pather.stenciler.cover(img.place.Idx)
		case clipTypeIntersection:
			fbo = r.pather.stenciler.intersections.fbos[img.place.Idx]
		}
		if !coverTex.Equal(fbo.tex) {
			coverTex = fbo.tex
			r.ctx.ActiveTexture(gl.TEXTURE1)
			r.ctx.BindTexture(gl.TEXTURE_2D, coverTex)
			r.ctx.ActiveTexture(gl.TEXTURE0)
		}
		uv := image.Rectangle{
			Min: img.place.Pos,
			Max: img.place.Pos.Add(drc.Size()),
		}
		coverScale, coverOff := texSpaceTransform(toRectF(uv), fbo.size)
		r.pather.cover(img.z, m.material, m.color, scale, off, m.uvScale, m.uvOffset, coverScale, coverOff)
	}
	r.ctx.DisableVertexAttribArray(attribPos)
	r.ctx.DisableVertexAttribArray(attribUV)
	r.ctx.DepthMask(true)
	r.ctx.Disable(gl.DEPTH_TEST)
}

func (r *renderer) uploadTexture(img *image.RGBA) {
	var pixels []byte
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if img.Stride != w*4 {
		panic("unsupported stride")
	}
	start := (b.Min.X + b.Min.Y*w) * 4
	end := (b.Max.X + (b.Max.Y-1)*w) * 4
	pixels = img.Pix[start:end]
	tt := r.ctx.caps.srgbaTriple
	r.ctx.TexImage2D(gl.TEXTURE_2D, 0, tt.internalFormat, w, h, tt.format, tt.typ, pixels)
}

func gamma(r, g, b, a uint32) [4]float32 {
	color := [4]float32{float32(r) / 0xffff, float32(g) / 0xffff, float32(b) / 0xffff, float32(a) / 0xffff}
	// Assume that image.Uniform colors are in sRGB space. Linearize.
	for i := 0; i <= 2; i++ {
		c := color[i]
		// Use the formula from EXT_sRGB.
		if c <= 0.04045 {
			c = c / 12.92
		} else {
			c = float32(math.Pow(float64((c+0.055)/1.055), 2.4))
		}
		color[i] = c
	}
	return color
}

func (b *blitter) blit(z float32, mat materialType, col [4]float32, scale, off, uvScale, uvOff f32.Point) {
	b.ctx.UseProgram(b.prog[mat])
	switch mat {
	case materialColor:
		b.ctx.Uniform4f(b.vars[mat].uColor, col[0], col[1], col[2], col[3])
	case materialTexture:
		b.ctx.Uniform2f(b.vars[mat].uUVScale, uvScale.X, uvScale.Y)
		b.ctx.Uniform2f(b.vars[mat].uUVOffset, uvOff.X, uvOff.Y)
	}
	b.ctx.Uniform1f(b.vars[mat].z, z)
	b.ctx.Uniform2f(b.vars[mat].uScale, scale.X, scale.Y)
	b.ctx.Uniform2f(b.vars[mat].uOffset, off.X, off.Y)
	b.ctx.DrawArrays(gl.TRIANGLE_STRIP, 0, 4)
}

// texSpaceTransform return the scale and offset that transforms the given subimage
// into quad texture coordinates.
func texSpaceTransform(r f32.Rectangle, bounds image.Point) (f32.Point, f32.Point) {
	size := f32.Point{X: float32(bounds.X), Y: float32(bounds.Y)}
	scale := f32.Point{X: r.Dx() / size.X, Y: r.Dy() / size.Y}
	offset := f32.Point{X: r.Min.X / size.X, Y: r.Min.Y / size.Y}
	return scale, offset
}

// clipSpaceTransform returns the scale and offset that transforms the given
// rectangle from a viewport into OpenGL clip space.
func clipSpaceTransform(r image.Rectangle, viewport image.Point) (f32.Point, f32.Point) {
	// First, transform UI coordinates to OpenGL coordinates:
	//
	//	[(-1, +1) (+1, +1)]
	//	[(-1, -1) (+1, -1)]
	//
	x, y := float32(r.Min.X), float32(r.Min.Y)
	w, h := float32(r.Dx()), float32(r.Dy())
	vx, vy := 2/float32(viewport.X), 2/float32(viewport.Y)
	x = x*vx - 1
	y = 1 - y*vy
	w *= vx
	h *= vy

	// Then, compute the transformation from the fullscreen quad to
	// the rectangle at (x, y) and dimensions (w, h).
	scale := f32.Point{X: w * .5, Y: h * .5}
	offset := f32.Point{X: x + w*.5, Y: y - h*.5}
	return scale, offset
}

func bindFramebuffer(ctx *context, fbo gl.Framebuffer) {
	ctx.BindFramebuffer(gl.FRAMEBUFFER, fbo)
	if st := ctx.CheckFramebufferStatus(gl.FRAMEBUFFER); st != gl.FRAMEBUFFER_COMPLETE {
		panic(fmt.Errorf("AA FBO not complete; status = 0x%x, err = %d", st, ctx.GetError()))
	}
}

func createTexture(ctx *context) gl.Texture {
	tex := ctx.CreateTexture()
	ctx.BindTexture(gl.TEXTURE_2D, tex)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	ctx.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	return tex
}

// Fill in maximal Y coordinates of the NW and NE corners.
func fillMaxY(verts []byte) {
	contour := 0
	bo := binary.LittleEndian
	for len(verts) > 0 {
		maxy := float32(math.Inf(-1))
		i := 0
		for ; i+path.VertStride*4 <= len(verts); i += path.VertStride * 4 {
			vert := verts[i : i+path.VertStride]
			// MaxY contains the integer contour index.
			pathContour := int(bo.Uint32(vert[int(unsafe.Offsetof(((*path.Vertex)(nil)).MaxY)):]))
			if contour != pathContour {
				contour = pathContour
				break
			}
			fromy := math.Float32frombits(bo.Uint32(vert[int(unsafe.Offsetof(((*path.Vertex)(nil)).FromY)):]))
			ctrly := math.Float32frombits(bo.Uint32(vert[int(unsafe.Offsetof(((*path.Vertex)(nil)).CtrlY)):]))
			toy := math.Float32frombits(bo.Uint32(vert[int(unsafe.Offsetof(((*path.Vertex)(nil)).ToY)):]))
			if fromy > maxy {
				maxy = fromy
			}
			if ctrly > maxy {
				maxy = ctrly
			}
			if toy > maxy {
				maxy = toy
			}
		}
		fillContourMaxY(maxy, verts[:i])
		verts = verts[i:]
	}
}

func fillContourMaxY(maxy float32, verts []byte) {
	bo := binary.LittleEndian
	for i := 0; i < len(verts); i += path.VertStride {
		off := int(unsafe.Offsetof(((*path.Vertex)(nil)).MaxY))
		bo.PutUint32(verts[i+off:], math.Float32bits(maxy))
	}
}

const blitVSrc = `
#version 100

precision highp float;

uniform float z;
uniform vec2 scale;
uniform vec2 offset;

attribute vec2 pos;

attribute vec2 uv;
uniform vec2 uvScale;
uniform vec2 uvOffset;

varying vec2 vUV;

void main() {
	vec2 p = pos;
	p *= scale;
	p += offset;
	gl_Position = vec4(p, z, 1);
	vUV = uv*uvScale + uvOffset;
}
`

const blitFSrc = `
#version 100

precision mediump float;

varying vec2 vUV;

HEADER

void main() {
	gl_FragColor = GET_COLOR;
}
`
