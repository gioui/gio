// SPDX-License-Identifier: Unlicense OR MIT

package clipboard

import (
	"io"

	"gioui.org/io/event"
)

// Content is one clipboard representation. Type is the MIME type of Data.
type Content struct {
	Type string
	Data []byte
}

// WriteCmd copies Data to the clipboard.
type WriteCmd struct {
	// Type is the MIME type of Data. All platforms support "application/text"
	// and "text/plain". Windows also supports "image/png", "text/html",
	// "application/rtf", and "text/rtf".
	Type string
	Data io.ReadCloser
	// Sensitive marks Data as a secret, such as a password. Where the
	// platform has a convention for it, the clipboard content is marked so
	// that clipboard managers and system features do not keep or show it:
	//
	//   - X11 and Wayland offer the x-kde-passwordManagerHint type with the
	//     data "secret", which KDE's Klipper honours.
	//   - macOS adds the org.nspasteboard.ConcealedType and
	//     org.nspasteboard.TransientType types of the nspasteboard.org
	//     convention.
	//   - Windows adds the registered formats
	//     ExcludeClipboardContentFromMonitorProcessing,
	//     CanIncludeInClipboardHistory (0) and CanUploadToCloudClipboard
	//     (0), which keep the content out of clipboard monitors, the
	//     clipboard history and the cloud clipboard.
	//   - Android sets the android.content.extra.IS_SENSITIVE extra of the
	//     clip, which hides the content in the clipboard preview of Android
	//     13 and later.
	//   - iOS writes the content with UIPasteboardOptionLocalOnly, which
	//     keeps it out of Universal Clipboard.
	//   - The browser's clipboard API has no such mark, so Sensitive has no
	//     effect in a browser.
	//
	// Sensitive does not clear the clipboard later; that is up to the app.
	Sensitive bool
}

// WriteContentsCmd replaces the clipboard with all the supplied
// representations at once. See [WriteCmd] for supported types.
// Each Type should occur at most once. An empty Contents slice clears the clipboard.
type WriteContentsCmd struct {
	Contents []Content
	// Sensitive marks all contents as secret. See [WriteCmd.Sensitive].
	Sensitive bool
}

// ReadContentsCmd requests several clipboard representations at once. The
// target receives one [io/transfer.DataEvent] for each requested type.
// Unsupported types produce an empty reader. An empty Types slice requests
// "application/text".
type ReadContentsCmd struct {
	Tag   event.Tag
	Types []string
}

// ReadCmd requests clipboard data, delivered through an [io/transfer.DataEvent].
// If the requested type is unavailable, the event contains an empty reader.
type ReadCmd struct {
	Tag event.Tag
	// Type is the MIME type to read. An empty Type requests "application/text".
	// Supported types are the same as for [WriteCmd].
	Type string
}

func (WriteCmd) ImplementsCommand()         {}
func (WriteContentsCmd) ImplementsCommand() {}
func (ReadContentsCmd) ImplementsCommand()  {}
func (ReadCmd) ImplementsCommand()          {}
