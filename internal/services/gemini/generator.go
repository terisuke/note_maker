package gemini

import (
	"fmt"
	"log"
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
func (g *Generator) GenerateArticle(
	referenceArticles []string,
	keywords []string,
	theme,
	targetAudience,
	exclusions,
	styleChoice,
	toneChoice string,
	wordCount int,
	articlePurpose,
	desiredContent,
	introductionPoints,
	mainPoints,
	conclusionMessage string,
) (string, error) {
	// プロンプトの構築
	var prompt strings.Builder

	// 参考記事がある場合は追加
	if len(referenceArticles) > 0 {
		prompt.WriteString("以下の参考記事を基に、新しい記事を生成してください。\n\n")

		// 参考記事の追加
		for i, article := range referenceArticles {
			prompt.WriteString(fmt.Sprintf("参考記事 %d:\n%s\n\n", i+1, article))
		}
	} else {
		prompt.WriteString("新しい記事を生成してください。\n\n")
	}

	// 指示の追加
	prompt.WriteString("以下の指示に従って記事を生成してください：\n")
	prompt.WriteString(fmt.Sprintf("- キーワード: %s\n", strings.Join(keywords, ", ")))
	prompt.WriteString(fmt.Sprintf("- テーマ: %s\n", theme))
	prompt.WriteString(fmt.Sprintf("- 想定読者層: %s\n", targetAudience))
	prompt.WriteString(fmt.Sprintf("- 文体: %s\n", styleChoice))
	prompt.WriteString(fmt.Sprintf("- トーン: %s\n", toneChoice))
	prompt.WriteString(fmt.Sprintf("- 目標文字数: %d\n", wordCount))
	prompt.WriteString(fmt.Sprintf("- 記事の目的: %s\n", articlePurpose))

	if desiredContent != "" {
		prompt.WriteString(fmt.Sprintf("- 含めたい具体的な内容: %s\n", desiredContent))
	}

	if introductionPoints != "" {
		prompt.WriteString(fmt.Sprintf("- 導入部分で触れるポイント: %s\n", introductionPoints))
	}

	if mainPoints != "" {
		prompt.WriteString(fmt.Sprintf("- 本論で説明する項目: %s\n", mainPoints))
	}

	if conclusionMessage != "" {
		prompt.WriteString(fmt.Sprintf("- 結論で強調したいメッセージ: %s\n", conclusionMessage))
	}

	if exclusions != "" {
		prompt.WriteString(fmt.Sprintf("- 含めないでほしい内容: %s\n", exclusions))
	}

	// プロンプトをログに出力
	log.Printf("Gemini APIに送信するプロンプト:\n%s", prompt.String())

	// Gemini APIを使用してコンテンツを生成
	content, err := g.client.GenerateContent(prompt.String())
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	return content, nil
}
