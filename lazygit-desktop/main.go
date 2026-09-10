package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// 前端构建产物（frontend/dist）会被打进最终二进制。
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "Lazygit Desktop",
		Width:     1380,
		Height:    880,
		MinWidth:  1040,
		MinHeight: 640,
		// 深色底，和前端主题一致，避免启动瞬间白屏闪烁。
		BackgroundColour: &options.RGBA{R: 15, G: 16, B: 22, A: 1},
		AssetServer: &assetserver.Options{
			Assets: assets,
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
