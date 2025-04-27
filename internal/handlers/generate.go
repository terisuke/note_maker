package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/teradakousuke/note_maker/internal/services/gemini"
	"github.com/teradakousuke/note_maker/internal/services/note"
)

// GenerateRequest は記事生成のリクエストボディの構造体
type GenerateRequest struct {
	NoteURL        string   `json:"note_url"`
	Username       string   `json:"username"`
	Keywords       []string `json:"keywords"`
	Theme          string   `json:"theme"`
	TargetAudience string   `json:"target_audience"`
	Exclusions     string   `json:"exclusions"`
	StyleChoice    string   `json:"style_choice"`
	ToneChoice     string   `json:"tone_choice"`
	WordCount      int      `json:"word_count"`
}

// ErrorResponse はエラーレスポンスの構造体
type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Details string `json:"details,omitempty"`
	} `json:"error"`
}

// SuccessResponse は成功レスポンスの構造体
type SuccessResponse struct {
	Draft string `json:"draft"`
}

// GenerateArticleHandler は記事生成のハンドラー
func GenerateArticleHandler(w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 入力検証
	if req.Theme == "" {
		respondWithError(w, "MISSING_REQUIRED_FIELD", "theme is required", "", http.StatusBadRequest)
		return
	}

	// デフォルト値設定
	if req.StyleChoice == "" {
		req.StyleChoice = "ですます調"
	}
	if req.ToneChoice == "" {
		req.ToneChoice = "客観的"
	}
	if req.WordCount <= 0 {
		req.WordCount = 1500
	}

	// Note記事取得サービスの初期化
	fetcher := note.NewFetcher()

	// 記事の取得
	var referenceArticles []string
	if req.Username != "" {
		// ユーザー名から最新の記事を取得
		articles, err := fetcher.FetchUserLatestArticles(req.Username, 3)
		if err != nil {
			respondWithError(w, "FETCH_ARTICLES_FAILED",
				fmt.Sprintf("Failed to fetch articles for user %s", req.Username),
				err.Error(),
				http.StatusInternalServerError)
			return
		}
		// 記事の内容を[]stringに変換
		for _, article := range articles {
			if article.Content != "" {
				referenceArticles = append(referenceArticles, article.Content)
			}
		}
	} else if req.NoteURL != "" {
		// 単一の記事を取得
		article, err := fetcher.FetchArticle(req.NoteURL)
		if err != nil {
			respondWithError(w, "FETCH_ARTICLE_FAILED",
				fmt.Sprintf("Failed to fetch article from URL %s", req.NoteURL),
				err.Error(),
				http.StatusInternalServerError)
			return
		}
		if article.Content != "" {
			referenceArticles = append(referenceArticles, article.Content)
		}
	}

	// 参照記事が取得できなかった場合のエラー処理
	if len(referenceArticles) == 0 {
		// エラーを返さず、空の参照記事で続行
		log.Printf("No reference articles found, proceeding with empty references")
	}

	// Gemini APIを使用した記事生成サービスの初期化
	generator, err := gemini.NewGenerator()
	if err != nil {
		respondWithError(w, "GENERATOR_INITIALIZATION_FAILED",
			"Failed to initialize article generator",
			err.Error(),
			http.StatusInternalServerError)
		return
	}

	// 記事の生成
	draft, err := generator.GenerateArticle(
		referenceArticles,
		req.Keywords,
		req.Theme,
		req.TargetAudience,
		req.Exclusions,
		req.StyleChoice,
		req.ToneChoice,
		req.WordCount,
	)
	if err != nil {
		respondWithError(w, "ARTICLE_GENERATION_FAILED",
			"Failed to generate article",
			err.Error(),
			http.StatusInternalServerError)
		return
	}

	respondWithJSON(w, http.StatusOK, SuccessResponse{Draft: draft})
}

// respondWithError はエラーレスポンスを返す
func respondWithError(w http.ResponseWriter, code, message, details string, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	errResp := ErrorResponse{}
	errResp.Error.Code = code
	errResp.Error.Message = message
	if details != "" {
		errResp.Error.Details = details
	}
	json.NewEncoder(w).Encode(errResp)
}

// respondWithJSON はJSONレスポンスを返す
func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(payload)
}
