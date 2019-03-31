// SPDX-License-Identifier: Unlicense OR MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"image/color"
	"log"
	"net/http"
	"os"

	"golang.org/x/image/draw"
	"golang.org/x/oauth2"

	_ "image/jpeg"
	_ "image/png"

	_ "net/http/pprof"

	"gioui.org/ui"
	"gioui.org/ui/app"
	gdraw "gioui.org/ui/draw"
	"gioui.org/ui/f32"
	"gioui.org/ui/gesture"
	"gioui.org/ui/key"
	"gioui.org/ui/layout"
	"gioui.org/ui/measure"
	"gioui.org/ui/pointer"
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

type App struct {
	w     *app.Window
	cfg   *ui.Config
	faces measure.Faces

	pqueue *pointer.Queue
	kqueue *key.Queue

	fab *ActionButton

	usersList   *layout.List
	edit, edit2 *text.Editor

	users        []*user
	userClicks   []gesture.Click
	selectedUser *userPage

	updateUsers chan []*user
}

type userPage struct {
	cfg           *ui.Config
	faces         measure.Faces
	redraw        redrawer
	user          *user
	commitsList   *layout.List
	commits       []*github.Commit
	commitsResult chan []*github.Commit
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
	imgSize float32
}

type redrawer func()

type ActionButton struct {
	face        text.Face
	cfg         *ui.Config
	Open        bool
	icons       []*icon
	sendIco     *icon
	btnClicker  *gesture.Click
	btnsClicker *gesture.Click
}

var (
	profile = flag.Bool("profile", false, "serve profiling data at http://localhost:6060")
	stats   = flag.Bool("stats", false, "show rendering statistics")
	token   = flag.String("token", "", "Github authentication token")
)

var fonts struct {
	regular *sfnt.Font
	bold    *sfnt.Font
	italic  *sfnt.Font
	mono    *sfnt.Font
}

func main() {
	if *token == "" {
		fmt.Println("The quota for anonymous GitHub API access is very low. Specify a token with -token to avoid quota errors.")
		fmt.Println("See https://help.github.com/en/articles/creating-a-personal-access-token-for-the-command-line.")
	}
	app.Main()
}

func initProfiling() {
	if !*profile {
		return
	}
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}

func init() {
	flag.Parse()
	initProfiling()
	fonts.regular = mustLoadFont(goregular.TTF)
	fonts.bold = mustLoadFont(gobold.TTF)
	fonts.italic = mustLoadFont(goitalic.TTF)
	fonts.mono = mustLoadFont(gomono.TTF)
	go func() {
		w, err := app.NewWindow(&app.WindowOptions{
			Width:  ui.Dp(400),
			Height: ui.Dp(800),
			Title:  "Gophers",
		})
		if err != nil {
			log.Fatal(err)
		}
		if err := newApp(w).run(); err != nil {
			log.Fatal(err)
		}
	}()
}

func (a *App) run() error {
	a.w.Profiling = *stats
	for a.w.IsAlive() {
		select {
		case users := <-a.updateUsers:
			a.users = users
			a.userClicks = make([]gesture.Click, len(users))
			a.w.Redraw()
		case e := <-a.w.Events():
			switch e := e.(type) {
			case pointer.Event:
				a.pqueue.Push(e)
			case key.Event:
				a.kqueue.Push(e)
				if e, ok := e.(key.Chord); ok {
					switch e.Name {
					case key.NameEscape:
						os.Exit(0)
					case 'P':
						if e.Modifiers&key.ModCommand != 0 {
							a.w.Profiling = !a.w.Profiling
						}
					}
				}
			case app.ChangeStage:
			case app.Draw:
				a.cfg = e.Config
				a.faces.Cfg = a.cfg
				cs := layout.ExactConstraints(a.w.Size())
				root, _ := a.Layout(cs)
				if a.w.Profiling {
					op, _ := layout.Align(
						layout.NE,
						layout.Margin(a.cfg,
							layout.Margins{Top: ui.Dp(16)},
							text.Label{Src: textColor, Face: a.face(fonts.mono, 8), Text: a.w.Timings()},
						),
					).Layout(cs)
					root = ui.Ops{root, op}
				}
				a.w.Draw(root)
				a.w.SetTextInput(a.kqueue.Frame(root))
				a.pqueue.Frame(root)
				a.faces.Frame()
			}
			a.w.Ack()
		}
	}
	return a.w.Err()
}

