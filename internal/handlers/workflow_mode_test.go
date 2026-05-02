package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListPersonasHandler(t *testing.T) {
	response := httptest.NewRecorder()
	ListPersonasHandler(response, httptest.NewRequest(http.MethodGet, "/api/personas", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload []struct {
		ID            string `json:"id"`
		DefaultFormat string `json:"default_format"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode personas: %v", err)
	}
	if len(payload) < 2 {
		t.Fatalf("personas = %d, want at least 2", len(payload))
	}
	if payload[0].ID != "terisuke" || payload[0].DefaultFormat == "" {
		t.Fatalf("unexpected first persona: %#v", payload[0])
	}
}

func TestListFormatsHandler(t *testing.T) {
	response := httptest.NewRecorder()
	ListFormatsHandler(response, httptest.NewRequest(http.MethodGet, "/api/formats", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", response.Code, response.Body.String())
	}
	var payload []struct {
		ID           string `json:"id"`
		RequiresMeta bool   `json:"requires_meta"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode formats: %v", err)
	}
	foundZenn := false
	for _, format := range payload {
		if format.ID == "zenn_article" && format.RequiresMeta {
			foundZenn = true
		}
	}
	if !foundZenn {
		t.Fatalf("zenn_article format with metadata requirement not found: %#v", payload)
	}
}
