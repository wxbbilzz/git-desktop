package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// 前端构建产物（frontend/dist）会被打进最终二进制。
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "Lazygit Desktop",
		Width:     1480,
		Height:    920,
		MinWidth:  1120,
		MinHeight: 680,
		// 深色底，和前端主题一致，避免启动瞬间白屏闪烁。
		BackgroundColour: &options.RGBA{R: 15, G: 16, B: 22, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
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
