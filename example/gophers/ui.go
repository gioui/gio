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

	"gioui.org/f32"
	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/profile"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/text/shape"
	"gioui.org/unit"
	"gioui.org/widget"
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
	fab          *ActionButton
	usersList    *layout.List
	users        []*user
	userClicks   []gesture.Click
	selectedUser *userPage
	edit, edit2  *text.Editor
	fetchCommits func(u string)

	// Profiling.
	profiling   bool
	profile     profile.Event
	lastMallocs uint64
}

type userPage struct {
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
	size unit.Value

	// Cached values.
	img     image.Image
	imgSize int
}

type ActionButton struct {
	Open    bool
	icons   []*icon
	sendIco *icon
}

var families struct {
	primary text.Family
	mono    text.Family
}

var theme struct {
	text     op.MacroOp
	tertText op.MacroOp
	brand    op.MacroOp
	white    op.MacroOp
}

func colorMaterial(ops *op.Ops, color color.RGBA) op.MacroOp {
	var mat op.MacroOp
	mat.Record(ops)
	paint.ColorOp{Color: color}.Add(ops)
	mat.Stop()
	return mat
}

func init() {
	families.primary = &shape.Family{
		Regular: mustLoadFont(goregular.TTF),
		Bold:    mustLoadFont(gobold.TTF),
		Italic:  mustLoadFont(goitalic.TTF),
	}
	families.mono = &shape.Family{
		Regular: mustLoadFont(gomono.TTF),
	}
	var ops op.Ops
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
		sendIco: &icon{src: icons.ContentSend, size: unit.Dp(24)},
		icons:   []*icon{},
	}
	u.edit2 = &text.Editor{
		Family: families.primary,
		Face: text.Face{
			Style: text.Italic,
		},
		Size: unit.Sp(14),
		//Alignment: text.End,
		SingleLine:   true,
		Hint:         "Hint",
		HintMaterial: theme.tertText,
		Material:     theme.text,
	}
	u.edit2.SetText("Single line editor. Edit me!")
	u.edit = &text.Editor{
		Family:   families.primary,
		Size:     unit.Sp(16),
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

func (u *UI) layoutTimings(gtx *layout.Context) {
	if !u.profiling {
		return
	}
	for _, e := range gtx.Events(u) {
		if e, ok := e.(profile.Event); ok {
			u.profile = e
		}
	}
	profile.Op{Key: u}.Add(gtx.Ops)
	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)
	mallocs := mstats.Mallocs - u.lastMallocs
	u.lastMallocs = mstats.Mallocs
	layout.Align(layout.NE).Layout(gtx, func() {
		layout.Inset{Top: unit.Dp(16)}.Layout(gtx, func() {
			txt := fmt.Sprintf("m: %d %s", mallocs, u.profile.Timings)
			text.Label{Material: theme.text, Size: unit.Sp(10), Text: txt}.Layout(gtx, families.mono)
		})
	})
}

func (u *UI) Layout(gtx *layout.Context) {
	for i := range u.userClicks {
		click := &u.userClicks[i]
		for _, e := range click.Events(gtx) {
			if e.Type == gesture.TypeClick {
				u.selectedUser = u.newUserPage(u.users[i])
			}
		}
	}
	if u.selectedUser == nil {
		u.layoutUsers(gtx)
	} else {
		u.selectedUser.Layout(gtx)
	}
	u.layoutTimings(gtx)
}

func (u *UI) newUserPage(user *user) *userPage {
	up := &userPage{
		user:        user,
		commitsList: &layout.List{Axis: layout.Vertical},
	}
	u.fetchCommits(user.login)
	return up
}

func (up *userPage) Layout(gtx *layout.Context) {
	l := up.commitsList
	if l.Dragging() {
		key.HideInputOp{}.Add(gtx.Ops)
	}
	l.Layout(gtx, len(up.commits), func(i int) {
		up.commit(gtx, i)
	})
}

