package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/teradakousuke/note_maker/internal/handlers"
)

func main() {
	// .envファイルの読み込み
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	// PORTを環境変数から取得。無ければデフォルトで8080に。
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ルーターの設定
	r := mux.NewRouter()
	registerRoutes(r)

	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}

func registerRoutes(r *mux.Router) {
	fs := http.FileServer(http.Dir("static"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}).Methods("GET")

	// APIエンドポイントの設定
	r.HandleFunc("/api/generate", handlers.GenerateArticleHandler).Methods("POST")
	r.HandleFunc("/api/models", handlers.ListModelsHandler).Methods("GET")
	r.HandleFunc("/api/config/storage", handlers.GetStorageConfigHandler).Methods("GET")
	r.HandleFunc("/api/config/storage", handlers.UpdateStorageConfigHandler).Methods("PATCH")
	r.HandleFunc("/api/personas", handlers.ListPersonasHandler).Methods("GET")
	r.HandleFunc("/api/personas", handlers.CreatePersonaHandler).Methods("POST")
	r.HandleFunc("/api/personas/{id}", handlers.UpdatePersonaHandler).Methods("PATCH")
	r.HandleFunc("/api/personas/{id}", handlers.DeletePersonaHandler).Methods("DELETE")
	r.HandleFunc("/api/formats", handlers.ListFormatsHandler).Methods("GET")
	r.HandleFunc("/api/history", handlers.ListWorkflowArtifactsHandler).Methods("GET")
	r.HandleFunc("/api/workflow/artifacts", handlers.ListWorkflowArtifactsHandler).Methods("GET")
	r.HandleFunc("/api/author-style", handlers.ListAuthorStylesHandler).Methods("GET")
	r.HandleFunc("/api/author-style/seed", handlers.SeedAuthorStyleHandler).Methods("POST")
	r.HandleFunc("/api/author-style/analyze", handlers.AnalyzeAuthorStyleHandler).Methods("POST")
	r.HandleFunc("/api/author-style/{id}", handlers.GetAuthorStyleHandler).Methods("GET")
	r.HandleFunc("/api/author-style/{id}", handlers.CreateStyleGuideVersionHandler).Methods("PATCH")
	r.HandleFunc("/api/author-style/{id}/versions", handlers.CreateStyleGuideVersionHandler).Methods("POST")
	r.HandleFunc("/api/brief-sessions/templates", handlers.GetBriefSessionTemplateHandler).Methods("GET")
	r.HandleFunc("/api/brief-sessions", handlers.ListBriefSessionsHandler).Methods("GET")
	r.HandleFunc("/api/brief-sessions", handlers.CreateBriefSessionHandler).Methods("POST")
	r.HandleFunc("/api/briefs", handlers.ListBriefArtifactsHandler).Methods("GET")
	r.HandleFunc("/api/briefs/{id}", handlers.GetBriefArtifactHandler).Methods("GET")
	r.HandleFunc("/api/briefs/{id}/versions", handlers.ListBriefVersionsHandler).Methods("GET")
	r.HandleFunc("/api/briefs/{id}", handlers.UpdateBriefArtifactHandler).Methods("PATCH")
	r.HandleFunc("/api/brief-sessions/{id}", handlers.GetBriefSessionHandler).Methods("GET")
	r.HandleFunc("/api/brief-sessions/{id}/answers", handlers.AnswerBriefSessionHandler).Methods("POST")
	r.HandleFunc("/api/brief-sessions/{id}/answers/{answer_id}/edit", handlers.EditBriefAnswerHandler).Methods("POST")
	r.HandleFunc("/api/sessions/{id}/answers/{answer_id}/edit", handlers.EditBriefAnswerHandler).Methods("POST")
	r.HandleFunc("/api/projects", handlers.ListProjectsHandler).Methods("GET")
	r.HandleFunc("/api/projects/{id}", handlers.GetProjectHandler).Methods("GET")
	r.HandleFunc("/api/articles/{id}", handlers.GetArticleHandler).Methods("GET")
	r.HandleFunc("/api/drafts", handlers.GenerateDraftHandler).Methods("POST")
	r.HandleFunc("/api/drafts/{id}", handlers.GetDraftHandler).Methods("GET")
	r.HandleFunc("/api/drafts/{id}/regenerate-section", handlers.RegenerateDraftSectionHandler).Methods("POST")

	// ルートパスへのアクセスはindex.htmlにリダイレクト
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})
}
