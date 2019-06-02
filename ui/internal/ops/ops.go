package ops

type OpType byte

// Start at a high number for easier debugging.
const FirstOpIndex = 200

const (
	TypeBlockDef OpType = iota + FirstOpIndex
	TypeBlock
	TypeTransform
	TypeLayer
	TypeRedraw
	TypeImage
	TypeDraw
	TypeColor
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
	TypePointerHandlerLen = 1 + 1
	TypeKeyHandlerLen     = 1 + 1
	TypeHideInputLen      = 1
	TypePushLen           = 1
	TypePopLen            = 1
	TypeAuxLen            = 1 + 4
	TypeClipLen           = 1 + 4*4

	TypeBlockDefRefs       = 0
	TypeBlockRefs          = 1
	TypeTransformRefs      = 0
	TypeLayerRefs          = 0
	TypeRedrawRefs         = 0
	TypeImageRefs          = 1
	TypeDrawRefs           = 0
	TypeColorRefs          = 0
	TypePointerHandlerRefs = 2
	TypeKeyHandlerRefs     = 1
	TypeHideInputRefs      = 0
	TypePushRefs           = 0
	TypePopRefs            = 0
	TypeAuxRefs            = 0
	TypeClipRefs           = 0
)
