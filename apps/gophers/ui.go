// SPDX-License-Identifier: Unlicense OR MIT

package main

// A Gio program that displays Go contributors from GitHub. See https://gioui.org for more information.

import (
	"fmt"
	"image"
	"image/color"
	"runtime"

	"golang.org/x/image/draw"

	_ "image/jpeg"
	_ "image/png"

	_ "net/http/pprof"

	"gioui.org/ui"
	"gioui.org/ui/f32"
	"gioui.org/ui/gesture"
	"gioui.org/ui/input"
	"gioui.org/ui/key"
	"gioui.org/ui/layout"
	"gioui.org/ui/measure"
	"gioui.org/ui/paint"
	"gioui.org/ui/pointer"
	"gioui.org/ui/system"
	"gioui.org/ui/text"
	"gioui.org/ui/widget"
	"golang.org/x/exp/shiny/iconvg"

	"github.com/google/go-github/v24/github"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"

	"golang.org/x/exp/shiny/materialdesign/icons"
)

type UI struct {
	faces        measure.Faces
	fab          *ActionButton
	usersList    *layout.List
	users        []*user
	userClicks   []gesture.Click
	selectedUser *userPage
	edit, edit2  *text.Editor
	fetchCommits func(u string)

	// Profiling.
	profiling   bool
	profile     system.ProfileEvent
	lastMallocs uint64
}

type userPage struct {
	faces       measure.Faces
	user        *user
	commitsList *layout.List
	commits     []*github.Commit
}

type user struct {
	name    string
	login   string
	company string
	avatar  image.Image
}

type icon struct {
	src  []byte
	size ui.Value

	// Cached values.
	img     image.Image
	imgSize int
}

type ActionButton struct {
	face    text.Face
	Open    bool
	icons   []*icon
	sendIco *icon
}

var fonts struct {
	regular *sfnt.Font
	bold    *sfnt.Font
	italic  *sfnt.Font
	mono    *sfnt.Font
}

var theme struct {
	text     ui.MacroOp
	tertText ui.MacroOp
	brand    ui.MacroOp
	white    ui.MacroOp
}

func colorMaterial(ops *ui.Ops, color color.RGBA) ui.MacroOp {
	var mat ui.MacroOp
	mat.Record(ops)
	paint.ColorOp{Color: color}.Add(ops)
	mat.Stop()
	return mat
}

func init() {
	fonts.regular = mustLoadFont(goregular.TTF)
	fonts.bold = mustLoadFont(gobold.TTF)
	fonts.italic = mustLoadFont(goitalic.TTF)
	fonts.mono = mustLoadFont(gomono.TTF)
	var ops ui.Ops
	theme.text = colorMaterial(&ops, rgb(0x333333))
	theme.tertText = colorMaterial(&ops, rgb(0xbbbbbb))
	theme.brand = colorMaterial(&ops, rgb(0x62798c))
	theme.white = colorMaterial(&ops, rgb(0xffffff))
}

func newUI(fetchCommits func(string)) *UI {
	u := &UI{
		fetchCommits: fetchCommits,
	}
	u.usersList = &layout.List{
		Axis: layout.Vertical,
	}
	u.fab = &ActionButton{
		face:    u.face(fonts.regular, 11),
		sendIco: &icon{src: icons.ContentSend, size: ui.Dp(24)},
		icons:   []*icon{},
	}
	u.edit2 = &text.Editor{
		Face: u.face(fonts.italic, 14),
		//Alignment: text.End,
		SingleLine:   true,
		Hint:         "Hint",
		HintMaterial: theme.tertText,
		Material:     theme.text,
	}
	u.edit2.SetText("Single line editor. Edit me!")
	u.edit = &text.Editor{
		Face:     u.face(fonts.regular, 16),
		Material: theme.text,
		//Alignment: text.End,
		//SingleLine: true,
	}
	u.edit.SetText(longTextSample)
	return u
}

func mustLoadFont(fontData []byte) *sfnt.Font {
	fnt, err := sfnt.Parse(fontData)
	if err != nil {
		panic("failed to load font")
	}
	return fnt
}

func rgb(c uint32) color.RGBA {
	return argb((0xff << 24) | c)
}

func argb(c uint32) color.RGBA {
	return color.RGBA{A: uint8(c >> 24), R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c)}
}

