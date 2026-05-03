package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestRegisterRoutesIncludesWorkflowReadAPIs(t *testing.T) {
	router := mux.NewRouter()
	registerRoutes(router)

	for _, tt := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/history"},
		{method: http.MethodGet, path: "/api/workflow/artifacts"},
		{method: http.MethodGet, path: "/api/author-style"},
		{method: http.MethodGet, path: "/api/author-style/style-1"},
		{method: http.MethodGet, path: "/api/brief-sessions"},
		{method: http.MethodGet, path: "/api/brief-sessions/templates"},
		{method: http.MethodGet, path: "/api/brief-sessions/session-1"},
		{method: http.MethodGet, path: "/api/briefs"},
		{method: http.MethodGet, path: "/api/briefs/session-1"},
		{method: http.MethodGet, path: "/api/models"},
		{method: http.MethodGet, path: "/api/personas"},
		{method: http.MethodGet, path: "/api/formats"},
	} {
		request, err := http.NewRequest(tt.method, tt.path, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		var match mux.RouteMatch
		if !router.Match(request, &match) {
			t.Fatalf("%s %s did not match a route", tt.method, tt.path)
		}
	}
}

func TestRegisterRoutesServesBrowserEntryAndStaticScript(t *testing.T) {
	chdir(t, "../..")

	router := mux.NewRouter()
	registerRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()

	for _, tt := range []struct {
		path        string
		contentType string
		body        []string
	}{
		{
			path:        "/",
			contentType: "text/html",
			body: []string{
				`<div id="model-status"`,
				`<select id="style-model"`,
				`<button id="open-history-btn"`,
				`<div id="style-guide-card"`,
				`<script src="/static/js/script.js"></script>`,
			},
		},
		{
			path:        "/static/js/script.js",
			contentType: "javascript",
			body: []string{
				"const configStorageKey = 'note-maker-config-v1'",
				"const historyEndpoint = '/api/workflow/artifacts'",
				"function saveModelConfig()",
				"function openSelectedHistory()",
				"function renderBriefCard(brief)",
			},
		},
	} {
		t.Run(tt.path, func(t *testing.T) {
			response, err := http.Get(server.URL + tt.path)
			if err != nil {
				t.Fatalf("GET %s: %v", tt.path, err)
			}
			defer response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", tt.path, response.StatusCode, http.StatusOK)
			}
			if got := response.Header.Get("Content-Type"); got == "" || !strings.Contains(got, tt.contentType) {
				t.Fatalf("GET %s Content-Type = %q, want contains %q", tt.path, got, tt.contentType)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			for _, want := range tt.body {
				if !strings.Contains(string(body), want) {
					t.Fatalf("GET %s body missing %q", tt.path, want)
				}
			}
		})
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}
