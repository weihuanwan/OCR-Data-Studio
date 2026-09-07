package main

import (
	"context"
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	app := NewApp(dir)

	err = wails.Run(&options.App{
		Title:     "OCR-Data-Studio",
		Width:     1280,
		Height:    860,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown: func(ctx context.Context) {
			// 👇 新增：程序退出时安全关闭日志文件
			if app.logFile != nil {
				app.logFile.Close()
			}
			if cleanupTempDir {
				_ = os.RemoveAll(tempDir)
			}
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
