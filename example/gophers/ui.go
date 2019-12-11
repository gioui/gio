// SPDX-License-Identifier: Unlicense OR MIT

package main

// A Gio program that displays Go contributors from GitHub. See https://gioui.org for more information.

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"runtime"

	"gioui.org/f32"
	"gioui.org/font/gofont"
	"gioui.org/gesture"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/io/profile"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"github.com/google/go-github/v24/github"

	"golang.org/x/exp/shiny/materialdesign/icons"

	"golang.org/x/image/draw"
)

type UI struct {
	fab          *widget.Button
	fabIcon      *material.Icon
	usersList    *layout.List
	users        []*user
	userClicks   []gesture.Click
	selectedUser *userPage
	edit, edit2  *widget.Editor
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
	name     string
	login    string
	company  string
	avatar   image.Image
	avatarOp paint.ImageOp
}

var theme *material.Theme

func init() {
	gofont.Register()
	theme = material.NewTheme()
	theme.Color.Text = rgb(0x333333)
	theme.Color.Hint = rgb(0xbbbbbb)
}

func newUI(fetchCommits func(string)) *UI {
	u := &UI{
		fetchCommits: fetchCommits,
	}
	u.usersList = &layout.List{
		Axis: layout.Vertical,
	}
	u.fab = new(widget.Button)
	u.edit2 = &widget.Editor{
		//Alignment: text.End,
		SingleLine: true,
	}
	var err error
	u.fabIcon, err = material.NewIcon(icons.ContentSend)
	if err != nil {
		log.Fatal(err)
	}
	u.edit2.SetText("Single line editor. Edit me!")
	u.edit = &widget.Editor{
		//Alignment: text.End,
		//SingleLine: true,
	}
	u.edit.SetText(longTextSample)
	return u
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
			lbl := theme.Caption(txt)
			lbl.Font.Variant = "Mono"
			lbl.Layout(gtx)
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
	label := theme.Caption(msg)
	in := layout.Inset{Top: unit.Dp(16), Right: unit.Dp(8), Left: unit.Dp(8)}
	in.Layout(gtx, func() {
		layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(func() {
				sz := gtx.Px(unit.Dp(48))
				cc := clipCircle{}
				cc.Layout(gtx, func() {
					gtx.Constraints = layout.RigidConstraints(gtx.Constraints.Constrain(image.Point{X: sz, Y: sz}))
					u.layoutAvatar(gtx)
				})
			}),
			layout.Flexed(1, func() {
				gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
				layout.Inset{Left: unit.Dp(8)}.Layout(gtx, func() {
					label.Layout(gtx)
				})
			}),
		)
	})
}

