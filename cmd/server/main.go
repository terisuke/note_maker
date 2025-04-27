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

	// 静的ファイルの配信 (staticディレクトリをルートとして提供)
	fs := http.FileServer(http.Dir("static"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	// APIエンドポイントの設定
	r.HandleFunc("/api/generate", handlers.GenerateArticleHandler).Methods("POST")
	r.HandleFunc("/api/models", handlers.ListModelsHandler).Methods("GET")

	// ルートパスへのアクセスはindex.htmlにリダイレクト
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/index.html")
	})

	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
