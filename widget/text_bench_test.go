package widget

import (
	"fmt"
	"image"
	"math/rand"
	"os"
	"sort"
	"testing"

	colEmoji "eliasnaur.com/font/noto/emoji/color"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/gpu/headless"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/exp/maps"
)

var (
	documents = map[string]string{
		"latin":   latinDocument,
		"arabic":  arabicDocument,
		"complex": complexDocument,
		"emoji":   emojiDocument,
	}
	emojiFace = func() opentype.Face {
		face, _ := opentype.Parse(colEmoji.TTF)
		return face
	}()
	sizes      = []int{10, 100, 1000}
	locales    = []system.Locale{arabic, english}
	benchFonts = func() []font.FontFace {
		collection := gofont.Collection()
		collection = append(collection, arabicCollection...)
		collection = append(collection, font.FontFace{
			Font: font.Font{
				Typeface: "Noto Color Emoji",
			},
			Face: emojiFace,
		})
		return collection
	}()
)

func runBenchmarkPermutations(b *testing.B, benchmark func(b *testing.B, runes int, locale system.Locale, document string)) {
	docKeys := maps.Keys(documents)
	sort.Strings(docKeys)
	for _, locale := range locales {
		for _, runes := range sizes {
			for _, textType := range docKeys {
				txt := documents[textType]
				b.Run(fmt.Sprintf("%drunes-%s-%s", runes, locale.Direction, textType), func(b *testing.B) {
					benchmark(b, runes, locale, txt)
				})
			}
		}
	}
}

var render bool

func init() {
	if _, ok := os.LookupEnv("RENDER_WIDGET_TESTS"); ok {
		render = true
	}
}

func BenchmarkLabelStatic(b *testing.B) {
	runBenchmarkPermutations(b, func(b *testing.B, runeCount int, locale system.Locale, txt string) {
		var win *headless.Window
		size := image.Pt(200, 1000)
		gtx := layout.Context{
			Ops: new(op.Ops),
			Constraints: layout.Constraints{
				Max: size,
			},
			Locale: locale,
		}
		cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(benchFonts))
		if render {
			win, _ = headless.NewWindow(size.X, size.Y)
			defer win.Release()
		}
		fontSize := unit.Sp(10)
		font := font.Font{}
		runes := []rune(txt)[:runeCount]
		runesStr := string(runes)
		l := Label{}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Layout(gtx, cache, font, fontSize, runesStr, op.CallOp{})
			if render {
				win.Frame(gtx.Ops)
			}
			gtx.Ops.Reset()
		}
	})
}

func BenchmarkLabelDynamic(b *testing.B) {
	runBenchmarkPermutations(b, func(b *testing.B, runeCount int, locale system.Locale, txt string) {
		var win *headless.Window
		size := image.Pt(200, 1000)
		gtx := layout.Context{
			Ops: new(op.Ops),
			Constraints: layout.Constraints{
				Max: size,
			},
			Locale: locale,
		}
		cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(benchFonts))
		if render {
			win, _ = headless.NewWindow(size.X, size.Y)
			defer win.Release()
		}
		fontSize := unit.Sp(10)
		font := font.Font{}
		runes := []rune(txt)[:runeCount]
		l := Label{}
		r := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// simulate a constantly changing string
			a := r.Intn(len(runes))
			b := r.Intn(len(runes))
			runes[a], runes[b] = runes[b], runes[a]
			l.Layout(gtx, cache, font, fontSize, string(runes), op.CallOp{})
			if render {
				win.Frame(gtx.Ops)
			}
			gtx.Ops.Reset()
		}
	})
}