func newApp(w *app.Window) *App {
	a := &App{
		w:           w,
		updateUsers: make(chan []*user),
		pqueue:      new(pointer.Queue),
		kqueue:      new(key.Queue),
	}
	a.usersList = &layout.List{Axis: layout.Vertical}
	a.fab = &ActionButton{
		face:        a.face(fonts.regular, 9),
		sendIco:     &icon{src: icons.ContentSend, size: ui.Dp(24)},
		icons:       []*icon{},
		btnClicker:  new(gesture.Click),
		btnsClicker: new(gesture.Click),
	}
	a.edit2 = &text.Editor{
		Src:  textColor,
		Face: a.face(fonts.italic, 14),
		//Alignment: text.End,
		SingleLine: true,
	}
	a.edit2.SetText("Single line editor. Edit me!")
	a.edit = &text.Editor{
		Src:  textColor,
		Face: a.face(fonts.regular, 14),
		//Alignment: text.End,
		//SingleLine: true,
	}
	a.edit.SetText(longTextSample)
	go a.fetchContributors()
	return a
}

func githubClient(ctx context.Context) *github.Client {
	var tc *http.Client
	if *token != "" {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: *token},
		)
		tc = oauth2.NewClient(ctx, ts)
	}
	return github.NewClient(tc)
}

func (a *App) fetchContributors() {
	ctx := context.Background()
	client := githubClient(ctx)
	cons, _, err := client.Repositories.ListContributors(ctx, "golang", "go", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "github: failed to fetch contributors: %v\n", err)
		return
	}
	var users []*user
	userErrs := make(chan error, len(cons))
	avatarErrs := make(chan error, len(cons))
	for _, con := range cons {
		con := con
		avatar := con.GetAvatarURL()
		if avatar == "" {
			continue
		}
		u := &user{
			login: con.GetLogin(),
		}
		users = append(users, u)
		go func() {
			guser, _, err := client.Users.Get(ctx, u.login)
			if err != nil {
				avatarErrs <- err
				return
			}
			u.name = guser.GetName()
			u.company = guser.GetCompany()
			avatarErrs <- nil
		}()
		go func() {
			a, err := fetchImage(avatar)
			u.avatar = a
			userErrs <- err
		}()
	}
	for i := 0; i < len(cons); i++ {
		if err := <-userErrs; err != nil {
			fmt.Fprintf(os.Stderr, "github: failed to fetch user: %v\n", err)
		}
		if err := <-avatarErrs; err != nil {
			fmt.Fprintf(os.Stderr, "github: failed to fetch avatar: %v\n", err)
		}
	}
	// Drop users with no avatar or name.
	for i := len(users) - 1; i >= 0; i-- {
		if u := users[i]; u.name == "" || u.avatar == nil {
			users = append(users[:i], users[i+1:]...)
		}
	}
	a.updateUsers <- users
}

func fetchImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetchImage: http.Get(%q): %v", url, err)
	}
	defer resp.Body.Close()
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("fetchImage: image decode failed: %v", err)
	}
	return img, nil
}

func mustLoadFont(fontData []byte) *sfnt.Font {
	fnt, err := sfnt.Parse(fontData)
	if err != nil {
		panic("failed to load font")
	}
	return fnt
}

var (
	backgroundColor = rgb(0xfbfbfb)
	brandColor      = rgb(0x62798c)
	divColor        = rgb(0xecedef)
	textColor       = rgb(0x333333)
	secTextColor    = rgb(0xe0e4e8)
	tertTextColor   = rgb(0xbbbbbb)
	whiteColor      = rgb(0xffffff)
	accentColor     = rgb(0x00c28c)
)

func rgb(c uint32) *image.Uniform {
	return argb((0xff << 24) | c)
}

func argb(c uint32) *image.Uniform {
	col := color.NRGBA{A: uint8(c >> 24), R: uint8(c >> 16), G: uint8(c >> 8), B: uint8(c)}
	return &image.Uniform{col}
}

func (a *App) face(f *sfnt.Font, size float32) text.Face {
	return a.faces.For(f, ui.Sp(size))
}

