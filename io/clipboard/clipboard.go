// SPDX-License-Identifier: Unlicense OR MIT

package clipboard

import (
	"io"

	"gioui.org/io/event"
)

// WriteCmd copies Text to the clipboard.
type WriteCmd struct {
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

// ReadCmd requests the text of the clipboard, delivered to
// the handler through an [io/transfer.DataEvent].
type ReadCmd struct {
	Tag event.Tag
}

func (WriteCmd) ImplementsCommand() {}
func (ReadCmd) ImplementsCommand()  {}
