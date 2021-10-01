// SPDX-License-Identifier: Unlicense OR MIT

package opconst

type OpType byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeMacro OpType = iota + firstOpIndex
	TypeCall
	TypeDefer
	TypeTransform
	TypePopTransform
	TypeInvalidate
	TypeImage
	TypePaint
	TypeColor
	TypeLinearGradient
	TypeArea
	TypePopArea
	TypePointerInput
	TypeClipboardRead
	TypeClipboardWrite
	TypeKeyInput
	TypeKeyFocus
	TypeKeySoftKeyboard
	TypeSave
	TypeLoad
	TypeAux
	TypeClip
	TypePopClip
	TypeProfile
	TypeCursor
	TypePath
	TypeStroke
)

const (
	TypeMacroLen           = 1 + 4 + 4
	TypeCallLen            = 1 + 4 + 4
	TypeDeferLen           = 1
	TypeTransformLen       = 1 + 1 + 4*6
	TypePopTransformLen    = 1
	TypeRedrawLen          = 1 + 8
	TypeImageLen           = 1
	TypePaintLen           = 1
	TypeColorLen           = 1 + 4
	TypeLinearGradientLen  = 1 + 8*2 + 4*2
	TypeAreaLen            = 1 + 1 + 1 + 4*4
	TypePopAreaLen         = 1
	TypePointerInputLen    = 1 + 1 + 1 + 2*4 + 2*4
	TypeClipboardReadLen   = 1
	TypeClipboardWriteLen  = 1
	TypeKeyInputLen        = 1 + 1
	TypeKeyFocusLen        = 1 + 1
	TypeKeySoftKeyboardLen = 1 + 1
	TypeSaveLen            = 1 + 4
	TypeLoadLen            = 1 + 4
	TypeAuxLen             = 1
	TypeClipLen            = 1 + 4*4 + 1 + 1
	TypePopClipLen         = 1
	TypeProfileLen         = 1
	TypeCursorLen          = 1 + 1
	TypePathLen            = 8 + 1
	TypeStrokeLen          = 1 + 4
)

func (t OpType) Size() int {
	return [...]int{
		TypeMacroLen,
		TypeCallLen,
		TypeDeferLen,
		TypeTransformLen,
		TypePopTransformLen,
		TypeRedrawLen,
		TypeImageLen,
		TypePaintLen,
		TypeColorLen,
		TypeLinearGradientLen,
		TypeAreaLen,
		TypePopAreaLen,
		TypePointerInputLen,
		TypeClipboardReadLen,
		TypeClipboardWriteLen,
		TypeKeyInputLen,
		TypeKeyFocusLen,
		TypeKeySoftKeyboardLen,
		TypeSaveLen,
		TypeLoadLen,
		TypeAuxLen,
		TypeClipLen,
		TypePopClipLen,
		TypeProfileLen,
		TypeCursorLen,
		TypePathLen,
		TypeStrokeLen,
	}[t-firstOpIndex]
}

func (t OpType) NumRefs() int {
	switch t {
	case TypeKeyInput, TypeKeyFocus, TypePointerInput, TypeProfile, TypeCall, TypeClipboardRead, TypeClipboardWrite, TypeCursor:
		return 1
	case TypeImage:
		return 2
	default:
		return 0
	}
}

func (t OpType) String() string {
	switch t {
	case TypeMacro:
		return "Macro"
	case TypeCall:
		return "Call"
	case TypeDefer:
		return "Defer"
	case TypeTransform:
		return "Transform"
	case TypePopTransform:
		return "PopTransform"
	case TypeInvalidate:
		return "Invalidate"
	case TypeImage:
		return "Image"
	case TypePaint:
		return "Paint"
	case TypeColor:
		return "Color"
	case TypeLinearGradient:
		return "LinearGradient"
	case TypeArea:
		return "Area"
	case TypePopArea:
		return "PopArea"
	case TypePointerInput:
		return "PointerInput"
	case TypeClipboardRead:
		return "ClipboardRead"
	case TypeClipboardWrite:
		return "ClipboardWrite"
	case TypeKeyInput:
		return "KeyInput"
	case TypeKeyFocus:
		return "KeyFocus"
	case TypeKeySoftKeyboard:
		return "KeySoftKeyboard"
	case TypeSave:
		return "Save"
	case TypeLoad:
		return "Load"
	case TypeAux:
		return "Aux"
	case TypeClip:
		return "Clip"
	case TypePopClip:
		return "PopClip"
	case TypeProfile:
		return "Profile"
	case TypeCursor:
		return "Cursor"
	case TypePath:
		return "Path"
	case TypeStroke:
		return "Stroke"
	default:
		panic("unnkown OpType")
	}
}
