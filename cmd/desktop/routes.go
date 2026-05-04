package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/teradakousuke/note_maker/internal/handlers"
)

func newDesktopHandler(assets desktopAssets) http.Handler {
	if assets.FS == nil && assets.StaticRoot == "" {
		assets = resolveDesktopAssets()
	}

	router := mux.NewRouter()
	registerDesktopRoutes(router, assets)
	return router
}

func registerDesktopRoutes(r *mux.Router, assets desktopAssets) {
	staticFiles := assetFileServer(assets)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticFiles))
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}).Methods(http.MethodGet)

	r.HandleFunc("/api/generate", handlers.GenerateArticleHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/models", handlers.ListModelsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/config/storage", handlers.GetStorageConfigHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/config/storage", handlers.UpdateStorageConfigHandler).Methods(http.MethodPatch)
	r.HandleFunc("/api/personas", handlers.ListPersonasHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/personas", handlers.CreatePersonaHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/personas/{id}", handlers.UpdatePersonaHandler).Methods(http.MethodPatch)
	r.HandleFunc("/api/personas/{id}", handlers.DeletePersonaHandler).Methods(http.MethodDelete)
	r.HandleFunc("/api/formats", handlers.ListFormatsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/history", handlers.ListWorkflowArtifactsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/workflow/artifacts", handlers.ListWorkflowArtifactsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/author-style", handlers.ListAuthorStylesHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/author-style/seed", handlers.SeedAuthorStyleHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/author-style/analyze", handlers.AnalyzeAuthorStyleHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/author-style/{id}", handlers.GetAuthorStyleHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/author-style/{id}", handlers.CreateStyleGuideVersionHandler).Methods(http.MethodPatch)
	r.HandleFunc("/api/author-style/{id}/versions", handlers.CreateStyleGuideVersionHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/brief-sessions/templates", handlers.GetBriefSessionTemplateHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/brief-sessions", handlers.ListBriefSessionsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/brief-sessions", handlers.CreateBriefSessionHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/briefs", handlers.ListBriefArtifactsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/briefs/{id}", handlers.GetBriefArtifactHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/briefs/{id}/versions", handlers.ListBriefVersionsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/briefs/{id}", handlers.UpdateBriefArtifactHandler).Methods(http.MethodPatch)
	r.HandleFunc("/api/brief-sessions/{id}", handlers.GetBriefSessionHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/brief-sessions/{id}/answers", handlers.AnswerBriefSessionHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/brief-sessions/{id}/answers/{answer_id}/edit", handlers.EditBriefAnswerHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/sessions/{id}/answers/{answer_id}/edit", handlers.EditBriefAnswerHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/projects", handlers.ListProjectsHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/projects/{id}", handlers.GetProjectHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/articles/{id}", handlers.GetArticleHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/drafts", handlers.GenerateDraftHandler).Methods(http.MethodPost)
	r.HandleFunc("/api/drafts/{id}", handlers.GetDraftHandler).Methods(http.MethodGet)
	r.HandleFunc("/api/drafts/{id}/regenerate-section", handlers.RegenerateDraftSectionHandler).Methods(http.MethodPost)

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		serveIndex(w, assets)
	}).Methods(http.MethodGet)
}

func serveIndex(w http.ResponseWriter, assets desktopAssets) {
	file, err := openAsset(assets, "index.html")
	if err != nil {
		http.Error(w, "desktop index asset not found", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, file)
}

func resolveStaticDir() string {
	if staticRoot := os.Getenv("NOTE_MAKER_STATIC_DIR"); staticRoot != "" {
		return staticRoot
	}

	for _, root := range staticDirCandidates() {
		if isStaticDir(root) {
			return root
		}
	}

	return "static"
}

func staticDirCandidates() []string {
	candidates := make([]string, 0, 12)
	if wd, err := os.Getwd(); err == nil {
		candidates = appendStaticAncestors(candidates, wd)
	}
	if executable, err := os.Executable(); err == nil {
		candidates = appendStaticAncestors(candidates, filepath.Dir(executable))
	}
	return candidates
}

func appendStaticAncestors(candidates []string, start string) []string {
	for dir := filepath.Clean(start); ; dir = filepath.Dir(dir) {
		candidates = append(candidates, filepath.Join(dir, "static"))
		parent := filepath.Dir(dir)
		if parent == dir {
			return candidates
		}
	}
}

func isStaticDir(root string) bool {
	for _, name := range []string{
		"index.html",
		filepath.Join("vendor", "alpine.min.js"),
		filepath.Join("js", "script.js"),
	} {
		if info, err := os.Stat(filepath.Join(root, name)); err != nil || info.IsDir() {
			return false
		}
	}
	return true
}
