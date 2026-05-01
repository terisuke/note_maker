package llamacpp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