func (u *UI) layoutUsers(gtx *layout.Context) {
	layout.Stack{Alignment: layout.SE}.Layout(gtx,
		layout.Expanded(func() {
			layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func() {
					gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
					layout.UniformInset(unit.Dp(16)).Layout(gtx, func() {
						sz := gtx.Px(unit.Dp(200))
						cs := gtx.Constraints
						gtx.Constraints = layout.RigidConstraints(cs.Constrain(image.Point{X: sz, Y: sz}))
						theme.Editor("Hint").Layout(gtx, u.edit)
					})
				}),
				layout.Rigid(func() {
					gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
					in := layout.Inset{Bottom: unit.Dp(16), Left: unit.Dp(16), Right: unit.Dp(16)}
					in.Layout(gtx, func() {
						e := theme.Editor("Hint")
						e.Font.Size = unit.Sp(14)
						e.Font.Style = text.Italic
						e.Layout(gtx, u.edit2)
					})
				}),
				layout.Rigid(func() {
					layout.Stack{}.Layout(gtx,
						layout.Expanded(func() {
							gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
							fill{rgb(0xf2f2f2)}.Layout(gtx)
						}),
						layout.Stacked(func() {
							in := layout.Inset{Top: unit.Dp(16), Right: unit.Dp(8), Bottom: unit.Dp(8), Left: unit.Dp(8)}
							in.Layout(gtx, func() {
								lbl := theme.Caption("GOPHERS")
								lbl.Color = rgb(0x888888)
								lbl.Layout(gtx)
							})
						}),
					)
				}),
				layout.Flexed(1, func() {
					gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
					u.layoutContributors(gtx)
				}),
			)
		}),
		layout.Stacked(func() {
			in := layout.UniformInset(unit.Dp(16))
			in.Layout(gtx, func() {
				for u.fab.Clicked(gtx) {
				}
				theme.IconButton(u.fabIcon).Layout(gtx, u.fab)
			})
		}),
	)
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
	in := layout.UniformInset(unit.Dp(8))
	in.Layout(gtx, func() {
		centerRowOpts().Layout(gtx,
			layout.Rigid(func() {
				in := layout.Inset{Right: unit.Dp(8)}
				cc := clipCircle{}
				in.Layout(gtx, func() {
					cc.Layout(gtx, func() {
						dim := gtx.Px(unit.Dp(48))
						sz := image.Point{X: dim, Y: dim}
						gtx.Constraints = layout.RigidConstraints(gtx.Constraints.Constrain(sz))
						user.layoutAvatar(gtx)
					})
				})
			}),
			layout.Rigid(func() {
				column().Layout(gtx,
					layout.Rigid(func() {
						baseline().Layout(gtx,
							layout.Rigid(func() {
								theme.Body1(user.name).Layout(gtx)
							}),
							layout.Flexed(1, func() {
								gtx.Constraints.Width.Min = gtx.Constraints.Width.Max
								layout.Align(layout.E).Layout(gtx, func() {
									layout.Inset{Left: unit.Dp(2)}.Layout(gtx, func() {
										theme.Caption("3 hours ago").Layout(gtx)
									})
								})
							}),
						)
					}),
					layout.Rigid(func() {
						in := layout.Inset{Top: unit.Dp(4)}
						in.Layout(gtx, func() {
							lbl := theme.Caption(user.company)
							lbl.Color = rgb(0xbbbbbb)
							lbl.Layout(gtx)
						})
					}),
				)
			}),
		)
	})
	pointer.Rect(image.Rectangle{Max: gtx.Dimensions.Size}).Add(gtx.Ops)
	click := &u.userClicks[index]
	click.Add(gtx.Ops)
}

func (u *user) layoutAvatar(gtx *layout.Context) {
	sz := gtx.Constraints.Width.Min
	if u.avatarOp.Size().X != sz {
		img := image.NewRGBA(image.Rectangle{Max: image.Point{X: sz, Y: sz}})
		draw.ApproxBiLinear.Scale(img, img.Bounds(), u.avatar, u.avatar.Bounds(), draw.Src, nil)
		u.avatarOp = paint.NewImageOp(img)
	}
	img := theme.Image(u.avatarOp)
	img.Scale = float32(sz) / float32(gtx.Px(unit.Dp(float32(sz))))
	img.Layout(gtx)
}

type fill struct {
	col color.RGBA
}

func (f fill) Layout(gtx *layout.Context) {
	cs := gtx.Constraints
	d := image.Point{X: cs.Width.Min, Y: cs.Height.Min}
	dr := f32.Rectangle{
		Max: f32.Point{X: float32(d.X), Y: float32(d.Y)},
	}
	paint.ColorOp{Color: f.col}.Add(gtx.Ops)
	paint.PaintOp{Rect: dr}.Add(gtx.Ops)
	gtx.Dimensions = layout.Dimensions{Size: d}
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
	clip.Rect{
		Rect: f32.Rectangle{Max: f32.Point{X: szf, Y: szf}},
		NE:   rr, NW: rr, SE: rr, SW: rr,
	}.Op(gtx.Ops).Add(gtx.Ops)
	m.Add()
	stack.Pop()
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
