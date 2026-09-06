module github.com/LennyFace24/CFip-go/cmd/app

go 1.25.0

// GUI 作为独立模块，通过本地 replace 复用父模块的 core/config 业务逻辑，
// 避免 Wails 的重依赖污染根模块。
replace github.com/LennyFace24/CFip-go => ../../

require (
	github.com/LennyFace24/CFip-go v0.0.0-00010101000000-000000000000
	github.com/wailsapp/wails/v3 v3.0.0-beta.16
)

require (
	github.com/adrg/xdg v0.5.3 // indirect
	github.com/coder/websocket v1.8.14 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/jchv/go-winloader v0.0.0-20250406163304-c1995be93bd1 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.46.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
