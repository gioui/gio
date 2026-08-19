package ibus

import (
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gioui.org/internal/debug"
	"gioui.org/io/key"

	"github.com/godbus/dbus/v5"
)

type EventHandler interface {
	Event(new string, sel key.Range, visible, preedit bool)
}

const inputContextIface = "org.freedesktop.IBus.InputContext"

type IBus struct {
	inputContextObject   dbus.BusObject
	ibusConn             *dbus.Conn
	h                    EventHandler
	visible, compositing bool
}

var imeLogger = log.New(log.Writer(), "[ime] ", log.Default().Flags())

func imeLogf(format string, args ...any) {
	if debug.Ime.Load() {
		imeLogger.Printf(format, args...)
	}
}

func imeLogerr(format string, args ...any) error {
	err := fmt.Errorf(format, args...)
	imeLogf("%v", err.Error())
	return err
}

// Start connects to IBus and creates an IBus object.
func Start(displayStr string, h EventHandler) (*IBus, error) {
	const ibusAddrEnvName = "IBUS_ADDRESS"

	addr := os.Getenv(ibusAddrEnvName)
	if addr != "" {
		return startWithAddr(addr, h)
	}

	var machineId string
	{
		buf, err := os.ReadFile("/etc/machine-id")
		if err != nil {
			return nil, imeLogerr("error reading machine-id: %v\n", err)
		}
		machineId = strings.TrimSpace(string(buf))
	}

	var disp string
	{
		hostName, screenStr, ok := strings.Cut(displayStr, ":")
		if !ok {
			return nil, imeLogerr("error reading DISPLAY env: %q\n", displayStr)
		}
		if hostName != "" {
			return nil, nil
		}

		disp, _, ok = strings.Cut(screenStr, ".")

		_, err := strconv.Atoi(disp)
		if err != nil {
			return nil, imeLogerr("error reading DISPLAY env: %q\n", displayStr)
		}
	}

	var xdgconf string
	{
		xdgconf = os.Getenv("XDG_CONFIG_DIR")
		if xdgconf == "" {
			xdgconf = os.ExpandEnv("$HOME/.config/")
		}
	}

	path := filepath.Join(xdgconf, "ibus", "bus", fmt.Sprintf("%s-unix-%s", machineId, disp))
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, imeLogerr("error reading ibus bus at %q: %v", path, err)
	}

	for _, line := range strings.Split(string(buf), "\n") {
		const pfx = ibusAddrEnvName + "="
		if strings.HasPrefix(line, pfx) {
			return startWithAddr(line[len(pfx):], h)
		}
	}

	return nil, imeLogerr("no iBus address in %s", path)
}

func startWithAddr(addr string, h EventHandler) (*IBus, error) {
	ibus := &IBus{h: h}
	conn, err := dbus.Connect(addr, dbus.WithSignalHandler((*messageHandler)(ibus)))
	if err != nil {
		return nil, imeLogerr("could not connect to ibus addr %q: %v", addr, err)
	}
	obj := conn.Object("org.freedesktop.IBus", "/org/freedesktop/IBus")
	var inputContextPath string
	err = obj.Call("org.freedesktop.IBus.CreateInputContext", 0, "gioui").Store(&inputContextPath)
	if err != nil {
		conn.Close()
		return nil, imeLogerr("ibus: could not create input context: %v\n", err)
	}
	slash := strings.LastIndex(inputContextPath, "/")
	if slash < 0 {
		conn.Close()
		return nil, imeLogerr("ibus: invalid return value for CreateInputContext: %q\n", inputContextPath)
	}
	ibus.inputContextObject = conn.Object("org.freedesktop.IBus", dbus.ObjectPath(inputContextPath))

	const (
		IBUS_CAP_PREEDIT_TEXT = 1 << 0
		IBUS_CAP_FOCUS        = 1 << 3
	)
	err = ibus.inputContextObject.Call(inputContextIface+".SetCapabilities", 0, uint32(IBUS_CAP_PREEDIT_TEXT|IBUS_CAP_FOCUS)).Err
	if err != nil {
		conn.Close()
		return nil, imeLogerr("ibus: SetCapabilities: %v", err)
	}

	err = conn.AddMatchSignal(dbus.WithMatchObjectPath(dbus.ObjectPath(inputContextPath)))
	if err != nil {
		conn.Close()
		return nil, imeLogerr("ibus: could not match signal: %v", err)
	}

	ibus.ibusConn = conn

	ibus.Reset()
	return ibus, nil
}

type messageHandler IBus

func getTextArg(signal *dbus.Signal) (string, key.Range, bool) {
	if len(signal.Body) == 0 {
		return "", key.Range{-1, -1}, false
	}
	body, ok := signal.Body[0].(dbus.Variant)
	if !ok {
		return "", key.Range{-1, -1}, false
	}
	args, ok := body.Value().([]any)
	if !ok || len(args) <= 2 {
		return "", key.Range{-1, -1}, false
	}
	sel := getSelectionRangeMaybe(args)
	text, ok := args[2].(string)
	return text, sel, ok
}

