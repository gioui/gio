package ops

type OpType byte

const (
	TypeBlockDef OpType = iota
	TypeBlock
	TypeTransform
	TypeLayer
	TypeRedraw
	TypeClip
	TypeImage
	TypeDraw
	TypeColor
	TypePointerHandler
	TypeKeyHandler
	TypeHideInput
	TypePush
	TypePop
)

const (
	TypeBlockDefLen       = 1 + 4
	TypeBlockLen          = 1 + 4
	TypeTransformLen      = 1 + 4*2
	TypeLayerLen          = 1
	TypeRedrawLen         = 1 + 8
	TypeClipLen           = 1 + 4
	TypeImageLen          = 1 + 4 + 4*4
	TypeDrawLen           = 1 + 4*4
	TypeColorLen          = 1 + 4
	TypePointerHandlerLen = 1 + 4 + 4 + 1
	TypeKeyHandlerLen     = 1 + 4 + 1
	TypeHideInputLen      = 1
	TypePushLen           = 1
	TypePopLen            = 1
)
