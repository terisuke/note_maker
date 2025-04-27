package gemini

import (
	"fmt"
	"strings"
)

// Generator はGemini APIを使用して記事を生成するサービス
type Generator struct {
	client *Client
}

// NewGenerator は新しいGeneratorを作成
func NewGenerator() (*Generator, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	return &Generator{
		client: client,
	}, nil
}

// GenerateArticle はGemini APIを使用して記事を生成
func (g *Generator) GenerateArticle(referenceArticles []string, keywords []string, theme, targetAudience, exclusions, styleChoice, toneChoice string, wordCount int) (string, error) {
	// プロンプトの構築
	var prompt strings.Builder
	prompt.WriteString("以下の参考記事を基に、新しい記事を生成してください。\n\n")

	// 参考記事の追加
	for i, article := range referenceArticles {
		prompt.WriteString(fmt.Sprintf("参考記事 %d:\n%s\n\n", i+1, article))
	}

	// 指示の追加
	prompt.WriteString("以下の指示に従って記事を生成してください：\n")
	prompt.WriteString(fmt.Sprintf("- キーワード: %s\n", strings.Join(keywords, ", ")))
	prompt.WriteString(fmt.Sprintf("- テーマ: %s\n", theme))
	prompt.WriteString(fmt.Sprintf("- 想定読者層: %s\n", targetAudience))
	prompt.WriteString(fmt.Sprintf("- 文体: %s\n", styleChoice))
	prompt.WriteString(fmt.Sprintf("- トーン: %s\n", toneChoice))
	prompt.WriteString(fmt.Sprintf("- 目標文字数: %d\n", wordCount))

	if exclusions != "" {
		prompt.WriteString(fmt.Sprintf("- 含めないでほしい内容: %s\n", exclusions))
	}

	// Gemini APIを使用してコンテンツを生成
	content, err := g.client.GenerateContent(prompt.String())
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	return content, nil
}