func (up *userPage) commit(gtx *layout.Context, index int) {
	u := up.user
	msg := up.commits[index].GetMessage()
	label := text.Label{Material: theme.text, Size: unit.Sp(12), Text: msg}
	in := layout.Inset{Top: unit.Dp(16), Right: unit.Dp(8), Left: unit.Dp(8)}
	in.Layout(gtx, func() {
		f := layout.Flex{Axis: layout.Horizontal}
		c1 := f.Rigid(gtx, func() {
			sz := gtx.Px(unit.Dp(48))
			cc := clipCircle{}
			cc.Layout(gtx, func() {
				gtx.Constraints = layout.RigidConstraints(gtx.Constraints.Constrain(image.Point{X: sz, Y: sz}))
				widget.Image{Src: u.avatar, Rect: u.avatar.Bounds()}.Layout(gtx)
			})
		})
		c2 := f.Flex(gtx, 1, func() {
			gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
			layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func() {
				label.Layout(gtx, families.primary)
			})
		})
		f.Layout(gtx, c1, c2)
	})
}

func (u *UI) layoutUsers(gtx *layout.Context) {
	var st layout.Stack
	c2 := st.Rigid(gtx, func() {
		layout.Align(layout.SE).Layout(gtx, func() {
			in := layout.UniformInset(unit.Dp(16))
			in.Layout(gtx, func() {
				u.fab.Layout(gtx)
			})
		})
	})

	c1 := st.Expand(gtx, func() {
		f := layout.Flex{Axis: layout.Vertical}

		c1 := f.Rigid(gtx, func() {
			gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
			layout.UniformInset(unit.Dp(16)).Layout(gtx, func() {
				sz := gtx.Px(unit.Dp(200))
				cs := gtx.Constraints
				gtx.Constraints = layout.RigidConstraints(cs.Constrain(image.Point{X: sz, Y: sz}))
				u.edit.Layout(gtx)
			})
		})

		c2 := f.Rigid(gtx, func() {
			gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
			in := layout.Inset{Bottom: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16)}
			in.Layout(gtx, func() {
				u.edit2.Layout(gtx)
			})
		})

		c3 := f.Rigid(gtx, func() {
			gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
			s := layout.Stack{Alignment: layout.Center}
			c2 := s.Rigid(gtx, func() {
				grey := colorMaterial(gtx.Ops, rgb(0x888888))
				in := layout.Inset{Top: unit.Dp(16), Right: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8)}
				in.Layout(gtx, func() {
					lbl := text.Label{Material: grey, Size: unit.Sp(11), Text: "GOPHERS"}
					lbl.Layout(gtx, families.primary)
				})
			})
			c1 := s.Expand(gtx, func() {
				fill{colorMaterial(gtx.Ops, rgb(0xf2f2f2))}.Layout(gtx)
			})
			s.Layout(gtx, c1, c2)
		})

		c4 := f.Flex(gtx, 1, func() {
			gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
			u.layoutContributors(gtx)
		})
		f.Layout(gtx, c1, c2, c3, c4)
	})
	st.Layout(gtx, c1, c2)
}

func (a *ActionButton) Layout(gtx *layout.Context) {
	f := layout.Flex{Axis: layout.Vertical, Alignment: layout.End}
	f.Layout(gtx, f.Rigid(gtx, func() {
		layout.Inset{Top: unit.Dp(4)}.Layout(gtx, func() {
			fab(gtx, a.sendIco.image(gtx), theme.brand, gtx.Px(unit.Dp(56)))
			pointer.EllipseAreaOp{Rect: image.Rectangle{Max: gtx.Dimensions.Size}}.Add(gtx.Ops)
		})
	}))
}

func (u *UI) layoutContributors(gtx *layout.Context) {
	l := u.usersList
	if l.Dragging() {
		key.HideInputOp{}.Add(gtx.Ops)
	}
	l.Layout(gtx, len(u.users), func(i int) {
		u.user(gtx, i)
	})
}

