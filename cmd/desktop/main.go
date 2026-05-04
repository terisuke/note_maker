//go:build !wails

package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	runtimeConfig, err := bootstrapDesktopRuntime()
	if err != nil {
		log.Fatal(err)
	}

	address := os.Getenv("NOTE_MAKER_DESKTOP_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}

	assets := resolveDesktopAssets()
	log.Printf("Starting Note Maker desktop preview on http://%s using assets from %s and data dir %s", address, assets.Description, runtimeConfig.DataDir)
	if err := http.ListenAndServe(address, newDesktopHandler(assets)); err != nil {
		log.Fatal(err)
	}
}
