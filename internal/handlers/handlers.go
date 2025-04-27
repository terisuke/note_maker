package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/teradakousuke/note_maker/internal/services/gemini"
)

// ListModelsHandler は利用可能なモデルのリストを返すハンドラー
func ListModelsHandler(w http.ResponseWriter, r *http.Request) {
	// Gemini APIクライアントの作成
	client, err := gemini.NewClient()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create Gemini client: %v", err), http.StatusInternalServerError)
		return
	}

	// モデルの一覧を取得
	models, err := client.ListModels()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list models: %v", err), http.StatusInternalServerError)
		return
	}

	// レスポンスの返却
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}