func (u *UI) face(f *sfnt.Font, size float32) text.Face {
	return u.faces.For(f, ui.Sp(size))
}

func (u *UI) layoutTimings(c ui.Config, q input.Queue, ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	if !u.profiling {
		return layout.Dimensions{}
	}
	for e, ok := q.Next(u); ok; e, ok = q.Next(u) {
		if e, ok := e.(system.ProfileEvent); ok {
			u.profile = e
		}
	}
	system.ProfileOp{Key: u}.Add(ops)
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)
	mallocs := mstats.Mallocs - u.lastMallocs
	u.lastMallocs = mstats.Mallocs
	al := layout.Align{Alignment: layout.NE}
	return al.Layout(ops, cs, func(cs layout.Constraints) layout.Dimensions {
		in := layout.Inset{Top: ui.Dp(16)}
		return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
			txt := fmt.Sprintf("m: %d %s", mallocs, u.profile.Timings)
			return text.Label{Material: theme.text, Face: u.face(fonts.mono, 10), Text: txt}.Layout(ops, cs)
		})
	})
}

func (u *UI) Layout(c ui.Config, q input.Queue, ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	u.faces.Reset(c)
	for i := range u.userClicks {
		click := &u.userClicks[i]
		for e, ok := click.Next(q); ok; e, ok = click.Next(q) {
			if e.Type == gesture.TypeClick {
				u.selectedUser = u.newUserPage(u.users[i])
			}
		}
	}
	var dims layout.Dimensions
	if u.selectedUser == nil {
		dims = u.layoutUsers(c, q, ops, cs)
	} else {
		dims = u.selectedUser.Layout(c, q, ops, cs)
	}
	u.layoutTimings(c, q, ops, cs)
	return dims
}

func (u *UI) newUserPage(user *user) *userPage {
	up := &userPage{
		faces:       u.faces,
		user:        user,
		commitsList: &layout.List{Axis: layout.Vertical},
	}
	u.fetchCommits(user.login)
	return up
}

func (up *userPage) Layout(c ui.Config, q input.Queue, ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	l := up.commitsList
	if l.Dragging() {
		key.HideInputOp{}.Add(ops)
	}
	return l.Layout(c, q, ops, cs, len(up.commits), func(cs layout.Constraints, i int) layout.Dimensions {
		return up.commit(c, ops, cs, i)
	})
}

func (up *userPage) commit(c ui.Config, ops *ui.Ops, cs layout.Constraints, index int) layout.Dimensions {
	u := up.user
	msg := up.commits[index].GetMessage()
	label := text.Label{Material: theme.text, Face: up.faces.For(fonts.regular, ui.Sp(12)), Text: msg}
	in := layout.Inset{Top: ui.Dp(16), Right: ui.Dp(8), Left: ui.Dp(8)}
	return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
		f := (&layout.Flex{Axis: layout.Horizontal}).Init(ops, cs)
		c1 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
			sz := c.Px(ui.Dp(48))
			cc := clipCircle{}
			return cc.Layout(ops, cs, func(cs layout.Constraints) layout.Dimensions {
				cs = layout.RigidConstraints(cs.Constrain(image.Point{X: sz, Y: sz}))
				return widget.Image{Src: u.avatar, Rect: u.avatar.Bounds()}.Layout(c, ops, cs)
			})
		})
		c2 := f.Flexible(1, func(cs layout.Constraints) layout.Dimensions {
			cs.Width.Min = cs.Width.Max
			in := layout.Inset{Left: ui.Dp(8)}
			return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
				return label.Layout(ops, cs)
			})
		})
		return f.Layout(c1, c2)
	})
}

