package llamacpp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGenerateCallsChatCompletions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var request chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "gemma4:31b" {
			t.Fatalf("unexpected model: %s", request.Model)
		}
		if request.Stream {
			t.Fatal("stream should be disabled")
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"# Draft"}}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v1", "gemma4:31b", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	draft, err := client.Generate(context.Background(), "write")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if draft != "# Draft" {
		t.Fatalf("unexpected draft: %q", draft)
	}
}

func TestListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"gemma4:31b"}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v1", "gemma4:31b", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	models, err := client.ListModels(context.Background())
	if err != nil {
		t.Fatalf("list models: %v", err)
	}
	if len(models) != 1 || models[0] != "gemma4:31b" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestGenerateStreamCallsChatCompletionsAndAssemblesChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var request chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !request.Stream {
			t.Fatal("stream should be enabled")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"# Draft\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"\\n\\nBody\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v1", "gemma4:31b", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	var chunks []string
	draft, err := client.GenerateStream(context.Background(), "write", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("generate stream: %v", err)
	}
	if draft != "# Draft\n\nBody" {
		t.Fatalf("unexpected draft: %q", draft)
	}
	if strings.Join(chunks, "") != draft {
		t.Fatalf("chunks did not assemble to draft: %#v", chunks)
	}
}

func TestGenerateWithSystemUsesCallerPrompt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Messages[0].Content != "<|nothink|>\nverify only" {
			t.Fatalf("unexpected system prompt: %q", request.Messages[0].Content)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"PASS\nSummary: OK"}}]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL+"/v1", "gemma4:latest", server.Client())
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	report, err := client.GenerateWithSystem(context.Background(), "<|nothink|>\nverify only", "check")
	if err != nil {
		t.Fatalf("generate with system: %v", err)
	}
	if report != "PASS\nSummary: OK" {
		t.Fatalf("unexpected report: %q", report)
	}
}

func TestNewClientFromEnvUsesGenericLLMSettings(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "http://example.test/v1")
	t.Setenv("LLM_MODEL", "gemma4:e2b")
	t.Setenv("LLAMACPP_BASE_URL", "http://legacy.test/v1")
	t.Setenv("LLAMACPP_MODEL", "legacy")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if client.baseURL != "http://example.test/v1" {
		t.Fatalf("unexpected base URL: %s", client.baseURL)
	}
	if client.model != "gemma4:e2b" {
		t.Fatalf("unexpected model: %s", client.model)
	}
}

func TestNewClientFromEnvForPurposeUsesPhaseModel(t *testing.T) {
	t.Setenv("LLM_BASE_URL", "http://example.test/v1")
	t.Setenv("LLM_MODEL", "gemma4:e2b")
	t.Setenv("DRAFT_LLM_MODEL", "gpt-oss:120b")
	t.Setenv("LLM_TIMEOUT_SECONDS", "900")

	client, err := NewClientFromEnvForPurpose("draft")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if client.model != "gpt-oss:120b" {
		t.Fatalf("unexpected model: %s", client.model)
	}
	if client.httpClient.Timeout != 900*time.Second {
		t.Fatalf("unexpected timeout: %s", client.httpClient.Timeout)
	}
}

func TestNewClientFromEnvFallsBackToLegacySettings(t *testing.T) {
	t.Setenv("LLAMACPP_BASE_URL", "http://legacy.test/v1")
	t.Setenv("LLAMACPP_MODEL", "gemma4:31b")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_MODEL", "")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if client.baseURL != "http://legacy.test/v1" {
		t.Fatalf("unexpected base URL: %s", client.baseURL)
	}
	if client.model != "gemma4:31b" {
		t.Fatalf("unexpected model: %s", client.model)
	}
}

func TestGenerateUsesFallbackClientWhenPrimaryFails(t *testing.T) {
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected fallback path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"# Fallback Draft"}}]}`))
	}))
	defer fallbackServer.Close()

	t.Setenv("LLM_BASE_URL", "http://127.0.0.1:1/v1")
	t.Setenv("LLM_MODEL", "remote")
	t.Setenv("FALLBACK_LLM_BASE_URL", fallbackServer.URL+"/v1")
	t.Setenv("FALLBACK_LLM_MODEL", "gemma4:31b")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	draft, err := client.Generate(context.Background(), "write")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if draft != "# Fallback Draft" {
		t.Fatalf("unexpected draft: %q", draft)
	}
}

func TestGenerateUsesOrderedFallbackChain(t *testing.T) {
	firstFallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "first fallback unavailable", http.StatusServiceUnavailable)
	}))
	defer firstFallback.Close()
	secondFallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request chatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if request.Model != "local-qwen" {
			t.Fatalf("unexpected final fallback model: %s", request.Model)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"# Ordered Fallback"}}]}`))
	}))
	defer secondFallback.Close()

	t.Setenv("LLM_BASE_URL", "http://127.0.0.1:1/v1")
	t.Setenv("LLM_MODEL", "remote-ollama")
	t.Setenv("DRAFT_LLM_FALLBACK_BASE_URLS", firstFallback.URL+"/v1, "+secondFallback.URL+"/v1")
	t.Setenv("DRAFT_LLM_FALLBACK_MODELS", "remote-llama, local-qwen")

	client, err := NewClientFromEnvForPurpose("draft")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	draft, err := client.Generate(context.Background(), "write")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if draft != "# Ordered Fallback" {
		t.Fatalf("unexpected draft: %q", draft)
	}
}

func TestGenerateFallsBackWhenResponseContentIsEmpty(t *testing.T) {
	emptyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""}}]}`))
	}))
	defer emptyServer.Close()
	fallbackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"# Non Empty"}}]}`))
	}))
	defer fallbackServer.Close()

	t.Setenv("LLM_BASE_URL", emptyServer.URL+"/v1")
	t.Setenv("LLM_MODEL", "thinking-model")
	t.Setenv("LLM_FALLBACK_BASE_URLS", fallbackServer.URL+"/v1")
	t.Setenv("LLM_FALLBACK_MODELS", "fallback-model")

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	draft, err := client.Generate(context.Background(), "write")
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if draft != "# Non Empty" {
		t.Fatalf("unexpected draft: %q", draft)
	}
}