func BenchmarkEditorStatic(b *testing.B) {
	runBenchmarkPermutations(b, func(b *testing.B, runeCount int, locale system.Locale, txt string) {
		var win *headless.Window
		size := image.Pt(200, 1000)
		gtx := layout.Context{
			Ops: new(op.Ops),
			Constraints: layout.Constraints{
				Max: size,
			},
			Locale: locale,
		}
		cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(benchFonts))
		if render {
			win, _ = headless.NewWindow(size.X, size.Y)
			defer win.Release()
		}
		fontSize := unit.Sp(10)
		font := font.Font{}
		runes := []rune(txt)[:runeCount]
		runesStr := string(runes)
		e := Editor{}
		e.SetText(runesStr)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
			if render {
				win.Frame(gtx.Ops)
			}
			gtx.Ops.Reset()
		}
	})
}

func BenchmarkEditorDynamic(b *testing.B) {
	runBenchmarkPermutations(b, func(b *testing.B, runeCount int, locale system.Locale, txt string) {
		var win *headless.Window
		size := image.Pt(200, 1000)
		gtx := layout.Context{
			Ops: new(op.Ops),
			Constraints: layout.Constraints{
				Max: size,
			},
			Locale: locale,
		}
		cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(benchFonts))
		if render {
			win, _ = headless.NewWindow(size.X, size.Y)
			defer win.Release()
		}
		fontSize := unit.Sp(10)
		font := font.Font{}
		runes := []rune(txt)[:runeCount]
		e := Editor{}
		e.SetText(string(runes))
		r := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// simulate a constantly changing string
			a := r.Intn(e.Len())
			b := r.Intn(e.Len())
			e.SetCaret(a, a+1)
			takeStr := e.SelectedText()
			e.Insert("")
			e.SetCaret(b, b)
			e.Insert(takeStr)
			e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
			if render {
				win.Frame(gtx.Ops)
			}
			gtx.Ops.Reset()
		}
	})
}

func FuzzEditorEditing(f *testing.F) {
	f.Add(complexDocument, int16(0), int16(len([]rune(complexDocument))))
	gtx := layout.Context{
		Ops: new(op.Ops),
		Constraints: layout.Constraints{
			Max: image.Pt(200, 1000),
		},
		Locale: arabic,
	}
	cache := text.NewShaper(text.NoSystemFonts(), text.WithCollection(benchFonts))
	fontSize := unit.Sp(10)
	font := font.Font{}
	e := Editor{}
	f.Fuzz(func(t *testing.T, txt string, replaceFrom, replaceTo int16) {
		e.SetText(txt)
		e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
		// simulate a constantly changing string
		if e.Len() > 0 {
			a := int(replaceFrom) % e.Len()
			b := int(replaceTo) % e.Len()
			e.SetCaret(a, a+1)
			takeStr := e.SelectedText()
			e.Insert("")
			e.SetCaret(b, b)
			e.Insert(takeStr)
		}
		e.Layout(gtx, cache, font, fontSize, op.CallOp{}, op.CallOp{})
		gtx.Ops.Reset()
	})
}

