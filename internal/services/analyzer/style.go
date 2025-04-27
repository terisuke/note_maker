package analyzer

import (
	"regexp"
	"strings"
)

// StyleAnalyzer は文体分析を行うサービス
type StyleAnalyzer struct{}

// NewStyleAnalyzer は新しいStyleAnalyzerを作成
func NewStyleAnalyzer() *StyleAnalyzer {
	return &StyleAnalyzer{}
}

// AnalyzeWritingStyle は複数の記事本文から文体を分析
func (a *StyleAnalyzer) AnalyzeWritingStyle(articleBodies []string) *StyleAnalysis {
	analysis := &StyleAnalysis{
		CommonExpressions: []string{},
	}

	if len(articleBodies) == 0 {
		return analysis
	}

	// 全記事のテキストを結合
	combinedText := strings.Join(articleBodies, "\n\n")

	// 段落分析
	analysis.ParagraphLength = a.analyzeParagraphLength(combinedText)

	// 文分析
	avgLength := a.analyzeSentenceLength(combinedText)
	analysis.AverageSentenceLength = float64(avgLength)

	// よく使われる表現の分析
	analysis.CommonExpressions = a.analyzeCommonExpressions(combinedText)

	// トーン分析
	analysis.Tone = a.analyzeTone(combinedText)

	return analysis
}

// analyzeParagraphLength は段落の長さを分析
func (a *StyleAnalyzer) analyzeParagraphLength(text string) string {
	// 段落を分割
	paragraphs := strings.Split(text, "\n\n")

	// 空の段落を除外
	var validParagraphs []string
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			validParagraphs = append(validParagraphs, p)
		}
	}

	if len(validParagraphs) == 0 {
		return "medium"
	}

	// 平均段落長を計算
	var totalLength int
	for _, p := range validParagraphs {
		totalLength += len(p)
	}
	avgLength := totalLength / len(validParagraphs)

	// 段落長の判定
	if avgLength < 100 {
		return "short"
	} else if avgLength > 300 {
		return "long"
	} else {
		return "medium"
	}
}

// analyzeSentenceLength は文の長さを分析
func (a *StyleAnalyzer) analyzeSentenceLength(text string) int {
	// 文を分割（句点、感嘆符、疑問符で区切る）
	sentenceRegex := regexp.MustCompile(`[。！？]`)
	sentences := sentenceRegex.Split(text, -1)

	// 空の文を除外
	var validSentences []string
	for _, s := range sentences {
		if strings.TrimSpace(s) != "" {
			validSentences = append(validSentences, s)
		}
	}

	if len(validSentences) == 0 {
		return 0
	}

	// 平均文長を計算
	var totalLength int
	for _, s := range validSentences {
		totalLength += len(s)
	}

	return totalLength / len(validSentences)
}

// analyzeCommonExpressions はよく使われる表現を分析
func (a *StyleAnalyzer) analyzeCommonExpressions(text string) []string {
	// 文末表現のパターン
	endPatterns := []string{
		"です", "ます", "だ", "である", "でしょう", "かもしれません",
		"思います", "考えます", "感じます", "でしょう", "かもしれません",
	}

	// 各パターンの出現回数をカウント
	patternCounts := make(map[string]int)
	for _, pattern := range endPatterns {
		count := strings.Count(text, pattern)
		if count > 0 {
			patternCounts[pattern] = count
		}
	}

	// 出現回数の多い順にソート
	var result []string
	for pattern := range patternCounts {
		result = append(result, pattern)
	}

	// 上位3つを返す
	if len(result) > 3 {
		result = result[:3]
	}

	return result
}

// analyzeTone はトーンを分析
func (a *StyleAnalyzer) analyzeTone(text string) string {
	// 丁寧な表現のパターン
	politePatterns := []string{
		"です", "ます", "ください", "お願い", "申し訳", "恐れ入ります",
	}

	// カジュアルな表現のパターン
	casualPatterns := []string{
		"だ", "である", "だろう", "かもしれない", "思う", "感じる",
	}

	// 各パターンの出現回数をカウント
	politeCount := 0
	for _, pattern := range politePatterns {
		politeCount += strings.Count(text, pattern)
	}

	casualCount := 0
	for _, pattern := range casualPatterns {
		casualCount += strings.Count(text, pattern)
	}

	// トーンの判定
	if politeCount > casualCount*2 {
		return "polite"
	} else if casualCount > politeCount*2 {
		return "casual"
	} else {
		return "formal"
	}
}
