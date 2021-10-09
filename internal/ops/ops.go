// SPDX-License-Identifier: Unlicense OR MIT

package ops

import (
	"encoding/binary"
	"math"

	"gioui.org/f32"
	"gioui.org/internal/byteslice"
	"gioui.org/internal/scene"
)

type Ops struct {
	// version is incremented at each Reset.
	version int
	// data contains the serialized operations.
	data []byte
	// refs hold external references for operations.
	refs []interface{}
	// nextStateID is the id allocated for the next
	// StateOp.
	nextStateID int

	macroStack stack
	stacks     [4]stack
}

type OpType byte

// Start at a high number for easier debugging.
const firstOpIndex = 200

const (
	TypeMacro OpType = iota + firstOpIndex
	TypeCall
	TypeDefer
	TypePushTransform
	TypeTransform
	TypePopTransform
	TypeInvalidate
	TypeImage
	TypePaint
	TypeColor
	TypeLinearGradient
	TypeArea
	TypePopArea
	TypePass
	TypePopPass
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

type StackID struct {
	id   int
	prev int
}

// StateOp represents a saved operation snapshop to be restored
// later.
type StateOP struct {
	id      int
	macroID int
	ops     *Ops
}

// stack tracks the integer identities of stack operations to ensure correct
// pairing of their push and pop methods.
type stack struct {
	currentID int
	nextID    int
}

type StackKind uint8

const (
	ClipStack StackKind = iota
	AreaStack
	TransStack
	PassStack
)

const (
	TypeMacroLen           = 1 + 4 + 4
	TypeCallLen            = 1 + 4 + 4
	TypeDeferLen           = 1
	TypePushTransformLen   = 1 + 4*6
	TypeTransformLen       = 1 + 1 + 4*6
	TypePopTransformLen    = 1
	TypeRedrawLen          = 1 + 8
	TypeImageLen           = 1
	TypePaintLen           = 1
	TypeColorLen           = 1 + 4
	TypeLinearGradientLen  = 1 + 8*2 + 4*2
	TypeAreaLen            = 1 + 1 + 4*4
	TypePopAreaLen         = 1
	TypePassLen            = 1
	TypePopPassLen         = 1
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

func (o *Ops) Reset() {
	o.macroStack = stack{}
	for i := range o.stacks {
		o.stacks[i] = stack{}
	}
	// Leave references to the GC.
	for i := range o.refs {
		o.refs[i] = nil
	}
	o.data = o.data[:0]
	o.refs = o.refs[:0]
	o.nextStateID = 0
	o.version++
}

func (o *Ops) Data() []byte {
	return o.data
}

func (o *Ops) Refs() []interface{} {
	return o.refs
}

func (o *Ops) Version() int {
	return o.version
}

func (o *Ops) Write(n int) []byte {
	o.data = append(o.data, make([]byte, n)...)
	return o.data[len(o.data)-n:]
}

func (o *Ops) PushMacro() StackID {
	return o.macroStack.push()
}

func (o *Ops) PopMacro(id StackID) {
	o.macroStack.pop(id)
}

func (o *Ops) FillMacro(startPC PC) {
	pc := o.PC()
	// Fill out the macro definition reserved in Record.
	data := o.data[startPC.data:]
	data = data[:TypeMacroLen]
	data[0] = byte(TypeMacro)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
}

func (o *Ops) AddCall(callOps *Ops, pc PC) {
	data := o.Write1(TypeCallLen, callOps)
	data[0] = byte(TypeCall)
	bo := binary.LittleEndian
	bo.PutUint32(data[1:], uint32(pc.data))
	bo.PutUint32(data[5:], uint32(pc.refs))
}

func (o *Ops) PushOp(kind StackKind) (StackID, int) {
	return o.stacks[kind].push(), o.macroStack.currentID
}

func (o *Ops) PopOp(kind StackKind, sid StackID, macroID int) {
	if o.macroStack.currentID != macroID {
		panic("stack push and pop must not cross macro boundary")
	}
	o.stacks[kind].pop(sid)
}

func (o *Ops) Write1(n int, ref1 interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, ref1)
	return o.data[len(o.data)-n:]
}

func (o *Ops) Write2(n int, ref1, ref2 interface{}) []byte {
	o.data = append(o.data, make([]byte, n)...)
	o.refs = append(o.refs, ref1, ref2)
	return o.data[len(o.data)-n:]
}

func (o *Ops) PC() PC {
	return PC{data: len(o.data), refs: len(o.refs)}
}

func (s *stack) push() StackID {
	s.nextID++
	sid := StackID{
		id:   s.nextID,
		prev: s.currentID,
	}
	s.currentID = s.nextID
	return sid
}

func (s *stack) check(sid StackID) {
	if s.currentID != sid.id {
		panic("unbalanced operation")
	}
}

func (s *stack) pop(sid StackID) {
	s.check(sid)
	s.currentID = sid.prev
}

// Save the effective transformation.
func (o *Ops) Save() StateOP {
	o.nextStateID++
	s := StateOP{
		ops:     o,
		id:      o.nextStateID,
		macroID: o.macroStack.currentID,
	}
	bo := binary.LittleEndian
	data := o.Write(TypeSaveLen)
	data[0] = byte(TypeSave)
	bo.PutUint32(data[1:], uint32(s.id))
	return s
}

// load a previously saved operations state given
// its ID.
func (s StateOP) Load() {
	bo := binary.LittleEndian
	data := s.ops.Write(TypeLoadLen)
	data[0] = byte(TypeLoad)
	bo.PutUint32(data[1:], uint32(s.id))
}

func DecodeCommand(d []byte) scene.Command {
	var cmd scene.Command
	copy(byteslice.Uint32(cmd[:]), d)
	return cmd
}

func EncodeCommand(out []byte, cmd scene.Command) {
	copy(out, byteslice.Uint32(cmd[:]))
}

func DecodeTransform(data []byte) (t f32.Affine2D, push bool) {
	if OpType(data[0]) != TypeTransform {
		panic("invalid op")
	}
	push = data[1] != 0
	data = data[2:]
	data = data[:4*6]

	bo := binary.LittleEndian
	a := math.Float32frombits(bo.Uint32(data))
	b := math.Float32frombits(bo.Uint32(data[4*1:]))
	c := math.Float32frombits(bo.Uint32(data[4*2:]))
	d := math.Float32frombits(bo.Uint32(data[4*3:]))
	e := math.Float32frombits(bo.Uint32(data[4*4:]))
	f := math.Float32frombits(bo.Uint32(data[4*5:]))
	return f32.NewAffine2D(a, b, c, d, e, f), push
}

// DecodeSave decodes the state id of a save op.
func DecodeSave(data []byte) int {
	if OpType(data[0]) != TypeSave {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	return int(bo.Uint32(data[1:]))
}

// DecodeLoad decodes the state id of a load op.
func DecodeLoad(data []byte) int {
	if OpType(data[0]) != TypeLoad {
		panic("invalid op")
	}
	bo := binary.LittleEndian
	return int(bo.Uint32(data[1:]))
}

func (t OpType) Size() int {
	return [...]int{
		TypeMacroLen,
		TypeCallLen,
		TypeDeferLen,
		TypePushTransformLen,
		TypeTransformLen,
		TypePopTransformLen,
		TypeRedrawLen,
		TypeImageLen,
		TypePaintLen,
		TypeColorLen,
		TypeLinearGradientLen,
		TypeAreaLen,
		TypePopAreaLen,
		TypePassLen,
		TypePopPassLen,
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
	case TypePushTransform:
		return "PushTransform"
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
	case TypePass:
		return "Pass"
	case TypePopPass:
		return "PopPass"
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
