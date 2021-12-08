package plugins

import (
	"gioui.org/internal/ops"
	"gioui.org/io/event"
	"gioui.org/op"
)

// RegisteredPlugins contains all plugins registered by Register.
var RegisteredPlugins = make([]PluginInitializer, 0)

// PluginInitializer is a function which will start the Plugin, it will be called once
// per each window created.
//
// In order to avoid cycle-import, the app.Window is provide as interface{}.
type PluginInitializer func(window interface{}) Plugin

// Register will register the given plugin and will be used for any
// new app.Window created.
//
// You must call `Register` on `init` function, otherwise will not possible
// to guarantee that it will work as expected. The id must be used for PluginOp.
func Register(id *int, plugin PluginInitializer) {
	RegisteredPlugins = append(RegisteredPlugins, plugin)
	*id = len(RegisteredPlugins) - 1
}

type Plugin interface {
	// Process receives the Content of plugins.PluginOp.
	Process(op interface{})

	// Push receives any event from app.Window, including custom events.
	//
	// Custom events should be sent using SendEvent on app.Window.
	//
	// If ok is true, it must provide the event and tag related to the given
	// event.
	Push(event event.Event) (tag event.Tag, evt event.Event, ok bool)
}

// PluginOp must be used to broadcast events from custom plugins.
type PluginOp struct {
	// ID is the identifier of the Plugin, obtained calling Register.
	ID int
	// Content holds the custom plugin operation content.
	Content interface{}
}

// Add adds the operation into the *op.Ops
func (op PluginOp) Add(o *op.Ops) {
	data := ops.Write1(&o.Internal, ops.TypeCustomPluginLen, op)
	data[0] = byte(ops.TypeCustomPlugin)
}