func (u *UI) layoutUsers(c ui.Config, q input.Queue, ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	st := (&layout.Stack{}).Init(ops, cs)
	c2 := st.Rigid(func(cs layout.Constraints) layout.Dimensions {
		al := layout.Align{Alignment: layout.SE}
		return al.Layout(ops, cs, func(cs layout.Constraints) layout.Dimensions {
			in := layout.UniformInset(ui.Dp(16))
			return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
				return u.fab.Layout(c, q, ops, cs)
			})
		})
	})

	c1 := st.Expand(func(cs layout.Constraints) layout.Dimensions {
		f := (&layout.Flex{Axis: layout.Vertical}).Init(ops, cs)

		c1 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
			cs.Width.Min = cs.Width.Max
			in := layout.UniformInset(ui.Dp(16))
			return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
				sz := c.Px(ui.Dp(200))
				cs = layout.RigidConstraints(cs.Constrain(image.Point{X: sz, Y: sz}))
				return u.edit.Layout(c, q, ops, cs)
			})
		})

		c2 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
			cs.Width.Min = cs.Width.Max
			in := layout.Inset{Bottom: ui.Dp(16), Left: ui.Dp(16), Right: ui.Dp(16)}
			return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
				return u.edit2.Layout(c, q, ops, cs)
			})
		})

		c3 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
			cs.Width.Min = cs.Width.Max
			s := layout.Stack{Alignment: layout.Center}
			s.Init(ops, cs)
			c2 := s.Rigid(func(cs layout.Constraints) layout.Dimensions {
				grey := colorMaterial(ops, rgb(0x888888))
				in := layout.Inset{Top: ui.Dp(16), Right: ui.Dp(8), Bottom: ui.Dp(8), Left: ui.Dp(8)}
				return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
					lbl := text.Label{Material: grey, Face: u.face(fonts.regular, 11), Text: "GOPHERS"}
					return lbl.Layout(ops, cs)
				})
			})
			c1 := s.Expand(func(cs layout.Constraints) layout.Dimensions {
				return fill{colorMaterial(ops, rgb(0xf2f2f2))}.Layout(ops, cs)
			})
			return s.Layout(c1, c2)
		})

		c4 := f.Flexible(1, func(cs layout.Constraints) layout.Dimensions {
			cs.Width.Min = cs.Width.Max
			return u.layoutContributors(c, q, ops, cs)
		})
		return f.Layout(c1, c2, c3, c4)
	})
	return st.Layout(c1, c2)
}

func (a *ActionButton) Layout(c ui.Config, q input.Queue, ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	f := layout.Flex{Axis: layout.Vertical, Alignment: layout.End}
	f.Init(ops, cs)
	return f.Layout(f.Rigid(func(cs layout.Constraints) layout.Dimensions {
		in := layout.Inset{Top: ui.Dp(4)}
		return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
			dims := fab(ops, a.sendIco.image(c), theme.brand, c.Px(ui.Dp(56)))
			pointer.EllipseAreaOp{Rect: image.Rectangle{Max: dims.Size}}.Add(ops)
			return dims
		})
	}))
}

func (u *UI) layoutContributors(c ui.Config, q input.Queue, ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	l := u.usersList
	if l.Dragging() {
		key.HideInputOp{}.Add(ops)
	}
	return l.Layout(c, q, ops, cs, len(u.users), func(cs layout.Constraints, i int) layout.Dimensions {
		return u.user(c, ops, cs, i)
	})
}

func (u *UI) user(c ui.Config, ops *ui.Ops, cs layout.Constraints, index int) layout.Dimensions {
	user := u.users[index]
	elem := layout.Flex{Axis: layout.Vertical}
	elem.Init(ops, cs)
	c1 := elem.Rigid(func(cs layout.Constraints) layout.Dimensions {
		in := layout.UniformInset(ui.Dp(8))
		dims := in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
			f := centerRowOpts()
			f.Init(ops, cs)
			c1 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
				in := layout.Inset{Right: ui.Dp(8)}
				cc := clipCircle{}
				return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
					return cc.Layout(ops, cs, func(cs layout.Constraints) layout.Dimensions {
						sz := image.Point{X: c.Px(ui.Dp(48)), Y: c.Px(ui.Dp(48))}
						cs = layout.RigidConstraints(cs.Constrain(sz))
						return widget.Image{Src: user.avatar, Rect: user.avatar.Bounds()}.Layout(c, ops, cs)
					})
				})
			})
			c2 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
				f := column()
				f.Init(ops, cs)
				c1 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
					f := baseline()
					f.Init(ops, cs)
					c1 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
						return text.Label{Material: theme.text, Face: u.face(fonts.regular, 13), Text: user.name}.Layout(ops, cs)
					})
					c2 := f.Flexible(1, func(cs layout.Constraints) layout.Dimensions {
						cs.Width.Min = cs.Width.Max
						al := layout.Align{Alignment: layout.E}
						return al.Layout(ops, cs, func(cs layout.Constraints) layout.Dimensions {
							in := layout.Inset{Left: ui.Dp(2)}
							return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
								lbl := text.Label{Material: theme.text, Face: u.face(fonts.regular, 10), Text: "3 hours ago"}
								return lbl.Layout(ops, cs)
							})
						})
					})
					return f.Layout(c1, c2)
				})
				c2 := f.Rigid(func(cs layout.Constraints) layout.Dimensions {
					in := layout.Inset{Top: ui.Dp(4)}
					return in.Layout(c, ops, cs, func(cs layout.Constraints) layout.Dimensions {
						return text.Label{Material: theme.tertText, Face: u.face(fonts.regular, 12), Text: user.company}.Layout(ops, cs)
					})
				})
				return f.Layout(c1, c2)
			})
			return f.Layout(c1, c2)
		})
		pointer.RectAreaOp{Rect: image.Rectangle{Max: dims.Size}}.Add(ops)
		click := &u.userClicks[index]
		click.Add(ops)
		return dims
	})
	return elem.Layout(c1)
}

