package llamacpp

import (
	"bufio"
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
	fallback, err := fallbackChainFromEnv(purpose, model)
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

func fallbackChainFromEnv(purpose, primaryModel string) (*Client, error) {
	purpose = strings.ToUpper(strings.TrimSpace(purpose))
	keys := func(suffix string) []string {
		if purpose == "" {
			return []string{"FALLBACK_" + suffix}
		}
		return []string{purpose + "_FALLBACK_" + suffix, "FALLBACK_" + suffix}
	}
	listKeys := func(suffix string) []string {
		if purpose == "" {
			return []string{"LLM_FALLBACK_" + suffix, "FALLBACK_LLM_" + suffix}
		}
		return []string{
			purpose + "_LLM_FALLBACK_" + suffix,
			purpose + "_FALLBACK_LLM_" + suffix,
			"LLM_FALLBACK_" + suffix,
			"FALLBACK_LLM_" + suffix,
		}
	}

	baseURLs := splitEnvList(firstEnv(listKeys("BASE_URLS")...))
	if len(baseURLs) == 0 {
		baseURL := firstEnv(keys("LLM_BASE_URL")...)
		if baseURL == "" {
			baseURL = firstEnv("FALLBACK_LLAMACPP_BASE_URL")
		}
		if baseURL != "" {
			baseURLs = []string{baseURL}
		}
	}
	models := splitEnvList(firstEnv(listKeys("MODELS")...))
	if len(models) == 0 {
		model := firstEnv(keys("LLM_MODEL")...)
		if model == "" && purpose == "" {
			model = firstEnv("FALLBACK_LLM_MODEL", "FALLBACK_LLAMACPP_MODEL")
		}
		if model != "" {
			models = []string{model}
		}
	}
	if len(baseURLs) == 0 && len(models) == 0 {
		return nil, nil
	}
	if len(baseURLs) == 0 {
		baseURLs = []string{defaultBaseURL}
	}

	var head *Client
	var previous *Client
	for index, baseURL := range baseURLs {
		model := fallbackModelForIndex(models, index, primaryModel)
		client, err := NewClient(baseURL, model, &http.Client{Timeout: timeoutFromEnv()})
		if err != nil {
			return nil, err
		}
		if head == nil {
			head = client
		}
		if previous != nil {
			previous.fallback = client
		}
		previous = client
	}
	return head, nil
}

func splitEnvList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := strings.TrimSpace(part); cleaned != "" {
			values = append(values, cleaned)
		}
	}
	return values
}

func fallbackModelForIndex(models []string, index int, primaryModel string) string {
	if len(models) == 0 {
		return primaryModel
	}
	if index < len(models) {
		return models[index]
	}
	if len(models) == 1 {
		return models[0]
	}
	return primaryModel
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

// BaseURL returns the OpenAI-compatible endpoint used by this client.
func (c *Client) BaseURL() string {
	return c.baseURL
}

// Model returns the model name used by this client.
func (c *Client) Model() string {
	return c.model
}

// Generate sends a prompt to /v1/chat/completions and returns the first text response.
func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	return c.GenerateWithSystem(ctx, defaultSystemPrompt, prompt)
}

// GenerateWithSystem sends a non-streaming chat completion with a caller-provided system prompt.
func (c *Client) GenerateWithSystem(ctx context.Context, systemPrompt, prompt string) (string, error) {
	body := c.chatCompletionRequest(systemPrompt, prompt, false)

	var response chatCompletionResponse
	if err := c.post(ctx, "/chat/completions", body, &response); err != nil {
		if c.fallback != nil && ctx.Err() == nil {
			return c.fallback.GenerateWithSystem(ctx, systemPrompt, prompt)
		}
		return "", err
	}
	if len(response.Choices) == 0 {
		if c.fallback != nil {
			return c.fallback.GenerateWithSystem(ctx, systemPrompt, prompt)
		}
		return "", fmt.Errorf("llama.cpp response had no choices")
	}
	content := strings.TrimSpace(response.Choices[0].Message.Content)
	if content == "" {
		if c.fallback != nil {
			return c.fallback.GenerateWithSystem(ctx, systemPrompt, prompt)
		}
		return "", fmt.Errorf("llama.cpp response choice was empty")
	}
	return content, nil
}

// GenerateStream sends a streaming chat completion request and calls onChunk for
// every text delta. It returns the assembled response so callers can validate it
// with the same path as the non-streaming API.
func (c *Client) GenerateStream(ctx context.Context, prompt string, onChunk func(string) error) (string, error) {
	content, err := c.generateStream(ctx, prompt, onChunk)
	if err != nil {
		if c.fallback != nil && strings.TrimSpace(content) == "" && ctx.Err() == nil {
			if streamingFallback, ok := any(c.fallback).(interface {
				GenerateStream(context.Context, string, func(string) error) (string, error)
			}); ok {
				return streamingFallback.GenerateStream(ctx, prompt, onChunk)
			}
			return c.fallback.Generate(ctx, prompt)
		}
		return content, err
	}
	return content, nil
}

const defaultSystemPrompt = "You are a careful Japanese editor. Return only a paste-ready Markdown article. Do not include reasoning, preambles, or code fences."

func (c *Client) chatCompletionRequest(systemPrompt, prompt string, stream bool) chatCompletionRequest {
	systemPrompt = strings.TrimSpace(systemPrompt)
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}
	body := chatCompletionRequest{
		Model: c.model,
		Messages: []message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 1.0,
		TopP:        0.95,
		MaxTokens:   4096,
		Stream:      stream,
	}
	return body
}

func (c *Client) generateStream(ctx context.Context, prompt string, onChunk func(string) error) (string, error) {
	encoded, err := json.Marshal(c.chatCompletionRequest(defaultSystemPrompt, prompt, true))
	if err != nil {
		return "", fmt.Errorf("encode llama.cpp request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		return "", fmt.Errorf("create llama.cpp stream request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")

	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("call llama.cpp stream: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("call llama.cpp stream: unexpected status %s", response.Status)
	}

	var builder strings.Builder
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var chunk chatCompletionStreamResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return builder.String(), fmt.Errorf("decode llama.cpp stream chunk: %w", err)
		}
		for _, choice := range chunk.Choices {
			content := choice.Delta.Content
			if content == "" {
				content = choice.Message.Content
			}
			if content == "" {
				continue
			}
			builder.WriteString(content)
			if onChunk != nil {
				if err := onChunk(content); err != nil {
					return builder.String(), err
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return builder.String(), fmt.Errorf("read llama.cpp stream: %w", err)
	}
	content := strings.TrimSpace(builder.String())
	if content == "" {
		return "", fmt.Errorf("llama.cpp stream response was empty")
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
		if c.fallback != nil && ctx.Err() == nil {
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

type chatCompletionStreamResponse struct {
	Choices []struct {
		Delta   message `json:"delta"`
		Message message `json:"message"`
	} `json:"choices"`
}

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}