func getVisible(signal *dbus.Signal) bool {
	if len(signal.Body) == 0 || len(signal.Body) < 3 {
		return false
	}
	body, _ := signal.Body[2].(bool)
	return body
}

func getSelectionRangeMaybe(args []any) key.Range {
	none := key.Range{-1, -1}
	if len(args) <= 3 {
		return none
	}
	attrlistv, ok := args[3].(dbus.Variant)
	if !ok {
		return none
	}
	attrlistVariant, ok := attrlistv.Value().([]any)
	if !ok {
		return none
	}
	if len(attrlistVariant) < 3 {
		return none
	}
	attrlist, ok := attrlistVariant[2].([]dbus.Variant)
	if !ok {
		return none
	}
	cand := none
	for _, attr := range attrlist {
		v := ibusAttr(attr)
		if v == nil {
			continue
		}
		// IBus IMEs can send various text stylings including different kinds of
		// underlines, and foreground and background colors. In practice no client
		// respects this and we can't accomodate it either. Look for a double
		// underlined part to highlight.
		const (
			IBUS_ATTR_TYPE_UNDERLINE   = 1
			IBUS_ATTR_UNDERLINE_DOUBLE = 2
		)
		if v[0] == IBUS_ATTR_TYPE_UNDERLINE && v[1] == IBUS_ATTR_UNDERLINE_DOUBLE {
			if cand != none {
				return none
			}
			cand = key.Range{int(v[2]), int(v[3])}
		}
	}
	return cand
}

func ibusAttr(a any) []uint32 {
	v, ok := a.(dbus.Variant)
	if !ok {
		return nil
	}
	vv, ok := v.Value().([]any)
	if !ok {
		return nil
	}
	r := []uint32{}
	for i := 2; i < len(vv); i++ {
		n, ok := vv[i].(uint32)
		if !ok {
			return nil
		}
		r = append(r, n)
	}
	if len(r) != 4 {
		return nil
	}
	return r
}

func (h *messageHandler) DeliverSignal(iface, name string, signal *dbus.Signal) {
	if iface != inputContextIface {
		return
	}

	ibus := (*IBus)(h)

	if h.ibusConn == nil || !h.compositing {
		return
	}

	switch name {
	case "UpdatePreeditText":
		if text, sel, ok := getTextArg(signal); ok {
			visible := getVisible(signal)
			if !visible {
				text = ""
			}
			if !ibus.visible && !visible {
				return
			}
			ibus.h.Event(text, sel, visible, true)
			ibus.visible = visible
		}
	case "HidePreeditText":
		if !ibus.visible {
			return
		}
		ibus.h.Event("", key.Range{-1, -1}, false, false)
		ibus.visible = false
	case "CommitText":
		if text, _, ok := getTextArg(signal); ok {
			ibus.h.Event(text, key.Range{-1, -1}, true, false)
		}
	}
}

// SetCursorLocation sends the position of the cursor to IBus, to be used to
// position the suggestion window.
// The position must be expressed in root window coordinates.
func (ibus *IBus) SetCursorLocation(pos image.Rectangle) {
	if ibus == nil || ibus.ibusConn == nil {
		return
	}
	ibus.inputContextObject.Call(inputContextIface+".SetCursorLocation", 0, int(pos.Min.X), int(pos.Min.Y), pos.Dx(), pos.Dy())
}

// Stop closes the connection to IBus
func (ibus *IBus) Stop() {
	if ibus == nil || ibus.ibusConn == nil {
		return
	}

	ibus.ibusConn.Close()
	ibus.ibusConn = nil
}

// ProcessKey sends a key event to IBus, returns true if the event was handled
func (ibus *IBus) ProcessKey(sym, keycode, state uint32) bool {
	if ibus == nil || ibus.ibusConn == nil {
		return false
	}

	var out bool
	ibus.inputContextObject.Call(inputContextIface+".ProcessKeyEvent", 0, sym, keycode, state).Store(&out)
	if out {
		ibus.compositing = true
	}
	return out
}

// Focused copies the focused status to IBus
func (ibus *IBus) Focused(focused bool) {
	if ibus == nil || ibus.ibusConn == nil {
		return
	}
	if focused {
		ibus.inputContextObject.Call(inputContextIface+".FocusIn", 0)
	} else {
		ibus.inputContextObject.Call(inputContextIface+".FocusOut", 0)
	}
}

// Reset sends the reset command to IBus
func (ibus *IBus) Reset() {
	if ibus == nil || ibus.ibusConn == nil {
		return
	}
	ibus.compositing = false
	ibus.inputContextObject.Call(inputContextIface+".Reset", 0)
}
