package ops

type OpType byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeMacroDef OpType = iota + firstOpIndex
	TypeMacro
	TypeTransform
	TypeLayer
	TypeInvalidate
	TypeImage
	TypeDraw
	TypeColor
	TypeArea
	TypePointerHandler
	TypePass
	TypeKeyHandler
	TypeHideInput
	TypePush
	TypePop
	TypeAux
	TypeClip
	TypeProfile
)

const (
	TypeMacroDefLen       = 1 + 4 + 4
	TypeMacroLen          = 1 + 4 + 4 + 4
	TypeTransformLen      = 1 + 4*2
	TypeLayerLen          = 1
	TypeRedrawLen         = 1 + 8
	TypeImageLen          = 1 + 4*4
	TypeDrawLen           = 1 + 4*4
	TypeColorLen          = 1 + 4
	TypeAreaLen           = 1 + 1 + 4*4
	TypePointerHandlerLen = 1 + 1
	TypePassLen           = 1 + 1
	TypeKeyHandlerLen     = 1 + 1
	TypeHideInputLen      = 1
	TypePushLen           = 1
	TypePopLen            = 1
	TypeAuxLen            = 1 + 4
	TypeClipLen           = 1 + 4*4
	TypeProfileLen        = 1
)

func (t OpType) Size() int {
	return [...]int{
		TypeMacroDefLen,
		TypeMacroLen,
		TypeTransformLen,
		TypeLayerLen,
		TypeRedrawLen,
		TypeImageLen,
		TypeDrawLen,
		TypeColorLen,
		TypeAreaLen,
		TypePointerHandlerLen,
		TypePassLen,
		TypeKeyHandlerLen,
		TypeHideInputLen,
		TypePushLen,
		TypePopLen,
		TypeAuxLen,
		TypeClipLen,
		TypeProfileLen,
	}[t-firstOpIndex]
}

func (t OpType) NumRefs() int {
	switch t {
	case TypeMacro, TypeImage, TypeKeyHandler, TypePointerHandler, TypeProfile:
		return 1
	default:
		return 0
	}
}
