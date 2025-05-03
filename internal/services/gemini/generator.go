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

	// 役割設定
	prompt.WriteString("あなたは、読者の関心を引きつけ、分かりやすく情報を伝えるプロのコンテンツライターです。\n")
	if targetAudience != "" {
		prompt.WriteString(fmt.Sprintf("特に、%s向けの解説を得意としています。\n", targetAudience))
	}
	prompt.WriteString("\n")

	// タスク定義
	prompt.WriteString("以下の入力コンテキストに基づき、新しい記事を作成してください。\n\n")

	// 入力コンテキスト
	prompt.WriteString("--- 入力コンテキスト ---\n")

	// 参考記事がある場合は追加
	if len(referenceArticles) > 0 {
		prompt.WriteString("参考記事:\n")
		for i, article := range referenceArticles {
			prompt.WriteString(fmt.Sprintf("記事 %d:\n%s\n\n", i+1, article))
		}
	}

	// ユーザー指示
	prompt.WriteString("記事作成の指示:\n")
	if len(keywords) > 0 {
		prompt.WriteString(fmt.Sprintf("キーワード: %s\n", strings.Join(keywords, ", ")))
	}
	if theme != "" {
		prompt.WriteString(fmt.Sprintf("テーマ: %s\n", theme))
	}
	if targetAudience != "" {
		prompt.WriteString(fmt.Sprintf("想定読者層: %s\n", targetAudience))
	}
	if styleChoice != "" {
		prompt.WriteString(fmt.Sprintf("文体: %s\n", styleChoice))
	}
	if toneChoice != "" {
		prompt.WriteString(fmt.Sprintf("トーン: %s\n", toneChoice))
	}
	if wordCount > 0 {
		prompt.WriteString(fmt.Sprintf("目標文字数: %d\n", wordCount))
	}
	if articlePurpose != "" {
		prompt.WriteString(fmt.Sprintf("記事の目的: %s\n", articlePurpose))
	}
	if desiredContent != "" {
		prompt.WriteString(fmt.Sprintf("含めたい内容: %s\n", desiredContent))
	}
	if introductionPoints != "" {
		prompt.WriteString(fmt.Sprintf("導入部分のポイント: %s\n", introductionPoints))
	}
	if mainPoints != "" {
		prompt.WriteString(fmt.Sprintf("本論のポイント: %s\n", mainPoints))
	}
	if conclusionMessage != "" {
		prompt.WriteString(fmt.Sprintf("結論のメッセージ: %s\n", conclusionMessage))
	}
	if exclusions != "" {
		prompt.WriteString(fmt.Sprintf("含めない内容: %s\n", exclusions))
	}
	prompt.WriteString("---\n\n")

	// 出力指示
	prompt.WriteString("--- 出力指示 ---\n")
	prompt.WriteString("以下の点に注意して記事を作成してください：\n\n")

	prompt.WriteString("1. 文章スタイルについて\n")
	prompt.WriteString("- 箇条書きは最小限に抑え、文章で説明を加える\n")
	prompt.WriteString("- 行頭の太字は控えめに使用する\n")
	prompt.WriteString("- 見出しにはコロンを使用せず、シンプルな表現を心がける\n")
	prompt.WriteString("- 「〜すること」「〜することによる」などの抽象名詞による体言止めを避け、具体的な表現を使用する\n\n")

	prompt.WriteString("2. 文章の構成について\n")
	prompt.WriteString("- 各セクションの冒頭に、その内容を説明する導入文を入れる\n")
	prompt.WriteString("- 箇条書きが必要な場合は、その前後に説明文を追加する\n")
	prompt.WriteString("- 見出しは質問形式や具体的な表現を使用する\n")
	prompt.WriteString("- 文章の流れを重視し、自然な接続を心がける\n\n")

	prompt.WriteString("3. 表現方法について\n")

	// ユーザーが選択した文体に基づいて指示を変更
	if styleChoice == "ですます調" {
		prompt.WriteString("- 「〜です」「〜ます」調を基本とし、一貫性を保つ\n")
	} else if styleChoice == "である調" {
		prompt.WriteString("- 「〜である」「〜だ」調を基本とし、一貫性を保つ\n")
	} else {
		prompt.WriteString("- 選択された文体（" + styleChoice + "）を基本とし、一貫性を保つ\n")
	}

	prompt.WriteString("- 専門用語は必要に応じて説明を加える\n")
	prompt.WriteString("- 具体例を交えながら説明を展開する\n")
	prompt.WriteString("- 読者への問いかけや対話的な表現を取り入れる\n\n")

	prompt.WriteString("4. 全体的な注意点\n")
	prompt.WriteString("- 文章の長さは適度に保ち、読みやすさを重視する\n")
	prompt.WriteString("- 重要なポイントは強調するが、過度な装飾は避ける\n")
	prompt.WriteString("- 情報の階層構造を明確にし、理解しやすい構成を心がける\n")
	prompt.WriteString("- 読者の興味を引く導入部から始め、自然な流れで結論へと導く\n\n")

	prompt.WriteString("それでは、上記の指示に基づいて記事の作成を開始してください。\n")

	// プロンプトをログに出力
	log.Printf("Gemini APIに送信するプロンプト:\n%s", prompt.String())

	// Gemini APIを使用してコンテンツを生成
	content, err := g.client.GenerateContent(prompt.String())
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	return content, nil
}
