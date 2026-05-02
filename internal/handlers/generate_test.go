package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/teradakousuke/note_maker/internal/domain/article"
)

func TestHandleGenerateArticleSuccess(t *testing.T) {
	service := &fakeArticleService{draft: "# Draft"}
	request := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewBufferString(`{"note_url":"https://note.com/u/n/n1","theme":"theme"}`))
	response := httptest.NewRecorder()

	handleGenerateArticle(service, response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", response.Code, response.Body.String())
	}
	var payload SuccessResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Draft != "# Draft" {
		t.Fatalf("unexpected draft: %q", payload.Draft)
	}
	if service.request.Theme != "theme" {
		t.Fatalf("request was not passed to service: %#v", service.request)
	}
}

func TestHandleGenerateArticleValidatesContract(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing source", body: `{"theme":"theme"}`},
		{name: "missing theme", body: `{"note_url":"https://note.com/u/n/n1"}`},
		{name: "bad json", body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewBufferString(tt.body))
			response := httptest.NewRecorder()

			handleGenerateArticle(&fakeArticleService{draft: "# Draft"}, response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("unexpected status: %d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestHandleGenerateArticleReportsServiceFailure(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewBufferString(`{"note_url":"https://note.com/u/n/n1","theme":"theme"}`))
	response := httptest.NewRecorder()

	handleGenerateArticle(&fakeArticleService{err: errors.New("llm unavailable")}, response, request)

	assertErrorResponse(t, response, http.StatusInternalServerError, "ARTICLE_GENERATION_FAILED")
}

func TestGenerateArticleHandlerReportsInvalidRuntimeConfig(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "://bad-url")
	request := httptest.NewRequest(http.MethodPost, "/api/generate", bytes.NewBufferString(`{"note_url":"https://note.com/u/n/n1","theme":"theme"}`))
	response := httptest.NewRecorder()

	GenerateArticleHandler(response, request)

	assertErrorResponse(t, response, http.StatusInternalServerError, "GENERATOR_INITIALIZATION_FAILED")
}

type fakeArticleService struct {
	request domain.GenerationRequest
	draft   string
	err     error
}

func (s *fakeArticleService) GenerateArticle(ctx context.Context, req domain.GenerationRequest) (string, error) {
	s.request = req
	return s.draft, s.err
}