const (
	latinDocument = `Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
Porttitor eget dolor morbi non arcu risus quis.
Nibh sit amet commodo nulla.
Posuere ac ut consequat semper viverra nam libero justo.
Risus in hendrerit gravida rutrum quisque.
Natoque penatibus et magnis dis parturient montes nascetur.
In metus vulputate eu scelerisque felis imperdiet proin fermentum.
Mattis rhoncus urna neque viverra.
Elit pellentesque habitant morbi tristique.
Nisl nunc mi ipsum faucibus vitae aliquet nec.
Sed augue lacus viverra vitae congue eu consequat.
At quis risus sed vulputate odio ut.
Sit amet volutpat consequat mauris nunc congue nisi.
Dignissim cras tincidunt lobortis feugiat.
Faucibus turpis in eu mi bibendum.
Odio aenean sed adipiscing diam donec adipiscing tristique.
Fermentum leo vel orci porta non pulvinar.
Ut venenatis tellus in metus vulputate eu scelerisque felis imperdiet.
Et netus et malesuada fames ac turpis.
Venenatis urna cursus eget nunc scelerisque viverra mauris in.
Risus ultricies tristique nulla aliquet enim tortor.
Risus pretium quam vulputate dignissim suspendisse in.
Interdum velit euismod in pellentesque massa placerat duis ultricies lacus.
Proin gravida hendrerit lectus a.
Auctor augue mauris augue neque gravida in fermentum et.
Laoreet sit amet cursus sit amet dictum.
In fermentum et sollicitudin ac orci phasellus egestas tellus rutrum.
Tempus imperdiet nulla malesuada pellentesque elit eget gravida.
Consequat id porta nibh venenatis cras sed.
Vulputate ut pharetra sit amet aliquam.
Congue mauris rhoncus aenean vel elit.
Risus quis varius quam quisque id diam vel quam elementum.
Pretium lectus quam id leo in vitae.
Sed sed risus pretium quam vulputate dignissim suspendisse in est.
Velit laoreet id donec ultrices.
Nunc sed velit dignissim sodales ut.
Nunc scelerisque viverra mauris in aliquam sem fringilla ut.
Sed enim ut sem viverra aliquet eget sit.
Convallis posuere morbi leo urna molestie at.
Aliquam id diam maecenas ultricies mi eget mauris.
Ipsum dolor sit amet consectetur adipiscing elit ut aliquam.
Accumsan tortor posuere ac ut consequat semper.
Viverra vitae congue eu consequat ac felis donec et odio.
Scelerisque in dictum non consectetur a.
Consequat nisl vel pretium lectus quam id leo in vitae.
Morbi tristique senectus et netus et malesuada fames ac turpis.
Ac orci phasellus egestas tellus.
Tempus egestas sed sed risus.
Ullamcorper morbi tincidunt ornare massa eget egestas purus.
Nibh venenatis cras sed felis eget velit.`
	arabicDocument = `و سأعرض مثال حي لهذا، من منا لم يتحمل جهد بدني شاق إلا من أجل الحصول على ميزة أو فائدة؟ ولكن من لديه الحق أن ينتقد شخص ما أراد أن يشعر بالسعادة التي لا تشوبها عواقب أليمة أو آخر أراد أن يتجنب الألم الذي ربما تنجم عنه بعض المتعة ؟ علي الجانب الآخر نشجب ونستنكر هؤلاء الرجال المفتونون بنشوة اللحظة الهائمون في رغباتهم فلا يدركون ما يعقبها من الألم والأسي المحتم، واللوم كذلك يشمل هؤلاء الذين أخفقوا في واجباتهم نتيجة لضعف إرادتهم فيتساوي مع هؤلاء الذين يتجنبون وينأون عن تحمل الكدح والألم .
من المفترض أن نفرق بين هذه الحالات بكل سهولة ومرونة.
في ذاك الوقت عندما تكون قدرتنا علي الاختيار غير مقيدة بشرط وعندما لا نجد ما يمنعنا أن نفعل الأفضل فها نحن نرحب بالسرور والسعادة ونتجنب كل ما يبعث إلينا الألم.
في بعض الأحيان ونظراً للالتزامات التي يفرضها علينا الواجب والعمل سنتنازل غالباً ونرفض الشعور بالسرور ونقبل ما يجلبه إلينا الأسى.
الإنسان الحكيم عليه أن يمسك زمام الأمور ويختار إما أن يرفض مصادر السعادة من أجل ما هو أكثر أهمية أو يتحمل الألم من أجل ألا يتحمل ما هو أسوأ.
و سأعرض مثال حي لهذا، من منا لم يتحمل جهد بدني شاق إلا من أجل الحصول على ميزة أو فائدة؟ ولكن من لديه الحق أن ينتقد شخص ما أراد أن يشعر بالسعادة التي لا تشوبها عواقب أليمة أو آخر أراد أن يتجنب الألم الذي ربما تنجم عنه بعض المتعة ؟ علي الجانب الآخر نشجب ونستنكر هؤلاء الرجال المفتونون بنشوة اللحظة الهائمون في رغباتهم فلا يدركون ما يعقبها من الألم والأسي المحتم، واللوم كذلك يشمل هؤلاء الذين أخفقوا في واجباتهم نتيجة لضعف إرادتهم فيتساوي مع هؤلاء الذين يتجنبون وينأون عن تحمل الكدح والألم .
من المفترض أن نفرق بين هذه الحالات بكل سهولة ومرونة.
في ذاك الوقت عندما تكون قدرتنا علي الاختيار غير مقيدة بشرط وعندما لا نجد ما يمنعنا أن نفعل الأفضل فها نحن نرحب بالسرور والسعادة ونتجنب كل ما يبعث إلينا الألم.
في بعض الأحيان ونظراً للالتزامات التي يفرضها علينا الواجب والعمل سنتنازل غالباً ونرفض الشعور بالسرور ونقبل ما يجلبه إلينا الأسى.
الإنسان الحكيم عليه أن يمسك زمام الأمور ويختار إما أن يرفض مصادر السعادة من أجل ما هو أكثر أهمية أو يتحمل الألم من أجل ألا يتحمل ما هو أسوأ.`
	complexDocument = `و سأعرض مثال dolor sit amet, لم يتحمل جهد adipiscing elit, sed do الحصول على ميزة incididunt ut labore أن ينتقد magna aliqua.
Porttitor إرادتهم فيتساوي morbi non arcu يدركون ما يعقبها .
Nibh نشجب ونستنكر commodo nulla.
بكل سهولة ومرونة ut consequat  لهذا، من منا  nam libero justo.
Risus in hendrerit علينا الواجب والعمل.
Natoque تكون قدرتنا علي magnis dis parturient  يمسك زمام الأمور ويختار.
In نجد ما يمنعنا eu scelerisque ونظراً للالتزامات التي fermentum.
Mattis ة بشرط وعندما لا  neque viverra.
يمسك زمام الأمور  habitant لهذا، من.
Nisl تي يفرضها علينا faucibus ،من منا لم nec.
Sed augue علي الاختيار غير vitae congue eu consequat.
At quis risus سك زمام الأمور ويختار.
Sit amet volutpat consequat mauris الأمور ويختار إما nisi.
Dignissim لواجب والعمل tincidunt سنتنازل feugiat.
Faucibus التزامات in eu mi bibendum.
Odio ويختار إما أن يرفض مصادر السعادة sed adipiscing ذا، من منا لم  tristique.
Fermentum leo vel ور ويختار إما  pulvinar.
Ut ر إما أن يرفض مصادر السعادة من in metus  تكون قدرتنا علي  felis imperdiet.
ي الاختيار غير مقيدة بشرط et malesuada fames ac turpis.
Venenatis على ميزة أو فائدة؟ ولكن  eget nunc scelerisque سك زمام الأمور ويختار إما in.
رتنا ultricies tristique ي الاختيار غير مقيدة بشرط enim tortor.
Risus اختيار غير مقيدة بشرط وعندما  quam سان الحكيم عليه أن  suspendisse in.
Interdum velit  ونظراً للالتزامات التي  pellentesque massa placerat لأمور ويختار إما أن يرفض  lacus.
Proin دما تكون قدرتنا علي الاختيار  lectus a.
Auctor  الوقت عندما تكون augue neque ض مثال حي  fermentum et.
Laoreet مسك زمام الأمور ويختار  amet cursus  لم يتحمل جهد  dictum.
In fermentum et sollicitudin ac orci phasellus  علي الاختيار غير  rutrum.
Tempus imperdiet  المفترض أن نفرق  pellentesque ت بكل سهولة eget gravida.
Consequat id portaمصادر السعادة  cras sed.
Vulputate علي الاختيار غير مقيدة sit amet aliquam.
Congue mauris حيان ونظراً للالتزامات التي vel elit.
Risus quis varius quam quisque id ار غير مقيدة بشرط elementum.
Pretium تي يفرضها علينا الواجب leo in vitae.
 شاق إلا من أجل pretium quam الحكيم عليه أن يمسك  suspendisse in est.
Velit ونظراً للالتزامات التي يفرضها ultrices.
 الوقت عندما تكون  velit dignissim يه أن يمسك .
Nunc scelerisque viverra mauris in aliquam sem ر إما أن  ut.
السعادة من أجل ما هو أكثر أهمية أو يتحمل الألم
Convallis posuere morbi leo urna molestie at.`
	emojiDocument = `📚🎶🐰🌷👹🌟 🔰🐲📑🍢🔎 👢💮👷👧💑🐪 📙📜🐎🏠🎠 👧🌼💛🎉💜🎍 🔜💷🐉👘🕟📗🍟 🎆🍚📹💄 🐾🎩💽👘 📒💕👅💽🐩 📷🌌🌚🎣📌. 🍈🍅🔖🍄 🍐🔈🍤🐽 🐹💘🍚👩📡 🎸🏠🔳🏩🌳💣 🔡🔠🕤🔔🎴📕 📼👝🎓🕗💸 📓🌽🍟💵🕗🌒🏉📨 🔀🏉🍴💘🍣💸 🔪🔻🕖🎰 🐲👮🔙🌇🐒🏇 🐝🌚🏫🔀👍 👾🎧🍋🍔👧 💣💞🐴👆🐢🏊📀 🕤🌃🍌🕛🔬. 🏃🍜🍔🐽🎁🏩🎰 📮🍄🐖💕👈 🔠🕡🐊💞🍬📳 🎤🌆🌛🐍🔳 🐄🔇🔱🌇📺👞 💌👍📳🎤🏂 👞🎉🍶📊🔶🌅🐭🕙 🍜📠🎴💒🔶 📀💂🌷👺👙.

📥🕝🎎🐻💘🍇🔤 💠🎇📦👩🍁 👜🍏🔏👎 🔟🌹🌗🎬🔙 🐁📛🐝🐏🐣 🔃🗻🔎🌺👀 📰📮🏩👯🐳🍀🍇 🍨📵🌂📌 👌📐🏨🐉 🍏🍘🔟🎣🔏📠 👤📭🐱📣. 🕓👶🎳📭🔌📃🔧 📟🔰🌂🎈🔣 🔤👍🍤👔🐪 🔨🎼🎊🎪🕝🐬 📴🎶🔈🔐🔘 🐬🐯🕜🎎👴🎃 🎑🐾👏👇🔭 🐥🔙💦🔩🔮 👊🐶👗📕 🐎📹👠🍤 🔢💘📷🐷🐂 🐫💕🕕🍖🔆🎽 👼🎶🌸👻🔷🌰 🔔💉💱🔂👵🔑. 🌁🎪🎌🍘🍏 🌛🍂🔎🕃📧👻 🎍🌔🐦🐻 🔉🎌🌘💉👒 📙💠🔙📰 🌒👏💪🌇💈 🌌📯📂🌀🔁 🏧💷🍀🐐🏈 📢🌏🔷💭 👋🕓🌓🕛🏢👡👋 🍶🐂🍠🔟 👵🏇🔶🕜👎. 👹💉🔌🍳🕗 🐫🌈🔠🐀🎩🎽 👺🔣🔂👪👴🐚🕙 👀🕓🔱🌇 🎻🐘🔐🕕 🌉🔡🐊🍮 💫🎆🎹🐍🎯 🐑🐱🍠🕑🍒.

🎳🐎🔹🎾🐹📖 📘🐒📷🕧🔛 🐾📺🎿🍖💂🕥 🍜🎷🍣👳 🕛📧💶🌑 🌀💣🎎🐛🎪🐒 🎇🌹👺🎆💄📚 🔓🍗📓🎂🌍🌘📢 🍩💞🏂💥🔹📇💴 🐇🕝💹🐣💔🎫 👐🍼🏰🎄🎨👚 👑🔗🍅🐈 🐰🐙🌻👹👆 👬🐧🍬🕡🐽💉 🌅🔉🎤🔁📨🔧🔀🍏 🎼🔛📉🌺. 👖🌔🍢🏂💯 🏁🐰🍉📬🍖 📨💜📮🔕🎣🔩 🔏🕀🏫🎳📵👭👟 💨💃📶🎃 📚🔇🍛👽🐍🐄 🔼👻🍮🍔🍨🏪🐺 📩📜🍨📖 🏢🎉🔢🌚🌀🔊💍 🐟🕚🔴🎿🍞 🌈📤👲🌿🌅🍲📛💍 🐦🔰🐗🐆🎻 👑🕐📔🎁🍙🔪🔭. 🎐🐵🎼🌒🎰🍳🎽 🐻🔉💺🕁🍷 🐛🍬💦📶🔖 🔕🌳💃🌺🔢 💒📒🔘🐸👩 🌺🍈🌀🏁🎢🔖 📈🎸🐖👪 🐅🏁🔹🎬🍖📊🗼 🎬📅💝📀🎐. 🌗📍👇🎠 🌸🐸🐐🍕🐋 💈🌌🐶💤🌻🐞 🍯🌳📌🍮🐻🍝 🕦📯🔱👒 💖🌱🐨🎰🏭🏈 🔳🏩🌟🔭📢📒 🔅💬💓💻💁💂 🔗🍂🏇🌒🌂💩🕢 🔙🌆💞📜🔘👇 🍎🌃🔢🌵🏬 🔄💢🍨📋💇🌄 🍝🍧💂🏮🏁. 🎬🐽🔇🎣🌜🔣 🌍🔒👿🎆🌞🍇🍸 👖🍘🏡🕣 📝🐖💆🎈 👙🔳👙🔩👀🔂 🐤📈💃👗🔌🎾🔭🍴 🌺👛🌵🌕🐺 🎆💼👌👘🍈👛 🎳🐪🕧🏄 💯🍟💂👖🎍 🕀💟🌷💕🐉🐲🎷. 🎍👂📓🌽 🐉🕕🐤🌲📟🔂💷 🎑📛🕠🔹🐚 🍆📹🐚🐵🏇🏢 🍠💱🕦💙🏢🐌🎎 🐄🍨📄🌾🎻🏈 🏇🍪💸🔆💍📢👢 💇🌋👝🕜🌍🐶🎓 🍪📄🐤🎃💖 🔲🕒🍧🌎🐪🌶 🍓👲🔭🍯🌔👌 🔼🐗🗼🍂 🔶🍯🎶🐅🐂💗🐴🐶 📭📰📔👬🏯🕟🐄🍊 💆👞📆🐶🌖🎁👺 🐃💺👊🌿🎌.

🍧🕔👆🔭🕛👇 🐆🔖🎂🐭📗🗼🐐 🌐🎢🌞💛🐚 🌿🎶💎💬🔩 💾🔐🎷🍙🐬🕐 🌏🍄🎾🐎🌽🍓🐳 💥🍎👳📫🐤📼🎾 👨🕃🕞🍯🍲. 💥🎍🔉🎈👻🔵🏬🔸 🔼🍹🔱🔮🕔 🌈💎👜📠 👢🍻🍢🎃👺🌍👰 🍵👃🕠🍎🍑 📜💥📘📌 🔹🔵🍷👅💏 💮💘🐜📠👬📖 🌅🍺🔇🌈👒🔀 🎢🌆💌🍬📱🎰 🌺🍆🔰🏁🍁🎠 🔇🔁🌹🔞🎀🎬🐭🌹 🏬📫🗾🎻📌. 🐠🏣👋👊🐟 👲🔣💻👅🎎 🎇🌲🕑🍨📯 🐜📵💙📷🎒🕔 🎇🏀🔴🐑🌗 🎧🔡👅🕁🏉👛🐬 🕧🐞🎩📓🍆📪 🐼📻👼🌄 🌟🌺🏦🍧🍕🐯 🕕🕦🐤💆🍧💩 🐑📜👏👐🐧🍞👵 👞🌲🍼🔍 🌛🐔🌄🎸🐯.`
)
