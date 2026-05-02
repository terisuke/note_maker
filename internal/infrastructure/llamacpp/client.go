package llamacpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://127.0.0.1:8081/v1"
	defaultModel   = "gemma4:31b"
)

// Client calls an OpenAI-compatible local LLM API such as llama.cpp or Ollama.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
	fallback   *Client
}

// NewClientFromEnv creates a client from LLM_BASE_URL/LLM_MODEL.
//
// LLAMACPP_BASE_URL and LLAMACPP_MODEL remain supported for existing local
// llama.cpp setups.
func NewClientFromEnv() (*Client, error) {
	return NewClientFromEnvForPurpose("")
}

// NewClientFromEnvForPurposeWithModel creates a purpose client and lets callers
// override the model without mutating process-wide environment.
func NewClientFromEnvForPurposeWithModel(purpose, modelOverride string) (*Client, error) {
	return newClientFromEnvForPurpose(purpose, modelOverride)
}

// NewClientFromEnvForPurpose creates a client for a workflow phase.
//
// For purpose "DRAFT", DRAFT_LLM_MODEL is used before LLM_MODEL. This allows
// heavier remote models for draft generation while keeping one default base URL.
func NewClientFromEnvForPurpose(purpose string) (*Client, error) {
	return newClientFromEnvForPurpose(purpose, "")
}

func newClientFromEnvForPurpose(purpose, modelOverride string) (*Client, error) {
	baseURL := firstEnv("LLM_BASE_URL", "LLAMACPP_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := strings.TrimSpace(modelOverride)
	if model == "" {
		model = modelFromEnv(purpose)
	}
	if model == "" {
		model = defaultModel
	}
	client, err := NewClient(baseURL, model, &http.Client{Timeout: timeoutFromEnv()})
	if err != nil {
		return nil, err
	}
	fallback, err := fallbackClientFromEnv(purpose, model)
	if err != nil {
		return nil, err
	}
	client.fallback = fallback
	return client, nil
}

func timeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(firstEnv("LLM_TIMEOUT_SECONDS", "LLAMACPP_TIMEOUT_SECONDS"))
	if raw == "" {
		return 180 * time.Second
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 180 * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func modelFromEnv(purpose string) string {
	purpose = strings.ToUpper(strings.TrimSpace(purpose))
	if purpose != "" {
		if model := os.Getenv(purpose + "_LLM_MODEL"); model != "" {
			return model
		}
	}
	return firstEnv("LLM_MODEL", "LLAMACPP_MODEL")
}

func firstEnv(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}

func fallbackClientFromEnv(purpose, primaryModel string) (*Client, error) {
	purpose = strings.ToUpper(strings.TrimSpace(purpose))
	keys := func(suffix string) []string {
		if purpose == "" {
			return []string{"FALLBACK_" + suffix}
		}
		return []string{purpose + "_FALLBACK_" + suffix, "FALLBACK_" + suffix}
	}
	baseURL := firstEnv(keys("LLM_BASE_URL")...)
	if baseURL == "" {
		baseURL = firstEnv("FALLBACK_LLAMACPP_BASE_URL")
	}
	model := firstEnv(keys("LLM_MODEL")...)
	if model == "" && purpose == "" {
		model = firstEnv("FALLBACK_LLM_MODEL", "FALLBACK_LLAMACPP_MODEL")
	}
	if baseURL == "" && model == "" {
		return nil, nil
	}
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if model == "" {
		model = primaryModel
	}
	return NewClient(baseURL, model, &http.Client{Timeout: timeoutFromEnv()})
}

// NewClient creates a llama.cpp client.
func NewClient(baseURL, model string, httpClient *http.Client) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base URL must include scheme and host")
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 180 * time.Second}
	}
	return &Client{baseURL: baseURL, model: model, httpClient: httpClient}, nil
}

// Generate sends a prompt to /v1/chat/completions and returns the first text response.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	body := chatCompletionRequest{
		Model: c.model,
		Messages: []message{
			{
				Role:    "system",
				Content: "You are a careful Japanese editor. Return only a paste-ready Markdown article. Do not include reasoning, preambles, or code fences.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 1.0,
		TopP:        0.95,
		MaxTokens:   4096,
		Stream:      false,
	}

	var response chatCompletionResponse
	if err := c.post(ctx, "/chat/completions", body, &response); err != nil {
		if c.fallback != nil {
			return c.fallback.Generate(ctx, prompt)
		}
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("llama.cpp response had no choices")
	}
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("llama.cpp response choice was empty")
	}
	return content, nil
}

// ListModels returns model IDs exposed by llama-server.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		if c.fallback != nil {
			return c.fallback.ListModels(ctx)
		}
		return nil, fmt.Errorf("list llama.cpp models: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		if c.fallback != nil {
			return c.fallback.ListModels(ctx)
		}
		return nil, fmt.Errorf("list llama.cpp models: unexpected status %s", response.Status)
	}

	var payload modelsResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	models := make([]string, 0, len(payload.Data))
	for _, model := range payload.Data {
		if model.ID != "" {
			models = append(models, model.ID)
		}
	}
	return models, nil
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode llama.cpp request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create llama.cpp request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call llama.cpp: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("call llama.cpp: unexpected status %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("decode llama.cpp response: %w", err)
	}
	return nil
}

type chatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	TopP        float64   `json:"top_p"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}
