package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
)

// ListModelsHandler は利用可能なモデルのリストを返すハンドラー
func ListModelsHandler(w http.ResponseWriter, r *http.Request) {
	client, err := llamacpp.NewClientFromEnv()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create local LLM client: %v", err), http.StatusInternalServerError)
		return
	}

	models, err := client.ListModels(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list local models: %v", err), http.StatusInternalServerError)
		return
	}

	// レスポンスの返却
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(models); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}
