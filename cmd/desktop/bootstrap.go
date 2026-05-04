package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/joho/godotenv"
)

const desktopBootstrapEnv = "NOTE_MAKER_DESKTOP_BOOTSTRAPPED"

type desktopRuntimeConfig struct {
	DataDir      string
	ConfigPath   string
	WorkflowPath string
	LLMBaseURL   string
	LLMModel     string
	LLMRuntime   string
}

type desktopPersistedConfig struct {
	WorkflowStoreDriver string `json:"workflow_store_driver"`
	WorkflowStorePath   string `json:"workflow_store_path"`
}

func bootstrapDesktopRuntime() (desktopRuntimeConfig, error) {
	_ = godotenv.Load()

	wasBootstrapped := os.Getenv(desktopBootstrapEnv) == "1"
	config, _, err := applyDesktopDefaults()
	if err != nil {
		return desktopRuntimeConfig{}, err
	}
	if err := ensureDesktopDataDir(config); err != nil {
		return desktopRuntimeConfig{}, err
	}

	if !wasBootstrapped {
		if err := os.Setenv(desktopBootstrapEnv, "1"); err != nil {
			return desktopRuntimeConfig{}, fmt.Errorf("mark desktop bootstrap: %w", err)
		}
		return config, reexecWithCurrentEnv()
	}

	return config, nil
}

func applyDesktopDefaults() (desktopRuntimeConfig, bool, error) {
	changed := false
	dataDir := strings.TrimSpace(os.Getenv("NOTE_MAKER_DATA_DIR"))
	if dataDir == "" {
		dataDir = defaultDesktopDataDir()
		changed = setEnvDefault("NOTE_MAKER_DATA_DIR", dataDir) || changed
	}
	dataDir = filepath.Clean(dataDir)

	configPath := filepath.Clean(strings.TrimSpace(os.Getenv("NOTE_MAKER_CONFIG_PATH")))
	if configPath == "." {
		configPath = filepath.Join(dataDir, "app_config.json")
		changed = setEnvDefault("NOTE_MAKER_CONFIG_PATH", configPath) || changed
	}

	workflowPath := filepath.Clean(strings.TrimSpace(os.Getenv("WORKFLOW_STORE_PATH")))
	if workflowPath == "." {
		workflowPath = filepath.Join(dataDir, "workflow_store.json")
		changed = setEnvDefault("WORKFLOW_STORE_PATH", workflowPath) || changed
	}

	changed = setLauncherLLMDefaults() || changed

	return desktopRuntimeConfig{
		DataDir:      dataDir,
		ConfigPath:   configPath,
		WorkflowPath: workflowPath,
		LLMBaseURL:   os.Getenv("LLM_BASE_URL"),
		LLMModel:     os.Getenv("LLM_MODEL"),
		LLMRuntime:   os.Getenv("LLM_RUNTIME"),
	}, changed, nil
}

func ensureDesktopDataDir(config desktopRuntimeConfig) error {
	if err := os.MkdirAll(filepath.Join(config.DataDir, "logs"), 0o755); err != nil {
		return fmt.Errorf("create desktop data dir: %w", err)
	}
	if _, err := os.Stat(config.ConfigPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat desktop app config: %w", err)
	}

	if strings.TrimSpace(os.Getenv("WORKFLOW_STORE_DRIVER")) != "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(config.ConfigPath), 0o755); err != nil {
		return fmt.Errorf("create desktop config dir: %w", err)
	}
	encoded, err := json.MarshalIndent(desktopPersistedConfig{
		WorkflowStoreDriver: "json",
		WorkflowStorePath:   config.WorkflowPath,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode desktop app config: %w", err)
	}
	if err := os.WriteFile(config.ConfigPath, append(encoded, '\n'), 0o600); err != nil {
		return fmt.Errorf("write desktop app config: %w", err)
	}
	return nil
}

func setLauncherLLMDefaults() bool {
	changed := false
	evoHost := envDefault("EVO_X2_TAILNET_HOST", "evo-x2.tailb30e58.ts.net", &changed)
	evoOllamaBaseURL := envDefault("EVO_X2_OLLAMA_LLM_BASE_URL", "http://"+evoHost+"/v1", &changed)
	evoLlamaBaseURL := envDefault("EVO_X2_LLAMA_CPP_LLM_BASE_URL", "http://"+evoHost+"/llama/v1", &changed)
	evoBaseURL := envDefault("EVO_X2_LLM_BASE_URL", evoOllamaBaseURL, &changed)
	llamacppHost := envDefault("LLAMACPP_HOST", "127.0.0.1", &changed)
	llamacppPort := envDefault("LLAMACPP_PORT", "8081", &changed)
	llamacppModel := envDefault("LLAMACPP_MODEL", "gemma4:31b", &changed)
	envDefault("LLM_RUNTIME", "remote", &changed)
	llmBaseURL := envDefault("LLM_BASE_URL", evoBaseURL, &changed)
	llmModel := envDefault("LLM_MODEL", llamacppModel, &changed)
	envDefault("STYLE_LLM_MODEL", "gemma4:e2b", &changed)
	envDefault("BRIEF_LLM_MODEL", "qwen3.6:27b", &changed)
	envDefault("ARTICLE_LLM_MODEL", "gemma4:e2b", &changed)
	envDefault("DRAFT_LLM_MODEL", llmModel, &changed)
	envDefault("VERIFY_LLM_MODEL", "gemma4:latest", &changed)
	envDefault("LLM_FALLBACK_BASE_URLS", evoLlamaBaseURL+",http://"+llamacppHost+":"+llamacppPort+"/v1", &changed)
	envDefault("LLAMACPP_BASE_URL", llmBaseURL, &changed)
	return changed
}

func envDefault(key, value string, changed *bool) string {
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return os.Getenv(key)
	}
	if setEnvDefault(key, value) {
		*changed = true
	}
	return value
}

func setEnvDefault(key, value string) bool {
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return false
	}
	_ = os.Setenv(key, value)
	return true
}

func defaultDesktopDataDir() string {
	home := strings.TrimSpace(os.Getenv("HOME"))
	switch runtime.GOOS {
	case "darwin":
		if home != "" {
			return filepath.Join(home, "Library", "Application Support", "Note Maker")
		}
	case "windows":
		if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
			return filepath.Join(appData, "Note Maker")
		}
		if userProfile := strings.TrimSpace(os.Getenv("USERPROFILE")); userProfile != "" {
			return filepath.Join(userProfile, "AppData", "Roaming", "Note Maker")
		}
	default:
		if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
			return filepath.Join(xdg, "note-maker")
		}
		if home != "" {
			return filepath.Join(home, ".local", "share", "note-maker")
		}
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "data")
	}
	return "data"
}

func fileURL(path string) string {
	cleaned := filepath.Clean(path)
	if runtime.GOOS == "windows" {
		cleaned = "/" + filepath.ToSlash(cleaned)
	} else {
		cleaned = filepath.ToSlash(cleaned)
	}
	return (&url.URL{Scheme: "file", Path: cleaned}).String()
}
