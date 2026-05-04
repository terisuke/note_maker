package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

type desktopAssets struct {
	FS          fs.FS
	StaticRoot  string
	Description string
}

func resolveDesktopAssets() desktopAssets {
	if staticRoot := os.Getenv("NOTE_MAKER_STATIC_DIR"); staticRoot != "" {
		return desktopAssets{
			StaticRoot:  staticRoot,
			Description: staticRoot,
		}
	}

	for _, root := range staticDirCandidates() {
		if isStaticDir(root) {
			return desktopAssets{
				StaticRoot:  root,
				Description: root,
			}
		}
	}

	assets, err := embeddedFrontendFS()
	if err == nil {
		return desktopAssets{
			FS:          assets,
			Description: "embedded frontend assets",
		}
	}

	return desktopAssets{
		StaticRoot:  "static",
		Description: fmt.Sprintf("static (embedded assets unavailable: %v)", err),
	}
}

func embeddedFrontendFS() (fs.FS, error) {
	return fs.Sub(embeddedFrontend, "frontend")
}

func openAsset(assets desktopAssets, name string) (fs.File, error) {
	if assets.FS != nil {
		return assets.FS.Open(name)
	}
	return os.Open(filepath.Join(assets.StaticRoot, filepath.FromSlash(name)))
}

func assetFileServer(assets desktopAssets) http.Handler {
	if assets.FS != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			serveAsset(w, r, assets, strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/"))
		})
	}
	return http.FileServer(http.Dir(assets.StaticRoot))
}

func serveAsset(w http.ResponseWriter, r *http.Request, assets desktopAssets, name string) {
	file, err := openAsset(assets, embeddedAssetName(name))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()

	if contentType := mime.TypeByExtension(path.Ext(name)); contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	if seeker, ok := file.(io.ReadSeeker); ok {
		http.ServeContent(w, r, name, modTime(file), seeker)
		return
	}
	_, _ = io.Copy(w, file)
}

func embeddedAssetName(name string) string {
	if strings.HasPrefix(name, "vendor/") {
		return "_vendor/" + strings.TrimPrefix(name, "vendor/")
	}
	return name
}

func modTime(file fs.File) time.Time {
	info, err := file.Stat()
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
