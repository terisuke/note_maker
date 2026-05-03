package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	storageDriverJSON   = "json"
	storageDriverSQLite = "sqlite"
)

var activeWorkflowStorage = workflowStorageConfig{
	Driver: storageDriverJSON,
	Path:   "data/workflow_store.json",
	Source: "default",
}

var workflowStorageMu sync.RWMutex

type workflowStorageConfig struct {
	Driver string
	Path   string
	Source string
}

type persistedAppConfig struct {
	WorkflowStoreDriver string `json:"workflow_store_driver"`
	WorkflowStorePath   string `json:"workflow_store_path"`
}

type storageConfigRequest struct {
	WorkflowStoreDriver string `json:"workflow_store_driver"`
	WorkflowStorePath   string `json:"workflow_store_path"`
}

type storageConfigResponse struct {
	ActiveDriver      string `json:"active_driver"`
	ActivePath        string `json:"active_path"`
	ConfiguredDriver  string `json:"configured_driver"`
	ConfiguredPath    string `json:"configured_path"`
	ConfigPath        string `json:"config_path"`
	EnvLocked         bool   `json:"env_locked"`
	RestartRequired   bool   `json:"restart_required"`
	RestartMessage    string `json:"restart_message"`
	EffectiveNextBoot bool   `json:"effective_next_boot"`
}

// GetStorageConfigHandler returns the current and next-boot workflow storage config.
func GetStorageConfigHandler(w http.ResponseWriter, r *http.Request) {
	respondWithJSON(w, http.StatusOK, currentStorageConfigResponse())
}

// UpdateStorageConfigHandler writes the next-boot workflow storage config.
func UpdateStorageConfigHandler(w http.ResponseWriter, r *http.Request) {
	if storageEnvLocked() {
		respondWithError(w, "STORAGE_CONFIG_LOCKED", "Storage config is locked by environment variables", "", http.StatusConflict)
		return
	}
	var req storageConfigRequest
	if err := decodeJSONRequest(r, &req); err != nil {
		respondWithError(w, "INVALID_REQUEST_FORMAT", "Invalid request body", "", http.StatusBadRequest)
		return
	}
	driver, path, err := normalizeWorkflowStorage(req.WorkflowStoreDriver, req.WorkflowStorePath)
	if err != nil {
		respondWithError(w, "INVALID_STORAGE_CONFIG", "Invalid storage config", err.Error(), http.StatusBadRequest)
		return
	}
	if err := writePersistedAppConfig(persistedAppConfig{
		WorkflowStoreDriver: driver,
		WorkflowStorePath:   path,
	}); err != nil {
		respondWithError(w, "STORAGE_CONFIG_SAVE_FAILED", "Failed to save storage config", err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, http.StatusOK, currentStorageConfigResponse())
}

func currentStorageConfigResponse() storageConfigResponse {
	active := getActiveWorkflowStorage()
	configured := resolveNextBootWorkflowStorage()
	restartRequired := active.Driver != configured.Driver || active.Path != configured.Path
	message := "現在の保存先で動作中です。"
	if restartRequired {
		message = "保存方式の変更はサーバー再起動後に反映されます。"
	}
	return storageConfigResponse{
		ActiveDriver:      active.Driver,
		ActivePath:        active.Path,
		ConfiguredDriver:  configured.Driver,
		ConfiguredPath:    configured.Path,
		ConfigPath:        appConfigPath(),
		EnvLocked:         storageEnvLocked(),
		RestartRequired:   restartRequired,
		RestartMessage:    message,
		EffectiveNextBoot: !restartRequired,
	}
}

func resolveWorkflowStorageConfig() workflowStorageConfig {
	if driver := strings.TrimSpace(os.Getenv("WORKFLOW_STORE_DRIVER")); driver != "" {
		path := strings.TrimSpace(os.Getenv("WORKFLOW_STORE_PATH"))
		normalizedDriver, normalizedPath, err := normalizeWorkflowStorage(driver, path)
		if err != nil {
			return workflowStorageConfig{Driver: driver, Path: path, Source: "env-invalid"}
		}
		return workflowStorageConfig{Driver: normalizedDriver, Path: normalizedPath, Source: "env"}
	}
	configured := resolveNextBootWorkflowStorage()
	if configured.Source == "default" && strings.TrimSpace(os.Getenv("WORKFLOW_STORE_PATH")) != "" {
		path := strings.TrimSpace(os.Getenv("WORKFLOW_STORE_PATH"))
		_, normalizedPath, err := normalizeWorkflowStorage(storageDriverJSON, path)
		if err != nil {
			return workflowStorageConfig{Driver: storageDriverJSON, Path: path, Source: "env-path-invalid"}
		}
		return workflowStorageConfig{Driver: storageDriverJSON, Path: normalizedPath, Source: "env-path"}
	}
	return configured
}

func resolveNextBootWorkflowStorage() workflowStorageConfig {
	config, ok := readPersistedAppConfig()
	if !ok {
		return workflowStorageConfig{Driver: storageDriverJSON, Path: "data/workflow_store.json", Source: "default"}
	}
	driver, path, err := normalizeWorkflowStorage(config.WorkflowStoreDriver, config.WorkflowStorePath)
	if err != nil {
		return workflowStorageConfig{Driver: storageDriverJSON, Path: "data/workflow_store.json", Source: "config-invalid"}
	}
	return workflowStorageConfig{Driver: driver, Path: path, Source: "config"}
}

func normalizeWorkflowStorage(driver, path string) (string, string, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	switch driver {
	case "", storageDriverJSON:
		driver = storageDriverJSON
		if strings.TrimSpace(path) == "" {
			path = "data/workflow_store.json"
		}
	case storageDriverSQLite:
		if strings.TrimSpace(path) == "" {
			path = "data/workflow_store.db"
		}
	default:
		return "", "", fmt.Errorf("workflow_store_driver must be json or sqlite")
	}
	return driver, filepath.Clean(strings.TrimSpace(path)), nil
}

func setActiveWorkflowStorage(config workflowStorageConfig) {
	workflowStorageMu.Lock()
	defer workflowStorageMu.Unlock()
	activeWorkflowStorage = config
}

func getActiveWorkflowStorage() workflowStorageConfig {
	workflowStorageMu.RLock()
	defer workflowStorageMu.RUnlock()
	return activeWorkflowStorage
}

func storageEnvLocked() bool {
	return strings.TrimSpace(os.Getenv("WORKFLOW_STORE_DRIVER")) != ""
}

func appConfigPath() string {
	if path := strings.TrimSpace(os.Getenv("NOTE_MAKER_CONFIG_PATH")); path != "" {
		return filepath.Clean(path)
	}
	return "data/app_config.json"
}

func readPersistedAppConfig() (persistedAppConfig, bool) {
	encoded, err := os.ReadFile(appConfigPath())
	if err != nil {
		return persistedAppConfig{}, false
	}
	var config persistedAppConfig
	if err := json.Unmarshal(encoded, &config); err != nil {
		return persistedAppConfig{}, false
	}
	return config, true
}

func writePersistedAppConfig(config persistedAppConfig) error {
	path := appConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create app config dir: %w", err)
	}
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("encode app config: %w", err)
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write app config temp: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace app config: %w", err)
	}
	return nil
}
