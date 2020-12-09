// SPDX-License-Identifier: Unlicense OR MIT

package opconst

type OpType byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeMacro OpType = iota + firstOpIndex
	TypeCall
	TypeTransform
	TypeLayer
	TypeInvalidate
	TypeImage
	TypePaint
	TypeColor
	TypeLinearGradient
	TypeArea
	TypePointerInput
	TypePass
	TypeClipboardRead
	TypeClipboardWrite
	TypeKeyInput
	TypeKeyFocus
	TypeKeySoftKeyboard
	TypePush
	TypePop
	TypeAux
	TypeClip
	TypeProfile
	TypeCursor
	TypePath
	TypeStroke
	TypeDash
)

const (
	TypeMacroLen           = 1 + 4 + 4
	TypeCallLen            = 1 + 4 + 4
	TypeTransformLen       = 1 + 4*6
	TypeLayerLen           = 1
	TypeRedrawLen          = 1 + 8
	TypeImageLen           = 1
	TypePaintLen           = 1
	TypeColorLen           = 1 + 4
	TypeLinearGradientLen  = 1 + 8*2 + 4*2
	TypeAreaLen            = 1 + 1 + 4*4
	TypePointerInputLen    = 1 + 1 + 1
	TypePassLen            = 1 + 1
	TypeClipboardReadLen   = 1
	TypeClipboardWriteLen  = 1
	TypeKeyInputLen        = 1
	TypeKeyFocusLen        = 1 + 1
	TypeKeySoftKeyboardLen = 1 + 1
	TypePushLen            = 1
	TypePopLen             = 1
	TypeAuxLen             = 1
	TypeClipLen            = 1 + 4*4 + 1
	TypeProfileLen         = 1
	TypeCursorLen          = 1 + 1
	TypePathLen            = 1 + 4
	TypeStrokeLen          = 1 + 4 + 4 + 1 + 1
	TypeDashLen            = 1 + 4 + 1
)

func (t OpType) Size() int {
	return [...]int{
		TypeMacroLen,
		TypeCallLen,
		TypeTransformLen,
		TypeLayerLen,
		TypeRedrawLen,
		TypeImageLen,
		TypePaintLen,
		TypeColorLen,
		TypeLinearGradientLen,
		TypeAreaLen,
		TypePointerInputLen,
		TypePassLen,
		TypeClipboardReadLen,
		TypeClipboardWriteLen,
		TypeKeyInputLen,
		TypeKeyFocusLen,
		TypeKeySoftKeyboardLen,
		TypePushLen,
		TypePopLen,
		TypeAuxLen,
		TypeClipLen,
		TypeProfileLen,
		TypeCursorLen,
		TypePathLen,
		TypeStrokeLen,
		TypeDashLen,
	}[t-firstOpIndex]
}

func (t OpType) NumRefs() int {
	switch t {
	case TypeKeyInput, TypePointerInput, TypeProfile, TypeCall, TypeClipboardRead, TypeClipboardWrite, TypeCursor:
		return 1
	case TypeImage:
		return 2
	default:
		return 0
	}
}
