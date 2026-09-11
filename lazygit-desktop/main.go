package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

// 前端构建产物（frontend/dist）会被打进最终二进制。
//
//go:embed all:frontend/dist
var assets embed.FS

// 应用图标（冰晶 + git 分支）。
//
//go:embed assets/bingit-icon.png
var appIcon []byte

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title: "Bingit",
		// 用自绘标题栏代替系统标题栏（见 frontend 的 TitleBar.tsx）。
		// 注意：Frameless 只去掉装饰，缩放能力由 DisableResize 单独控制，仍是可缩放的。
		Frameless: true,
		Width:     1480,
		Height:    920,
		MinWidth:  1120,
		MinHeight: 680,
		// 深色底，和前端主题一致，避免启动瞬间白屏闪烁。
		BackgroundColour: &options.RGBA{R: 15, G: 16, B: 22, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Linux 专属：窗口图标 + 程序名（影响任务栏归组）
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "bingit",
		},
		// 允许把文件夹拖进窗口打开（前端用 --wails-drop-target 标记可放置区域）
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