func (a *App) Layout(cs layout.Constraints) (ui.Op, layout.Dimens) {
	if a.selectedUser == nil {
		return a.layoutUsers(cs)
	} else {
		a.selectedUser.Update(a.pqueue)
		return a.selectedUser.Layout(cs)
	}
}

func newUserPage(cfg *ui.Config, user *user, redraw redrawer, faces measure.Faces) *userPage {
	up := &userPage{
		cfg:           cfg,
		faces:         faces,
		redraw:        redraw,
		user:          user,
		commitsList:   &layout.List{Axis: layout.Vertical},
		commitsResult: make(chan []*github.Commit, 1),
	}
	up.fetchCommits()
	return up
}

func (up *userPage) Update(pqueue pointer.Events) {
	up.commitsList.Scroll(up.cfg, pqueue)
}

func (up *userPage) Layout(cs layout.Constraints) (ui.Op, layout.Dimens) {
	l := up.commitsList
	var ops ui.Ops
	if l.Dragging() {
		ops = append(ops, key.OpHideInput{})
	}
	select {
	case commits := <-up.commitsResult:
		up.commits = commits
	default:
	}
	for i, ok := l.Init(cs, len(up.commits)); ok; i, ok = l.Index() {
		l.Elem(up.commit(i))
	}
	op, dims := l.Layout()
	return append(ops, op), dims
}

func (up *userPage) commit(index int) layout.Widget {
	sz := ui.Dp(48)
	u := up.user
	c := up.cfg
	avatar := clipCircle(layout.Sized(c, sz, sz, widget.Image{Src: u.avatar, Rect: u.avatar.Bounds()}))
	msg := up.commits[index].GetMessage()
	label := text.Label{Src: textColor, Face: up.faces.For(fonts.regular, ui.Sp(12)), Text: msg}
	return layout.Margin(c,
		layout.Margins{Top: ui.Dp(16), Right: ui.Dp(8), Left: ui.Dp(8)},
		layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
			return (&layout.Flex{Axis: layout.Horizontal, MainAxisAlignment: layout.Start, CrossAxisAlignment: layout.Start}).
				Init(cs).
				Rigid(avatar).
				Flexible(-1, 1, layout.Fit, layout.Margin(c, layout.Margins{Left: ui.Dp(8)}, label)).
				Layout()
		}),
	)
}

func (up *userPage) fetchCommits() {
	go func() {
		ctx := context.Background()
		gh := githubClient(ctx)
		repoCommits, _, err := gh.Repositories.ListCommits(ctx, "golang", "go", &github.CommitsListOptions{
			Author: up.user.login,
		})
		if err != nil {
			log.Printf("failed to fetch commits: %v", err)
			return
		}
		var commits []*github.Commit
		for _, commit := range repoCommits {
			if c := commit.GetCommit(); c != nil {
				commits = append(commits, c)
			}
		}
		up.commitsResult <- commits
		up.redraw()
	}()
}

func (a *App) layoutUsers(cs layout.Constraints) (ui.Op, layout.Dimens) {
	c := a.cfg
	a.fab.Update(c, a.pqueue)
	st := (&layout.Stack{Alignment: layout.Center}).Init(cs).
		Rigid(layout.Align(
			layout.SE,
			layout.Margin(c,
				layout.EqualMargins(ui.Dp(16)),
				a.fab,
			),
		))
	a.edit.Update(c, a.pqueue, a.kqueue)
	a.edit2.Update(c, a.pqueue, a.kqueue)
	return st.Expand(0, layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
		return (&layout.Flex{Axis: layout.Vertical, MainAxisAlignment: layout.Start, CrossAxisAlignment: layout.Stretch}).Init(cs).
			Rigid(layout.Margin(c,
				layout.EqualMargins(ui.Dp(16)),
				layout.Sized(c, ui.Dp(0), ui.Dp(200), a.edit),
			)).
			Rigid(layout.Margin(c,
				layout.Margins{Bottom: ui.Dp(16), Left: ui.Dp(16), Right: ui.Dp(16)},
				a.edit2,
			)).
			Rigid(layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
				return (&layout.Stack{Alignment: layout.Center}).Init(cs).
					Rigid(layout.Margin(c,
						layout.Margins{Top: ui.Dp(16), Right: ui.Dp(8), Bottom: ui.Dp(8), Left: ui.Dp(8)},
						text.Label{Src: rgb(0x888888), Face: a.face(fonts.regular, 9), Text: "GOPHERS"},
					)).
					Expand(0, fill(rgb(0xf2f2f2))).
					Layout()
			})).
			Flexible(-1, 1, layout.Fit, a.layoutContributors()).
			Layout()
	})).
		Layout()
}

