package ops

type OpType byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeBlockDef OpType = iota + firstOpIndex
	TypeBlock
	TypeTransform
	TypeLayer
	TypeInvalidate
	TypeImage
	TypeDraw
	TypeColor
	TypeArea
	TypePointerHandler
	TypeKeyHandler
	TypeHideInput
	TypePush
	TypePop
	TypeAux
	TypeClip
)

const (
	TypeBlockDefLen       = 1 + 4 + 4
	TypeBlockLen          = 1 + 4 + 4 + 4
	TypeTransformLen      = 1 + 4*2
	TypeLayerLen          = 1
	TypeRedrawLen         = 1 + 8
	TypeImageLen          = 1 + 4*4
	TypeDrawLen           = 1 + 4*4
	TypeColorLen          = 1 + 4
	TypeAreaLen           = 1 + 1 + 2*4
	TypePointerHandlerLen = 1 + 1
	TypeKeyHandlerLen     = 1 + 1
	TypeHideInputLen      = 1
	TypePushLen           = 1
	TypePopLen            = 1
	TypeAuxLen            = 1 + 4
	TypeClipLen           = 1 + 4*4
)

func (t OpType) Size() int {
	return [...]int{
		TypeBlockDefLen,
		TypeBlockLen,
		TypeTransformLen,
		TypeLayerLen,
		TypeRedrawLen,
		TypeImageLen,
		TypeDrawLen,
		TypeColorLen,
		TypeAreaLen,
		TypePointerHandlerLen,
		TypeKeyHandlerLen,
		TypeHideInputLen,
		TypePushLen,
		TypePopLen,
		TypeAuxLen,
		TypeClipLen,
	}[t-firstOpIndex]
}

func (t OpType) NumRefs() int {
	switch t {
	case TypeBlock, TypeImage, TypeKeyHandler, TypePointerHandler:
		return 1
	default:
		return 0
	}
}
