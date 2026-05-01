package article

import "testing"

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
