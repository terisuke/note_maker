//go:build wails

package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func newDesktopMenu(appCtx *context.Context, config desktopRuntimeConfig) *menu.Menu {
	applicationMenu := menu.NewMenu()
	applicationMenu.Append(menu.AppMenu())

	fileMenu := applicationMenu.AddSubmenu("File")
	fileMenu.AddText("Open Data Folder", accelerator("CmdOrCtrl+Shift+O"), func(*menu.CallbackData) {
		if ctx := currentWailsContext(appCtx); ctx != nil {
			runtime.BrowserOpenURL(ctx, fileURL(config.DataDir))
		}
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit", accelerator("CmdOrCtrl+Q"), func(*menu.CallbackData) {
		if ctx := currentWailsContext(appCtx); ctx != nil {
			runtime.Quit(ctx)
		}
	})

	applicationMenu.Append(menu.EditMenu())

	helpMenu := applicationMenu.AddSubmenu("Help")
	helpMenu.AddText("LLM Diagnostics", accelerator("CmdOrCtrl+Shift+D"), func(*menu.CallbackData) {
		if ctx := currentWailsContext(appCtx); ctx != nil {
			go showLLMDiagnostics(ctx, config)
		}
	})

	return applicationMenu
}

func showLLMDiagnostics(ctx context.Context, config desktopRuntimeConfig) {
	runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "LLM Diagnostics",
		Message: llmDiagnosticsSummary(config),
	})
}

func currentWailsContext(appCtx *context.Context) context.Context {
	if appCtx == nil || *appCtx == nil {
		return nil
	}
	return *appCtx
}

func accelerator(shortcut string) *keys.Accelerator {
	accelerator, err := keys.Parse(shortcut)
	if err != nil {
		return nil
	}
	return accelerator
}