func (a *ActionButton) Update(c *ui.Config, q pointer.Events) {
	a.cfg = c
	a.btnsClicker.Update(q)
	a.btnClicker.Update(q)
}

func (a *ActionButton) Layout(cs layout.Constraints) (ui.Op, layout.Dimens) {
	c := a.cfg
	fl := (&layout.Flex{Axis: layout.Vertical, MainAxisAlignment: layout.Start, CrossAxisAlignment: layout.End, MainAxisSize: layout.Min}).Init(cs)
	fabCol := brandColor
	fl.Rigid(layout.Margin(c,
		layout.Margins{Top: ui.Dp(4)},
		layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
			op, dims := fab(c, a.sendIco.image(c), fabCol, ui.Dp(56)).Layout(cs)
			ops := ui.Ops{op, a.btnClicker.Op(gesture.Ellipse(dims.Size))}
			return ops, dims
		}),
	))
	return fl.Layout()
}

func (a *App) layoutContributors() layout.Widget {
	return layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
		c := a.cfg
		l := a.usersList
		l.Scroll(c, a.pqueue)
		var ops ui.Ops
		if l.Dragging() {
			ops = append(ops, key.OpHideInput{})
		}
		for i, ok := l.Init(cs, len(a.users)); ok; i, ok = l.Index() {
			l.Elem(a.user(c, i))
		}
		op, dims := l.Layout()
		return append(ops, op), dims
	})
}

func (a *App) user(c *ui.Config, index int) layout.Widget {
	u := a.users[index]
	click := &a.userClicks[index]
	sz := ui.Dp(48)
	for _, r := range click.Update(a.pqueue) {
		if r.Type == gesture.TypeClick {
			a.selectedUser = newUserPage(a.cfg, u, a.w.Redraw, a.faces)
		}
	}
	avatar := clipCircle(layout.Sized(a.cfg, sz, sz, widget.Image{Src: u.avatar, Rect: u.avatar.Bounds()}))
	return layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
		elem := (&layout.Flex{Axis: layout.Vertical, MainAxisAlignment: layout.Start, CrossAxisAlignment: layout.Start}).Init(cs)
		elem.Rigid(layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
			op, dims := layout.Margin(c,
				layout.EqualMargins(ui.Dp(8)),
				layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
					return centerRowOpts().Init(cs).
						Rigid(layout.Margin(c, layout.Margins{Right: ui.Dp(8)}, avatar)).
						Rigid(column(
							baseline(
								text.Label{Src: textColor, Face: a.face(fonts.regular, 11), Text: u.name},
								layout.Align(layout.E, layout.Margin(c,
									layout.Margins{Left: ui.Dp(2)},
									text.Label{Src: textColor, Face: a.face(fonts.regular, 8), Text: "3 hours ago"},
								)),
							),
							layout.Margin(c,
								layout.Margins{Top: ui.Dp(4)},
								text.Label{Src: tertTextColor, Face: a.face(fonts.regular, 10), Text: u.company},
							),
						)).
						Layout()
				}),
			).Layout(cs)
			ops := ui.Ops{op, click.Op(gesture.Rect(dims.Size))}
			return ops, dims
		}))
		return elem.Layout()
	})
}

func fill(img image.Image) layout.Widget {
	return widget.Image{Src: img, Rect: image.Rectangle{Max: image.Point{X: 1, Y: 1}}}
}

func column(widgets ...layout.Widget) layout.Widget {
	return flex(&layout.Flex{Axis: layout.Vertical, MainAxisAlignment: layout.Start, CrossAxisAlignment: layout.Start}, widgets...)
}

func centerColumn(widgets ...layout.Widget) layout.Widget {
	return flex(&layout.Flex{Axis: layout.Vertical, MainAxisAlignment: layout.Start, CrossAxisAlignment: layout.Center, MainAxisSize: layout.Min}, widgets...)
}

