package article

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

var sentenceSplitPattern = regexp.MustCompile(`[。！？!?]\s*`)

var defaultStyleKeywords = []string{
	"僕",
	"私",
	"起業",
	"音楽",
	"エンジニア",
	"AI",
	"アウトプット",
	"LT",
	"挑戦",
	"救い",
	"違和感",
	"言語化",
	"自分",
}

// StyleProfile captures measurable writing-style signals.
type StyleProfile struct {
	CharCount             int            `json:"char_count"`
	ParagraphCount        int            `json:"paragraph_count"`
	SentenceCount         int            `json:"sentence_count"`
	AverageSentenceRunes  float64        `json:"average_sentence_runes"`
	AverageParagraphRunes float64        `json:"average_paragraph_runes"`
	HeadingCount          int            `json:"heading_count"`
	QuoteCount            int            `json:"quote_count"`
	FirstPersonCount      int            `json:"first_person_count"`
	KeywordCounts         map[string]int `json:"keyword_counts"`
}

// StyleComparison compares a generated draft with a reference corpus.
type StyleComparison struct {
	Score          float64        `json:"score"`
	Reference      StyleProfile   `json:"reference"`
	Candidate      StyleProfile   `json:"candidate"`
	MatchedSignals []string       `json:"matched_signals"`
	Risks          []string       `json:"risks"`
	MetricScores   map[string]int `json:"metric_scores"`
}

// AnalyzeStyle returns objective style signals for Japanese note-style text.
func AnalyzeStyle(text string) StyleProfile {
	normalized := normalizeStyleText(text)
	paragraphs := splitParagraphs(normalized)
	sentences := splitSentences(normalized)

	profile := StyleProfile{
		CharCount:        utf8.RuneCountInString(removeMarkdownMarkup(normalized)),
		ParagraphCount:   len(paragraphs),
		SentenceCount:    len(sentences),
		HeadingCount:     countHeadingLines(normalized),
		QuoteCount:       strings.Count(normalized, "「") + strings.Count(normalized, "『"),
		FirstPersonCount: strings.Count(normalized, "僕") + strings.Count(normalized, "私") + strings.Count(normalized, "俺"),
		KeywordCounts:    make(map[string]int, len(defaultStyleKeywords)),
	}
	for _, paragraph := range paragraphs {
		profile.AverageParagraphRunes += float64(utf8.RuneCountInString(removeMarkdownMarkup(paragraph)))
	}
	if profile.ParagraphCount > 0 {
		profile.AverageParagraphRunes /= float64(profile.ParagraphCount)
	}
	for _, sentence := range sentences {
		profile.AverageSentenceRunes += float64(utf8.RuneCountInString(removeMarkdownMarkup(sentence)))
	}
	if profile.SentenceCount > 0 {
		profile.AverageSentenceRunes /= float64(profile.SentenceCount)
	}
	for _, keyword := range defaultStyleKeywords {
		profile.KeywordCounts[keyword] = strings.Count(normalized, keyword)
	}
	return profile
}

// AnalyzeStyleCorpus aggregates multiple reference articles into one profile.
func AnalyzeStyleCorpus(articles []Article) StyleProfile {
	parts := make([]string, 0, len(articles))
	for _, article := range articles {
		if strings.TrimSpace(article.Title) != "" {
			parts = append(parts, "# "+article.Title)
		}
		if strings.TrimSpace(article.Content) != "" {
			parts = append(parts, article.Content)
		}
	}
	return AnalyzeStyle(strings.Join(parts, "\n\n"))
}

