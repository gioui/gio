// Package transfer contains operations and events for brokering data transfers.
//
// The transfer protocol is as follows:
//
//   - Data sources use [SourceFilter] to receive a [RequestEvent] when data is
//     requested. Sources respond to requests with [OfferCmd].
//   - Data targets use [TargetFilter] to receive data through a [DataEvent].
//     The target must close the reader returned by [DataEvent.Open] after use.
//
// When a user initiates a pointer-guided drag and drop transfer, each
// potential target receives an InitiateEvent with the MIME types shared by
// that target and the source. During an in-app drag, potential targets receive
// Enter, Drag and Leave pointer events, and other handlers are canceled unless
// they hold a pressed pointer. When a drag gesture completes, a CancelEvent is
// sent to the source and potential targets.
//
// For a platform drag, the source is requested to supply the representations
// for the platform at drag start. The destination receives the platform drop,
// even when it is in the source window. With local routing the source is
// requested only when the drag is released on a matching target.
//
// Drags from other applications send InitiateEvent, then DataEvent or
// CancelEvent. DataEvent.Position identifies the drop location; there is no
// source to receive a RequestEvent because the platform supplies the data.
// Files dropped from another application use the MIME type "text/uri-list".
// [DataEvent.Open] returns file URIs separated by CRLF, not file contents.
//
// Windows, macOS, and Wayland use platform drags for the primary mouse
// button. Touch and platforms without a native drag source use local routing.
package transfer

import (
	"io"

	"gioui.org/f32"
	"gioui.org/io/event"
)

// OfferCmd is used by data sources as a response to a RequestEvent.
type OfferCmd struct {
	Tag event.Tag
	// Type is the MIME type of Data.
	// It must be the Type from the corresponding RequestEvent.
	Type string
	// Data contains the offered data. It is closed when the
	// transfer is complete or cancelled.
	// Data must be kept valid until closed, and it may be used from
	// a goroutine separate from the one processing the frame.
	Data io.ReadCloser
}

func (OfferCmd) ImplementsCommand() {}

// SourceFilter filters for any [RequestEvent] that match a MIME type
// as well as [InitiateEvent] and [CancelEvent].
// Use multiple filters to offer multiple types.
type SourceFilter struct {
	// Target is a tag included in a previous event.Op.
	Target event.Tag
	// Type is the MIME type supported by this source.
	Type string
}

// TargetFilter filters for any [DataEvent] whose type matches a MIME type
// as well as [InitiateEvent] and [CancelEvent]. Use multiple filters to accept
// multiple types.
type TargetFilter struct {
	// Target is a tag included in a previous event.Op.
	Target event.Tag
	// Type is the MIME type accepted by this target.
	Type string
}

// RequestEvent requests data from a data source. The source must
// respond with an OfferCmd.
type RequestEvent struct {
	// Type is the first matched type between the source and the target.
	Type string
}

func (RequestEvent) ImplementsEvent() {}

// InitiateEvent is sent to potential data targets when a drag-and-drop
// transfer gesture is initiated.
type InitiateEvent struct {
	// Types lists the MIME types offered by the source. When delivered to
	// a target, it contains only the types also accepted by that target.
	Types []string
}

func (InitiateEvent) ImplementsEvent() {}

// CancelEvent is sent to data sources and targets to cancel the
// effects of an InitiateEvent.
type CancelEvent struct{}

func (CancelEvent) ImplementsEvent() {}

// DataEvent is sent to the target receiving the transfer.
type DataEvent struct {
	// ExternalDrop reports data delivered by the platform drag-and-drop system,
	// including a drop from another Gio window in the same process.
	ExternalDrop bool
	// Position is the platform-reported drop position in window coordinates,
	// measured in pixels.
	Position f32.Point
	// Type is the MIME type of Data.
	Type string
	// Open returns the transfer data. It is only valid to call Open in the frame
	// the DataEvent is received. The caller must close the return value after use.
	Open func() io.ReadCloser
}

func (DataEvent) ImplementsEvent() {}

func (SourceFilter) ImplementsFilter() {}
func (TargetFilter) ImplementsFilter() {}
