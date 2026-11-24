// SPDX-License-Identifier: Unlicense OR MIT

package clipboard

import "gioui.org/io/event"

// Content is one clipboard representation. Type is the MIME type of Data.
type Content struct {
	Type string
	Data []byte
}

// WriteCmd replaces the clipboard with all the supplied representations.
type WriteCmd struct {
	// Each MIME type should occur at most once. An empty slice clears the clipboard.
	// All platforms support "application/text" and "text/plain".
	// Windows also supports "image/png", "text/html",
	// "application/rtf", and "text/rtf".
	Contents []Content
	// Sensitive marks the contents as a secret, such as a password. Where the
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

// ReadCmd requests clipboard data. The target receives one [gioui.org/io/transfer.DataEvent]
// for all requested types. Opening an unavailable representation yields an empty reader.
type ReadCmd struct {
	Tag event.Tag
	// Types lists the MIME types to read. An empty slice requests "application/text".
	// Supported types are the same as for [WriteCmd].
	Types []string
}

func (WriteCmd) ImplementsCommand() {}
func (ReadCmd) ImplementsCommand()  {}
