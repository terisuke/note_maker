package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageConfigHandlersPersistNextBootSQLite(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "app_config.json")
	t.Setenv("NOTE_MAKER_CONFIG_PATH", configPath)
	t.Setenv("WORKFLOW_STORE_PATH", "")
	setActiveWorkflowStorage(workflowStorageConfig{Driver: storageDriverJSON, Path: defaultJSONPath, Source: "test"})

	body := `{"workflow_store_driver":"sqlite","workflow_store_path":"data/custom.db"}`
	request := httptest.NewRequest(http.MethodPatch, "/api/config/storage", strings.NewReader(body))
	response := httptest.NewRecorder()

	UpdateStorageConfigHandler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var payload storageConfigResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.ConfiguredDriver != storageDriverSQLite || payload.ConfiguredPath != "data/custom.db" {
		t.Fatalf("unexpected configured storage: %#v", payload)
	}
	if !payload.RestartRequired {
		t.Fatalf("restart_required = false, payload = %#v", payload)
	}

	getResponse := httptest.NewRecorder()
	GetStorageConfigHandler(getResponse, httptest.NewRequest(http.MethodGet, "/api/config/storage", nil))
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
	var fetched storageConfigResponse
	if err := json.NewDecoder(getResponse.Body).Decode(&fetched); err != nil {
		t.Fatalf("decode get response: %v", err)
	}
	if fetched.ConfiguredDriver != storageDriverSQLite || fetched.ConfiguredPath != "data/custom.db" {
		t.Fatalf("unexpected fetched storage: %#v", fetched)
	}
}

func TestStorageConfigHandlerRejectsEnvLockedUpdate(t *testing.T) {
	t.Setenv("WORKFLOW_STORE_DRIVER", "sqlite")
	request := httptest.NewRequest(http.MethodPatch, "/api/config/storage", strings.NewReader(`{"workflow_store_driver":"json"}`))
	response := httptest.NewRecorder()

	UpdateStorageConfigHandler(response, request)

	assertErrorResponse(t, response, http.StatusConflict, "STORAGE_CONFIG_LOCKED")
}

func TestStorageConfigHandlerRejectsInvalidDriver(t *testing.T) {
	t.Setenv("WORKFLOW_STORE_DRIVER", "")
	request := httptest.NewRequest(http.MethodPatch, "/api/config/storage", strings.NewReader(`{"workflow_store_driver":"mysql"}`))
	response := httptest.NewRecorder()

	UpdateStorageConfigHandler(response, request)

	assertErrorResponse(t, response, http.StatusBadRequest, "INVALID_STORAGE_CONFIG")
}

func TestResolveWorkflowStorageConfigUsesEnvironment(t *testing.T) {
	t.Setenv("WORKFLOW_STORE_DRIVER", "sqlite")
	t.Setenv("WORKFLOW_STORE_PATH", "data/env.db")

	config := resolveWorkflowStorageConfig()

	if config.Driver != storageDriverSQLite || config.Path != "data/env.db" || config.Source != "env" {
		t.Fatalf("unexpected env config: %#v", config)
	}
}

func TestResolveWorkflowStorageConfigUsesJSONPathEnvironment(t *testing.T) {
	t.Setenv("WORKFLOW_STORE_DRIVER", "")
	t.Setenv("WORKFLOW_STORE_PATH", "data/env.json")
	t.Setenv("NOTE_MAKER_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	config := resolveWorkflowStorageConfig()

	if config.Driver != storageDriverJSON || config.Path != "data/env.json" || config.Source != "env-path" {
		t.Fatalf("unexpected env path config: %#v", config)
	}
}

func TestResolveWorkflowStorageConfigUsesSQLitePathEnvironment(t *testing.T) {
	t.Setenv("WORKFLOW_STORE_DRIVER", "")
	t.Setenv("WORKFLOW_STORE_PATH", "data/env.db")
	t.Setenv("NOTE_MAKER_CONFIG_PATH", filepath.Join(t.TempDir(), "missing.json"))

	config := resolveWorkflowStorageConfig()

	if config.Driver != storageDriverSQLite || config.Path != "data/env.db" || config.Source != "env-path" {
		t.Fatalf("unexpected env path config: %#v", config)
	}
}

func TestNormalizeWorkflowStorageDefaultsPaths(t *testing.T) {
	driver, path, err := normalizeWorkflowStorage("", "")
	if err != nil {
		t.Fatalf("normalize default: %v", err)
	}
	if driver != storageDriverSQLite || path != defaultSQLitePath {
		t.Fatalf("default storage = %s %s", driver, path)
	}

	driver, path, err = normalizeWorkflowStorage("json", "")
	if err != nil {
		t.Fatalf("normalize json: %v", err)
	}
	if driver != storageDriverJSON || path != defaultJSONPath {
		t.Fatalf("json storage = %s %s", driver, path)
	}

	driver, path, err = normalizeWorkflowStorage("sqlite", "")
	if err != nil {
		t.Fatalf("normalize sqlite: %v", err)
	}
	if driver != storageDriverSQLite || path != defaultSQLitePath {
		t.Fatalf("sqlite default = %s %s", driver, path)
	}
}