type fill struct {
	material ui.MacroOp
}

func (f fill) Layout(ops *ui.Ops, cs layout.Constraints) layout.Dimensions {
	d := image.Point{X: cs.Width.Max, Y: cs.Height.Max}
	dr := f32.Rectangle{
		Max: f32.Point{X: float32(d.X), Y: float32(d.Y)},
	}
	f.material.Add(ops)
	paint.PaintOp{Rect: dr}.Add(ops)
	return layout.Dimensions{Size: d, Baseline: d.Y}
}

func column() layout.Flex {
	return layout.Flex{Axis: layout.Vertical}
}

func centerRowOpts() layout.Flex {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
}

func baseline() layout.Flex {
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}
}

type clipCircle struct {
}

func (c *clipCircle) Layout(ops *ui.Ops, cs layout.Constraints, w layout.Widget) layout.Dimensions {
	var m ui.MacroOp
	m.Record(ops)
	dims := w(cs)
	m.Stop()
	max := dims.Size.X
	if dy := dims.Size.Y; dy > max {
		max = dy
	}
	szf := float32(max)
	rr := szf * .5
	var stack ui.StackOp
	stack.Push(ops)
	rrect(ops, szf, szf, rr, rr, rr, rr)
	m.Add(ops)
	stack.Pop()
	return dims
}

func fab(ops *ui.Ops, ico image.Image, mat ui.MacroOp, size int) layout.Dimensions {
	dp := image.Point{X: (size - ico.Bounds().Dx()) / 2, Y: (size - ico.Bounds().Dy()) / 2}
	dims := image.Point{X: size, Y: size}
	rr := float32(size) * .5
	rrect(ops, float32(size), float32(size), rr, rr, rr, rr)
	mat.Add(ops)
	paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{X: float32(size), Y: float32(size)}}}.Add(ops)
	paint.ImageOp{Src: ico, Rect: ico.Bounds()}.Add(ops)
	paint.PaintOp{
		Rect: toRectF(ico.Bounds().Add(dp)),
	}.Add(ops)
	return layout.Dimensions{Size: dims}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: float32(r.Min.X), Y: float32(r.Min.Y)},
		Max: f32.Point{X: float32(r.Max.X), Y: float32(r.Max.Y)},
	}
}

func (ic *icon) image(cfg ui.Config) image.Image {
	sz := cfg.Px(ic.size)
	if sz == ic.imgSize {
		return ic.img
	}
	m, _ := iconvg.DecodeMetadata(ic.src)
	dx, dy := m.ViewBox.AspectRatio()
	img := image.NewRGBA(image.Rectangle{Max: image.Point{X: sz, Y: int(float32(sz) * dy / dx)}})
	var ico iconvg.Rasterizer
	ico.SetDstImage(img, img.Bounds(), draw.Src)
	// Use white for icons.
	m.Palette[0] = color.RGBA{A: 0xff, R: 0xff, G: 0xff, B: 0xff}
	iconvg.Decode(&ico, ic.src, &iconvg.DecodeOptions{
		Palette: &m.Palette,
	})
	ic.img = img
	ic.imgSize = sz
	return img
}

