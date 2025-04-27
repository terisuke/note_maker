package analyzer

import (
	"context"
	"fmt"

	"github.com/teradakousuke/note_maker/internal/services/note"
)

// Analyzer は記事の文体分析を行うサービス
type Analyzer struct {
	fetcher       *note.Fetcher
	styleAnalyzer *StyleAnalyzer
}

// NewAnalyzer は新しいAnalyzerを作成
func NewAnalyzer(fetcher *note.Fetcher) *Analyzer {
	return &Analyzer{
		fetcher:       fetcher,
		styleAnalyzer: NewStyleAnalyzer(),
	}
}

// AnalyzeUserStyle はユーザーの文体を分析
func (a *Analyzer) AnalyzeUserStyle(ctx context.Context, userID string) (*StyleAnalysis, error) {
	articles, err := a.fetcher.FetchUserLatestArticles(userID, 5)
	if err != nil {
		return nil, err
	}

	// 記事の内容を[]stringに変換
	var contents []string
	for _, article := range articles {
		if article.Content != "" {
			contents = append(contents, article.Content)
		}
	}

	if len(contents) == 0 {
		return &StyleAnalysis{
			ParagraphLength:       "medium",
			AverageSentenceLength: 0,
			Tone:                  "formal",
			CommonExpressions:     []string{},
		}, nil
	}

	analysis := a.styleAnalyzer.AnalyzeWritingStyle(contents)
	return analysis, nil
}

// AnalyzeArticleStyle は記事の文体を分析
func (a *Analyzer) AnalyzeArticleStyle(ctx context.Context, url string) (*StyleAnalysis, error) {
	article, err := a.fetcher.FetchArticle(url)
	if err != nil {
		return nil, err
	}

	if article.Content == "" {
		return &StyleAnalysis{
			ParagraphLength:       "medium",
			AverageSentenceLength: 0,
			Tone:                  "formal",
			CommonExpressions:     []string{},
		}, nil
	}

	analysis := a.styleAnalyzer.AnalyzeWritingStyle([]string{article.Content})
	return analysis, nil
}

// CompareStyles は2つの記事の文体を比較
func (a *Analyzer) CompareStyles(ctx context.Context, url1, url2 string) (*StyleComparison, error) {
	articles, err := a.fetcher.FetchMultipleArticles([]string{url1, url2})
	if err != nil {
		return nil, err
	}

	if len(articles) != 2 {
		return nil, fmt.Errorf("2つの記事が必要です")
	}

	analysis1 := a.styleAnalyzer.AnalyzeWritingStyle([]string{articles[0].Content})
	analysis2 := a.styleAnalyzer.AnalyzeWritingStyle([]string{articles[1].Content})

	return &StyleComparison{
		Article1: analysis1,
		Article2: analysis2,
	}, nil
}

// GetStyleRecommendations は文体改善の提案を生成
func (a *Analyzer) GetStyleRecommendations(ctx context.Context, analysis *StyleAnalysis) []string {
	var recommendations []string

	// 文の長さに基づく提案
	if analysis.AverageSentenceLength > 50 {
		recommendations = append(recommendations, "文が長くなりすぎています。短く分割することを検討してください。")
	}

	// トーンに基づく提案
	switch analysis.Tone {
	case "casual":
		recommendations = append(recommendations, "より丁寧な表現を使用することをお勧めします。")
	case "formal":
		recommendations = append(recommendations, "読者に親しみやすい表現を取り入れることを検討してください。")
	}

	// 段落の長さに基づく提案
	if analysis.ParagraphLength == "long" {
		recommendations = append(recommendations, "段落を短く分割して、読みやすさを改善することをお勧めします。")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "現在の文体は適切です。このまま続けてください。")
	}

	return recommendations
}
