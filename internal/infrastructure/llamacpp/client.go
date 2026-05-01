package llamacpp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://127.0.0.1:8081/v1"
	defaultModel   = "gemma4:31b"
)

// Client calls a llama.cpp llama-server OpenAI-compatible API.
type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
}

// NewClientFromEnv creates a client from LLAMACPP_BASE_URL and LLAMACPP_MODEL.
func NewClientFromEnv() (*Client, error) {
	baseURL := os.Getenv("LLAMACPP_BASE_URL")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	model := os.Getenv("LLAMACPP_MODEL")
	if model == "" {
		model = defaultModel
	}
	return NewClient(baseURL, model, &http.Client{Timeout: 180 * time.Second})
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
		return nil, fmt.Errorf("list llama.cpp models: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
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
