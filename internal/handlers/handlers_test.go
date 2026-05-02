package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListModelsHandlerReturnsOpenAICompatibleModels(t *testing.T) {
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gemma4:31b"},{"id":"qwen3.6:27b"}]}`))
	}))
	defer llmServer.Close()
	t.Setenv("LLM_BASE_URL", llmServer.URL+"/v1")
	t.Setenv("LLM_MODEL", "gemma4:31b")

	response := httptest.NewRecorder()
	ListModelsHandler(response, httptest.NewRequest(http.MethodGet, "/api/models", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var models []string
	if err := json.NewDecoder(response.Body).Decode(&models); err != nil {
		t.Fatalf("decode models: %v", err)
	}
	if len(models) != 2 || models[0] != "gemma4:31b" || models[1] != "qwen3.6:27b" {
		t.Fatalf("models = %#v", models)
	}
}

func TestListModelsHandlerReportsInvalidRuntimeConfig(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "://bad-url")

	response := httptest.NewRecorder()
	ListModelsHandler(response, httptest.NewRequest(http.MethodGet, "/api/models", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
