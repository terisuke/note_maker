//go:build wails

package main

import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

func main() {
	runtimeConfig, err := bootstrapDesktopRuntime()
	if err != nil {
		log.Fatal(err)
	}

	assets, err := embeddedFrontendFS()
	if err != nil {
		log.Fatalf("load embedded desktop assets: %v", err)
	}
	assetsConfig := desktopAssets{
		FS:          assets,
		Description: "embedded frontend assets",
	}
	var appCtx context.Context
	if err := wails.Run(&options.App{
		Title:                    "Note Maker",
		Width:                    1280,
		Height:                   860,
		MinWidth:                 1024,
		MinHeight:                700,
		EnableDefaultContextMenu: true,
		Menu:                     newDesktopMenu(&appCtx, runtimeConfig),
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: newDesktopHandler(assetsConfig),
		},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
			log.Printf("Note Maker desktop started with %s and data dir %s", assetsConfig.Description, runtimeConfig.DataDir)
		},
	}); err != nil {
		log.Fatal(err)
	}
}
