package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	articleapp "github.com/teradakousuke/note_maker/internal/application/article"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
	notenote "github.com/teradakousuke/note_maker/internal/infrastructure/note"
)

// GenerateRequest は記事生成のリクエストボディの構造体
type GenerateRequest struct {
	NoteURL               string   `json:"note_url"`
	Username              string   `json:"username"`
	Keywords              []string `json:"keywords"`
	Theme                 string   `json:"theme"`
	TargetAudience        string   `json:"target_audience"`
	Exclusions            string   `json:"exclusions"`
	StyleChoice           string   `json:"style_choice"`
	ToneChoice            string   `json:"tone_choice"`
	WordCount             int      `json:"word_count"`
	ReferenceArticleLimit int      `json:"reference_article_limit"`
	ArticlePurpose        string   `json:"article_purpose"`
	DesiredContent        string   `json:"desired_content"`
	IntroductionPoints    string   `json:"introduction_points"`
	MainPoints            string   `json:"main_points"`
	ConclusionMessage     string   `json:"conclusion_message"`
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

type articleService interface {
	GenerateArticle(ctx context.Context, req articledomain.GenerationRequest) (string, error)
}

// GenerateArticleHandler は記事生成のハンドラー
func GenerateArticleHandler(w http.ResponseWriter, r *http.Request) {
	service, err := newDefaultArticleService()
	if err != nil {
		respondWithError(w, "GENERATOR_INITIALIZATION_FAILED",
			"Failed to initialize local LLM client",
			err.Error(),
			http.StatusInternalServerError)
		return
	}
	handleGenerateArticle(service, w, r)
}

func newDefaultArticleService() (articleService, error) {
	generator, err := llamacpp.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return articleapp.NewService(notenote.NewFetcher(), generator, nil), nil
}

func handleGenerateArticle(service articleService, w http.ResponseWriter, r *http.Request) {
	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	generationRequest := articledomain.GenerationRequest{
		NoteURL:               req.NoteURL,
		Username:              req.Username,
		Keywords:              req.Keywords,
		Theme:                 req.Theme,
		TargetAudience:        req.TargetAudience,
		Exclusions:            req.Exclusions,
		StyleChoice:           req.StyleChoice,
		ToneChoice:            req.ToneChoice,
		WordCount:             req.WordCount,
		ReferenceArticleLimit: req.ReferenceArticleLimit,
		ArticlePurpose:        req.ArticlePurpose,
		DesiredContent:        req.DesiredContent,
		IntroductionPoints:    req.IntroductionPoints,
		MainPoints:            req.MainPoints,
		ConclusionMessage:     req.ConclusionMessage,
	}
	if err := generationRequest.Validate(); err != nil {
		respondWithError(w, "MISSING_REQUIRED_FIELD", err.Error(), "", http.StatusBadRequest)
		return
	}

	draft, err := service.GenerateArticle(r.Context(), generationRequest)
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
	if err := json.NewEncoder(w).Encode(errResp); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}

// respondWithJSON はJSONレスポンスを返す
func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