func (u *UI) user(gtx *layout.Context, index int) {
	user := u.users[index]
	elem := layout.Flex{Axis: layout.Vertical}
	c1 := elem.Rigid(gtx, func() {
		in := layout.UniformInset(unit.Dp(8))
		in.Layout(gtx, func() {
			f := centerRowOpts()
			c1 := f.Rigid(gtx, func() {
				in := layout.Inset{Right: unit.Dp(8)}
				cc := clipCircle{}
				in.Layout(gtx, func() {
					cc.Layout(gtx, func() {
						sz := image.Point{X: gtx.Px(unit.Dp(48)), Y: gtx.Px(unit.Dp(48))}
						gtx.Constraints = layout.RigidConstraints(gtx.Constraints.Constrain(sz))
						widget.Image{Src: user.avatar, Rect: user.avatar.Bounds()}.Layout(gtx)
					})
				})
			})
			c2 := f.Rigid(gtx, func() {
				f := column()
				c1 := f.Rigid(gtx, func() {
					f := baseline()
					c1 := f.Rigid(gtx, func() {
						text.Label{Material: theme.text, Size: unit.Sp(13), Text: user.name}.Layout(gtx, families.primary)
					})
					c2 := f.Flex(gtx, 1, func() {
						gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
						layout.Align(layout.E).Layout(gtx, func() {
							layout.Inset{Left: unit.Dp(2)}.Layout(gtx, func() {
								lbl := text.Label{Material: theme.text, Size: unit.Sp(10), Text: "3 hours ago"}
								lbl.Layout(gtx, families.primary)
							})
						})
					})
					f.Layout(gtx, c1, c2)
				})
				c2 := f.Rigid(gtx, func() {
					in := layout.Inset{Top: unit.Dp(4)}
					in.Layout(gtx, func() {
						text.Label{Material: theme.tertText, Size: unit.Sp(12), Text: user.company}.Layout(gtx, families.primary)
					})
				})
				f.Layout(gtx, c1, c2)
			})
			f.Layout(gtx, c1, c2)
		})
		pointer.RectAreaOp{Rect: image.Rectangle{Max: gtx.Dimensions.Size}}.Add(gtx.Ops)
		click := &u.userClicks[index]
		click.Add(gtx.Ops)
	})
	elem.Layout(gtx, c1)
}

type fill struct {
	material op.MacroOp
}

func (f fill) Layout(gtx *layout.Context) {
	cs := gtx.Constraints
	d := image.Point{X: cs.Width.Max, Y: cs.Height.Max}
	dr := f32.Rectangle{
		Max: f32.Point{X: float32(d.X), Y: float32(d.Y)},
	}
	f.material.Add(gtx.Ops)
	paint.PaintOp{Rect: dr}.Add(gtx.Ops)
	gtx.Dimensions = layout.Dimensions{Size: d, Baseline: d.Y}
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

func (c *clipCircle) Layout(gtx *layout.Context, w layout.Widget) {
	var m op.MacroOp
	m.Record(gtx.Ops)
	w()
	m.Stop()
	dims := gtx.Dimensions
	max := dims.Size.X
	if dy := dims.Size.Y; dy > max {
		max = dy
	}
	szf := float32(max)
	rr := szf * .5
	var stack op.StackOp
	stack.Push(gtx.Ops)
	rrect(gtx.Ops, szf, szf, rr, rr, rr, rr)
	m.Add(gtx.Ops)
	stack.Pop()
}

func fab(gtx *layout.Context, ico image.Image, mat op.MacroOp, size int) {
	dp := image.Point{X: (size - ico.Bounds().Dx()) / 2, Y: (size - ico.Bounds().Dy()) / 2}
	dims := image.Point{X: size, Y: size}
	rr := float32(size) * .5
	rrect(gtx.Ops, float32(size), float32(size), rr, rr, rr, rr)
	mat.Add(gtx.Ops)
	paint.PaintOp{Rect: f32.Rectangle{Max: f32.Point{X: float32(size), Y: float32(size)}}}.Add(gtx.Ops)
	paint.ImageOp{Src: ico, Rect: ico.Bounds()}.Add(gtx.Ops)
	paint.PaintOp{
		Rect: toRectF(ico.Bounds().Add(dp)),
	}.Add(gtx.Ops)
	gtx.Dimensions = layout.Dimensions{Size: dims}
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: float32(r.Min.X), Y: float32(r.Min.Y)},
		Max: f32.Point{X: float32(r.Max.X), Y: float32(r.Max.Y)},
	}
}

func (ic *icon) image(cfg unit.Converter) image.Image {
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
func rrect(ops *op.Ops, width, height, se, sw, nw, ne float32) {
	w, h := float32(width), float32(height)
	const c = 0.55228475 // 4*(sqrt(2)-1)/3
	var b paint.Path
	b.Begin(ops)
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