// CompareStyle scores whether the candidate resembles the reference style.
func CompareStyle(reference, candidate StyleProfile) StyleComparison {
	metrics := map[string]int{
		"paragraph_length":  int(math.Round(100 * ratioScore(reference.AverageParagraphRunes, candidate.AverageParagraphRunes))),
		"sentence_length":   int(math.Round(100 * ratioScore(reference.AverageSentenceRunes, candidate.AverageSentenceRunes))),
		"heading_structure": markdownHeadingScore(candidate.HeadingCount),
		"quote_density":     int(math.Round(100 * densityScore(reference.QuoteCount, reference.CharCount, candidate.QuoteCount, candidate.CharCount))),
		"first_person":      int(math.Round(100 * densityScore(reference.FirstPersonCount, reference.CharCount, candidate.FirstPersonCount, candidate.CharCount))),
		"keyword_overlap":   int(math.Round(100 * keywordOverlap(reference.KeywordCounts, candidate.KeywordCounts))),
	}

	weights := map[string]float64{
		"paragraph_length":  0.18,
		"sentence_length":   0.18,
		"heading_structure": 0.12,
		"quote_density":     0.16,
		"first_person":      0.16,
		"keyword_overlap":   0.20,
	}
	var score float64
	for metric, value := range metrics {
		score += float64(value) * weights[metric]
	}

	comparison := StyleComparison{
		Score:        math.Round(score*10) / 10,
		Reference:    reference,
		Candidate:    candidate,
		MetricScores: metrics,
	}
	for _, metric := range sortedMetricNames(metrics) {
		value := metrics[metric]
		if value >= 75 {
			comparison.MatchedSignals = append(comparison.MatchedSignals, fmt.Sprintf("%s=%d", metric, value))
		} else if value < 55 {
			comparison.Risks = append(comparison.Risks, fmt.Sprintf("%s=%d", metric, value))
		}
	}
	if candidate.CharCount < 800 {
		comparison.Risks = append(comparison.Risks, "candidate_length_too_short")
	}
	if candidate.HeadingCount == 0 {
		comparison.Risks = append(comparison.Risks, "candidate_has_no_markdown_headings")
	}
	return comparison
}

func normalizeStyleText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	normalized := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line == "" {
			if !blank {
				normalized = append(normalized, "")
			}
			blank = true
			continue
		}
		blank = false
		normalized = append(normalized, line)
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func splitParagraphs(text string) []string {
	blocks := strings.Split(text, "\n\n")
	paragraphs := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block != "" {
			paragraphs = append(paragraphs, block)
		}
	}
	if len(paragraphs) == 0 && strings.TrimSpace(text) != "" {
		return []string{strings.TrimSpace(text)}
	}
	return paragraphs
}

func splitSentences(text string) []string {
	raw := sentenceSplitPattern.Split(text, -1)
	sentences := make([]string, 0, len(raw))
	for _, sentence := range raw {
		sentence = strings.TrimSpace(sentence)
		if sentence != "" {
			sentences = append(sentences, sentence)
		}
	}
	return sentences
}

func countHeadingLines(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			count++
		}
	}
	return count
}

func removeMarkdownMarkup(text string) string {
	replacer := strings.NewReplacer("#", "", "*", "", "`", "", "-", "")
	return replacer.Replace(text)
}

func ratioScore(reference, candidate float64) float64 {
	if reference <= 0 && candidate <= 0 {
		return 1
	}
	if reference <= 0 || candidate <= 0 {
		return 0
	}
	delta := math.Abs(reference - candidate)
	return math.Max(0, 1-delta/math.Max(reference, candidate))
}

func densityScore(referenceCount, referenceChars, candidateCount, candidateChars int) float64 {
	if referenceChars <= 0 || candidateChars <= 0 {
		return 0
	}
	referenceDensity := float64(referenceCount) / float64(referenceChars)
	candidateDensity := float64(candidateCount) / float64(candidateChars)
	return ratioScore(referenceDensity, candidateDensity)
}

func keywordOverlap(reference, candidate map[string]int) float64 {
	if len(reference) == 0 {
		return 1
	}
	total := 0
	matched := 0
	for keyword, referenceCount := range reference {
		if referenceCount <= 0 {
			continue
		}
		total++
		if candidate[keyword] > 0 {
			matched++
		}
	}
	if total == 0 {
		return 1
	}
	return float64(matched) / float64(total)
}

func markdownHeadingScore(headings int) int {
	switch {
	case headings >= 3 && headings <= 6:
		return 100
	case headings == 2 || headings == 7:
		return 75
	case headings == 1 || headings == 8:
		return 50
	default:
		return 0
	}
}

func sortedMetricNames(metrics map[string]int) []string {
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
