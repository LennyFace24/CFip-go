package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

func init() {
	// 注册自定义事件，绑定生成器会据此产出强类型的 TS API。
	application.RegisterEvent[SpeedResult]("speed:result")
	application.RegisterEvent[SpeedSummary]("speed:done")
}

// main 初始化应用：装配三个服务、创建主窗口并启动事件循环。
func main() {
	configService := &ConfigService{}
	ipService := &IPService{}
	speedService := &SpeedService{}
	logService := &LogService{}

	app := application.New(application.Options{
		Name:        "CFip",
		Description: "Cloudflare 优选 IP 测速工具",
		Services: []application.Service{
			application.NewService(configService),
			application.NewService(ipService),
			application.NewService(speedService),
			application.NewService(logService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// 服务需要 app 才能弹系统对话框、发事件、写剪贴板。
	ipService.app = app
	speedService.app = app
	logService.app = app

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "CFip · Cloudflare 优选 IP",
		Width:     1180,
		Height:    760,
		MinWidth:  960,
		MinHeight: 620,
		// 无边框：标题栏由前端绘制，保持三端观感一致
		Frameless: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(18, 18, 22),
		URL:              "/",
	})

	// Run the application. This blocks until the application has been exited.
	err := app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
