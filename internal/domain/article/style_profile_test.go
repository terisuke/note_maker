package article

import (
	"strings"
	"testing"
)

func TestAnalyzeStyleCountsJapaneseSignals(t *testing.T) {
	profile := AnalyzeStyle(`# タイトル

僕は音楽家からエンジニアになった。
「これは挑戦だ」と思った。

## 見出し

AIで自分の違和感を言語化する。`)

	if profile.ParagraphCount != 4 {
		t.Fatalf("unexpected paragraph count: %d", profile.ParagraphCount)
	}
	if profile.HeadingCount != 2 {
		t.Fatalf("unexpected heading count: %d", profile.HeadingCount)
	}
	if profile.FirstPersonCount == 0 || profile.QuoteCount == 0 {
		t.Fatalf("expected first-person and quote signals: %#v", profile)
	}
	if profile.KeywordCounts["音楽"] == 0 || profile.KeywordCounts["AI"] == 0 {
		t.Fatalf("expected keyword counts: %#v", profile.KeywordCounts)
	}
}

func TestAnalyzeStyleCountsCloudiaTechnicalSignals(t *testing.T) {
	profile := AnalyzeStyle(`# Zenn向けCLI

クラウディアはGoのCLIでZenn向けMarkdownとfrontmatterを検証するばい。
Qiitaと混ぜず、コード、実装手順、再現、初心者のつまずきを整理するとよ。`)

	for _, keyword := range []string{"クラウディア", "Go", "CLI", "Zenn", "Markdown", "frontmatter", "Qiita", "コード", "実装", "手順", "検証", "再現", "初心者", "つまずき", "ばい", "とよ"} {
		if profile.KeywordCounts[keyword] == 0 {
			t.Fatalf("expected keyword %q to be counted: %#v", keyword, profile.KeywordCounts)
		}
	}
}

func TestCompareStyleReportsRisks(t *testing.T) {
	reference := AnalyzeStyle("僕はAIと起業について書く。\n\n「違和感」を言語化する。")
	candidate := AnalyzeStyle("# Draft\n\nこれは短い説明です。")

	comparison := CompareStyle(reference, candidate)
	if comparison.Score <= 0 || comparison.Score >= 100 {
		t.Fatalf("unexpected score: %#v", comparison)
	}
	if len(comparison.Risks) == 0 {
		t.Fatalf("expected style risks: %#v", comparison)
	}
}

func TestCompareStyleScoresCloudiaTechnicalKeywordsAndLongOutline(t *testing.T) {
	reference := AnalyzeStyle(repeatedTechnicalSections(9))
	candidate := AnalyzeStyle(repeatedTechnicalSections(14))

	comparison := CompareStyle(reference, candidate)
	if comparison.MetricScores["keyword_overlap"] < 70 {
		t.Fatalf("keyword overlap score = %d, want Cloudia/Zenn technical signals to count: %#v", comparison.MetricScores["keyword_overlap"], comparison)
	}
	if comparison.MetricScores["heading_structure"] < 75 {
		t.Fatalf("heading score = %d, want long technical outline to remain reviewable", comparison.MetricScores["heading_structure"])
	}
}

func TestCompareStyleAllowsLongFormHeadingsAndLightQuotes(t *testing.T) {
	reference := AnalyzeStyle("# Reference\n\n" + repeatText("僕はAIと違和感を言語化する。\n\n", 8))
	candidate := AnalyzeStyle(`# Draft

## One

僕はAIと「違和感」を言語化する。

## Two

僕は自分の体験を説明する。

## Three

僕は音楽と起業をつなげる。

## Four

僕は読者に次の一歩を渡す。

## Five

僕は判断基準をまとめる。

## Six

僕は検証を続ける。

## Seven

僕は手触りを残す。

## Eight

僕は最後に問いを置く。`)

	comparison := CompareStyle(reference, candidate)
	if comparison.MetricScores["heading_structure"] != 100 {
		t.Fatalf("heading score = %d, want 100", comparison.MetricScores["heading_structure"])
	}
	if comparison.MetricScores["quote_density"] < 55 {
		t.Fatalf("quote density score = %d, want non-failing", comparison.MetricScores["quote_density"])
	}
}

func TestCompareStyleToleratesModerateFirstPersonOveruse(t *testing.T) {
	reference := AnalyzeStyle(repeatText("僕はAIと違和感を言語化する。\n\n", 8))
	candidate := AnalyzeStyle(repeatText("僕はAIと違和感を言語化する。僕は自分の体験を書く。僕は読者に渡す。\n\n", 8))

	comparison := CompareStyle(reference, candidate)
	if comparison.MetricScores["first_person"] < 55 {
		t.Fatalf("first person score = %d, want moderate overuse to remain reviewable", comparison.MetricScores["first_person"])
	}
}

func repeatedTechnicalSections(count int) string {
	var builder strings.Builder
	for i := 0; i < count; i++ {
		builder.WriteString("## 手順\n\n")
		builder.WriteString("クラウディアはGoのCLIでZenn向けMarkdownとfrontmatter、topics、コード、実装手順を検証するばい。")
		builder.WriteString("Qiitaと混ぜず、媒体別プロンプト、再現、初心者のつまずきを自分の手元で整理するとよ。\n\n")
	}
	return builder.String()
}

func repeatText(value string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += value
	}
	return result
}
