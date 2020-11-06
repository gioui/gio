// Keep in sync with go.mod; the only difference should be the gioui.org
// replace. To use it:
//
//     cd gogio
//     GOFLAGS=-modfile=../go.local.mod go test

module gioui.org/cmd

go 1.13

require (
	gioui.org v0.0.0-00010101000000-000000000000
	github.com/chromedp/cdproto v0.0.0-20191114225735-6626966fbae4
	github.com/chromedp/chromedp v0.5.2
	golang.org/x/image v0.0.0-20200618115811-c13761719519
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/tools v0.0.0-20190927191325-030b2cf1153e
)

replace gioui.org => ../
