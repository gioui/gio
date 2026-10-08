module gioui.org

go 1.25.0

require (
	eliasnaur.com/font v0.0.0-20230308162249-dd43949cb42d
	gioui.org/shader v1.0.9
	github.com/go-text/typesetting v0.3.5
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0
	golang.org/x/exp/shiny v0.0.0-20250408133849-7e4ce0ab07d0
	golang.org/x/image v0.26.0
	golang.org/x/sys v0.39.0
	golang.org/x/text v0.32.0
)

require (
	github.com/ebitengine/purego v0.11.0
	golang.org/x/net v0.48.0
)

require github.com/godbus/dbus/v5 v5.2.2

require github.com/christerso/vulkan-go v0.0.0-20260618152204-bff25e5b7646

replace github.com/christerso/vulkan-go => ./third_party/vulkan-go

replace gioui.org/shader => ./third_party/gio-shader
