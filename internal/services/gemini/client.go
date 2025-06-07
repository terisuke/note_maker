package gemini

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/genai"
)

// Client はGemini APIクライアント
type Client struct {
	client *genai.Client
	ctx    context.Context
}

// NewClient は新しいGemini APIクライアントを作成
func NewClient() (*Client, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY environment variable is not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}, nil
}

// GenerateContent はGemini APIを使用してコンテンツを生成
func (c *Client) GenerateContent(prompt string) (string, error) {
	temperature := float32(0.7)
	topP := float32(0.95)
	topK := float32(40)
	maxOutputTokens := int32(8192)

	result, err := c.client.Models.GenerateContent(
		c.ctx,
		"gemini-2.5-pro-preview-06-05",
		genai.Text(prompt),
		&genai.GenerateContentConfig{
			Temperature:     &temperature,
			MaxOutputTokens: maxOutputTokens,
			TopP:            &topP,
			TopK:            &topK,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(result.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	candidate := result.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("no parts in content")
	}

	part := candidate.Content.Parts[0]
	if part.Text == "" {
		return "", fmt.Errorf("no text in part")
	}

	return part.Text, nil
}

// ListModels は利用可能なモデルのリストを取得
func (c *Client) ListModels() ([]string, error) {
	models, err := c.client.Models.List(c.ctx, &genai.ListModelsConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}

	var modelNames []string
	for {
		model, err := models.Next(c.ctx)
		if err != nil {
			break
		}
		modelNames = append(modelNames, model.Name)
	}

	return modelNames, nil
}