func centerRowOpts(widgets ...layout.Widget) *layout.Flex {
	return &layout.Flex{Axis: layout.Horizontal, MainAxisAlignment: layout.Start, CrossAxisAlignment: layout.Center, MainAxisSize: layout.Min}
}

func centerRow(widgets ...layout.Widget) layout.Widget {
	return flex(centerRowOpts(), widgets...)
}

func baseline(widgets ...layout.Widget) layout.Widget {
	return flex(&layout.Flex{Axis: layout.Horizontal, CrossAxisAlignment: layout.Baseline, MainAxisSize: layout.Min}, widgets...)
}

func flex(f *layout.Flex, widgets ...layout.Widget) layout.Widget {
	return layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
		f.Init(cs)
		for _, w := range widgets {
			f.Rigid(w)
		}
		return f.Layout()
	})
}

func clipCircle(w layout.Widget) layout.Widget {
	return layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
		op, dims := w.Layout(cs)
		max := dims.Size.X
		if dy := dims.Size.Y; dy > max {
			max = dy
		}
		szf := float32(max)
		rr := szf * .5
		op = gdraw.OpClip{
			Path: rrect(szf, szf, rr, rr, rr, rr),
			Op:   op,
		}
		return op, dims
	})
}

func fab(c *ui.Config, ico, col image.Image, size ui.Value) layout.Widget {
	return layout.F(func(cs layout.Constraints) (ui.Op, layout.Dimens) {
		szf := c.Pixels(size)
		sz := int(szf + .5)
		rr := szf * .5
		dp := image.Point{X: (sz - ico.Bounds().Dx()) / 2, Y: (sz - ico.Bounds().Dy()) / 2}
		dims := image.Point{X: sz, Y: sz}
		op := gdraw.OpClip{
			Path: rrect(szf, szf, rr, rr, rr, rr),
			Op: ui.Ops{
				gdraw.OpImage{Rect: f32.Rectangle{Max: f32.Point{X: float32(sz), Y: float32(sz)}}, Src: col, SrcRect: col.Bounds()},
				gdraw.OpImage{
					Rect:    toRectF(ico.Bounds().Add(dp)),
					Src:     ico,
					SrcRect: ico.Bounds(),
				},
			},
		}
		return op, layout.Dimens{Size: dims}
	})
}

func toRectF(r image.Rectangle) f32.Rectangle {
	return f32.Rectangle{
		Min: f32.Point{X: float32(r.Min.X), Y: float32(r.Min.Y)},
		Max: f32.Point{X: float32(r.Max.X), Y: float32(r.Max.Y)},
	}
}

func (ic *icon) image(cfg *ui.Config) image.Image {
	sz := cfg.Pixels(ic.size)
	if sz == ic.imgSize {
		return ic.img
	}
	m, _ := iconvg.DecodeMetadata(ic.src)
	dx, dy := m.ViewBox.AspectRatio()
	img := image.NewNRGBA(image.Rectangle{Max: image.Point{X: int(sz), Y: int(sz * dy / dx)}})
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
func rrect(width, height, se, sw, nw, ne float32) *gdraw.Path {
	w, h := float32(width), float32(height)
	const c = 0.55228475 // 4*(sqrt(2)-1)/3
	var b gdraw.PathBuilder
	b.Move(f32.Point{X: w, Y: h - se})
	b.Cube(f32.Point{X: 0, Y: se * c}, f32.Point{X: -se + se*c, Y: se}, f32.Point{X: -se, Y: se}) // SE
	b.Line(f32.Point{X: sw - w + se, Y: 0})
	b.Cube(f32.Point{X: -sw * c, Y: 0}, f32.Point{X: -sw, Y: -sw + sw*c}, f32.Point{X: -sw, Y: -sw}) // SW
	b.Line(f32.Point{X: 0, Y: nw - h + sw})
	b.Cube(f32.Point{X: 0, Y: -nw * c}, f32.Point{X: nw - nw*c, Y: -nw}, f32.Point{X: nw, Y: -nw}) // NW
	b.Line(f32.Point{X: w - ne - nw, Y: 0})
	b.Cube(f32.Point{X: ne * c, Y: 0}, f32.Point{X: ne, Y: ne - ne*c}, f32.Point{X: ne, Y: ne}) // NE
	return b.Path()
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