// https://pomax.github.io/bezierinfo/#circles_cubic.
func rrect(ops *ui.Ops, width, height, se, sw, nw, ne float32) {
	w, h := float32(width), float32(height)
	const c = 0.55228475 // 4*(sqrt(2)-1)/3
	var b paint.PathBuilder
	b.Init(ops)
	b.Move(f32.Point{X: w, Y: h - se})
	b.Cube(f32.Point{X: 0, Y: se * c}, f32.Point{X: -se + se*c, Y: se}, f32.Point{X: -se, Y: se}) // SE
	b.Line(f32.Point{X: sw - w + se, Y: 0})
	b.Cube(f32.Point{X: -sw * c, Y: 0}, f32.Point{X: -sw, Y: -sw + sw*c}, f32.Point{X: -sw, Y: -sw}) // SW
	b.Line(f32.Point{X: 0, Y: nw - h + sw})
	b.Cube(f32.Point{X: 0, Y: -nw * c}, f32.Point{X: nw - nw*c, Y: -nw}, f32.Point{X: nw, Y: -nw}) // NW
	b.Line(f32.Point{X: w - ne - nw, Y: 0})
	b.Cube(f32.Point{X: ne * c, Y: 0}, f32.Point{X: ne, Y: ne - ne*c}, f32.Point{X: ne, Y: ne}) // NE
	b.End()
}

const longTextSample = `1. I learned from my grandfather, Verus, to use good manners, and to
put restraint on anger. 2. In the famous memory of my father I had a
pattern of modesty and manliness. 3. Of my mother I learned to be
pious and generous; to keep myself not only from evil deeds, but even
from evil thoughts; and to live with a simplicity which is far from
customary among the rich. 4. I owe it to my great-grandfather that I
did not attend public lectures and discussions, but had good and able
teachers at home; and I owe him also the knowledge that for things of
this nature a man should count no expense too great.

5. My tutor taught me not to favour either green or blue at the
chariot races, nor, in the contests of gladiators, to be a supporter
either of light or heavy armed. He taught me also to endure labour;
not to need many things; to serve myself without troubling others; not
to intermeddle in the affairs of others, and not easily to listen to
slanders against them.

6. Of Diognetus I had the lesson not to busy myself about vain things;
not to credit the great professions of such as pretend to work
wonders, or of sorcerers about their charms, and their expelling of
Demons and the like; not to keep quails (for fighting or divination),
nor to run after such things; to suffer freedom of speech in others,
and to apply myself heartily to philosophy. Him also I must thank for
my hearing first Bacchius, then Tandasis and Marcianus; that I wrote
dialogues in my youth, and took a liking to the philosopher's pallet
and skins, and to the other things which, by the Grecian discipline,
belong to that profession.

7. To Rusticus I owe my first apprehensions that my nature needed
reform and cure; and that I did not fall into the ambition of the
common Sophists, either by composing speculative writings or by
declaiming harangues of exhortation in public; further, that I never
strove to be admired by ostentation of great patience in an ascetic
life, or by display of activity and application; that I gave over the
study of rhetoric, poetry, and the graces of language; and that I did
not pace my house in my senatorial robes, or practise any similar
affectation. I observed also the simplicity of style in his letters,
particularly in that which he wrote to my mother from Sinuessa. I
learned from him to be easily appeased, and to be readily reconciled
with those who had displeased me or given cause of offence, so soon as
they inclined to make their peace; to read with care; not to rest
satisfied with a slight and superficial knowledge; nor quickly to
assent to great talkers. I have him to thank that I met with the
discourses of Epictetus, which he furnished me from his own library.

8. From Apollonius I learned true liberty, and tenacity of purpose; to
regard nothing else, even in the smallest degree, but reason always;
and always to remain unaltered in the agonies of pain, in the losses
of children, or in long diseases. He afforded me a living example of
how the same man can, upon occasion, be most yielding and most
inflexible. He was patient in exposition; and, as might well be seen,
esteemed his fine skill and ability in teaching others the principles
of philosophy as the least of his endowments. It was from him that I
learned how to receive from friends what are thought favours without
seeming humbled by the giver or insensible to the gift.`
