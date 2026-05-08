package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestDesktopHandlerServesExistingStaticAssets(t *testing.T) {
	server := httptest.NewServer(newDesktopHandler(resolveDesktopAssets()))
	defer server.Close()

	for _, tt := range []struct {
		path string
		want string
	}{
		{
			path: "/",
			want: `<script src="/static/js/script.js"></script>`,
		},
		{
			path: "/static/vendor/alpine.min.js",
			want: "Alpine",
		},
		{
			path: "/static/js/store.js",
			want: "Alpine.store('app'",
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
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			if !strings.Contains(string(body), tt.want) {
				t.Fatalf("GET %s body missing %q", tt.path, tt.want)
			}
		})
	}
}

func TestDesktopHandlerServesEmbeddedStaticAssetsWithoutRepoStatic(t *testing.T) {
	assets, err := embeddedFrontendFS()
	if err != nil {
		t.Fatalf("embedded frontend fs: %v", err)
	}
	server := httptest.NewServer(newDesktopHandler(desktopAssets{
		FS:          assets,
		Description: "test embedded assets",
	}))
	defer server.Close()

	response, err := http.Get(server.URL + "/static/js/store.js")
	if err != nil {
		t.Fatalf("GET embedded store.js: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("embedded store.js status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read embedded store.js: %v", err)
	}
	if !strings.Contains(string(body), "Alpine.store('app'") {
		t.Fatalf("embedded store.js missing Alpine app store")
	}
}

func TestRegisterDesktopRoutesIncludesWorkflowAPIs(t *testing.T) {
	router := mux.NewRouter()
	registerDesktopRoutes(router, resolveDesktopAssets())

	for _, tt := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/history"},
		{method: http.MethodGet, path: "/api/workflow/artifacts"},
		{method: http.MethodGet, path: "/api/personas"},
		{method: http.MethodPost, path: "/api/personas"},
		{method: http.MethodGet, path: "/api/formats"},
		{method: http.MethodGet, path: "/api/brief-sessions/templates"},
		{method: http.MethodPost, path: "/api/brief-sessions/session-1/answers"},
		{method: http.MethodPost, path: "/api/drafts"},
		{method: http.MethodPost, path: "/api/drafts/draft-1/regenerate-section"},
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

func TestEnsureDesktopDataDirWritesLauncherCompatibleDefaults(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "Note Maker")
	config := desktopRuntimeConfig{
		DataDir:      dataDir,
		ConfigPath:   filepath.Join(dataDir, "app_config.json"),
		WorkflowPath: filepath.Join(dataDir, "workflow_store.db"),
	}
	t.Setenv("WORKFLOW_STORE_DRIVER", "")

	if err := ensureDesktopDataDir(config); err != nil {
		t.Fatalf("ensure desktop data dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "logs")); err != nil {
		t.Fatalf("logs dir was not created: %v", err)
	}
	encoded, err := os.ReadFile(config.ConfigPath)
	if err != nil {
		t.Fatalf("read app config: %v", err)
	}
	var persisted desktopPersistedConfig
	if err := json.Unmarshal(encoded, &persisted); err != nil {
		t.Fatalf("decode app config: %v", err)
	}
	if persisted.WorkflowStoreDriver != "sqlite" || persisted.WorkflowStorePath != config.WorkflowPath {
		t.Fatalf("unexpected persisted config: %#v", persisted)
	}
}
