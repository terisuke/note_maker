package draft

import (
	"fmt"
	"strings"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

// EvaluateStyle compares a validated draft against the author's style profile.
func EvaluateStyle(profile AuthorStyleProfile, brief ArticleBrief, articleDraft articledomain.Draft) StyleEvaluation {
	cloudiaStyle := isCloudiaStyleEvaluation(profile, brief)
	styleMarkdown := articleDraft.Markdown()
	if cloudiaStyle {
		styleMarkdown = styleEvaluationMarkdown(styleMarkdown, brief.OutputFormatID)
	}
	candidate := articledomain.AnalyzeStyle(styleMarkdown)
	comparison := articledomain.CompareStyle(profile.Metrics, candidate)

	evaluation := StyleEvaluation{
		Passed:              true,
		Comparison:          comparison,
		Thresholds:          StrictThresholds,
		RequiredFirstPerson: requiredFirstPerson(profile, brief),
	}

	if comparison.Score < StrictThresholds.TotalScore {
		evaluation.addFailure("total_style_score", comparison.Score, StrictThresholds.TotalScore)
	}
	evaluation.checkMetric("paragraph_length", StrictThresholds.ParagraphLength)
	evaluation.checkMetric("sentence_length", StrictThresholds.SentenceLength)
	evaluation.checkMetric("keyword_overlap", StrictThresholds.KeywordOverlap)
	evaluation.checkMetric("quote_density", StrictThresholds.QuoteDensity)
	evaluation.checkMetric("first_person", StrictThresholds.FirstPerson)

	if evaluation.RequiredFirstPerson != "" && !strings.Contains(styleMarkdown, evaluation.RequiredFirstPerson) {
		evaluation.Passed = false
		evaluation.Failures = append(evaluation.Failures, fmt.Sprintf("preferred_first_person=%q missing", evaluation.RequiredFirstPerson))
	}
	evaluation.checkCloudiaPersonaSignal(cloudiaStyle, styleMarkdown)

	if len(evaluation.Failures) > 0 {
		evaluation.Passed = false
	}
	return evaluation
}

func (e *StyleEvaluation) checkMetric(metric string, threshold int) {
	value, ok := e.Comparison.MetricScores[metric]
	if !ok {
		e.Passed = false
		e.Failures = append(e.Failures, fmt.Sprintf("%s missing", metric))
		return
	}
	if value < threshold {
		e.Passed = false
		e.Failures = append(e.Failures, fmt.Sprintf("%s=%d below %d", metric, value, threshold))
	}
}

func (e *StyleEvaluation) addFailure(metric string, value, threshold float64) {
	e.Passed = false
	e.Failures = append(e.Failures, fmt.Sprintf("%s=%.1f below %.1f", metric, value, threshold))
}

func (e *StyleEvaluation) checkCloudiaPersonaSignal(cloudiaStyle bool, styleMarkdown string) {
	if !cloudiaStyle {
		return
	}
	if containsAny(styleMarkdown, []string{"クラウディア", "うち", "ばい", "とよ", "やけん", "なんしよっと"}) {
		return
	}
	e.Passed = false
	e.Failures = append(e.Failures, "cloudia_first_person_signal missing from article body")
}

func isCloudiaStyleEvaluation(profile AuthorStyleProfile, brief ArticleBrief) bool {
	if personadomain.NormalizeID(brief.PersonaID) != personadomain.IDCloudia {
		return false
	}
	profileHints := []string{profile.ID, profile.Source.Username, profile.PreferredFirstPerson}
	for _, article := range profile.Source.Articles {
		profileHints = append(profileHints, article.ID, article.URL, article.Title)
	}
	return containsAny(strings.ToLower(strings.Join(profileHints, "\n")), []string{"cloudia", "クラウディア", "うち"})
}

func requiredFirstPerson(profile AuthorStyleProfile, brief ArticleBrief) string {
	if override := explicitFirstPersonOverride(brief); override != "" {
		return override
	}
	return normalizedFirstPerson(profile.PreferredFirstPerson)
}

func styleEvaluationMarkdown(markdown, formatID string) string {
	markdown = strings.ReplaceAll(markdown, "\r\n", "\n")
	markdown = strings.ReplaceAll(markdown, "\r", "\n")
	markdown = stripYAMLFrontmatter(markdown)
	markdown = stripCodeFences(markdown)
	if outputformat.NormalizeID(formatID) == outputformat.IDZennArticle {
		markdown = stripZennBoilerplate(markdown)
	}
	return strings.TrimSpace(markdown)
}

func stripYAMLFrontmatter(markdown string) string {
	if !strings.HasPrefix(strings.TrimSpace(markdown), "---\n") {
		return markdown
	}
	text := strings.TrimSpace(markdown)
	if end := strings.Index(text[4:], "\n---"); end >= 0 {
		rest := text[4+end+len("\n---"):]
		return strings.TrimLeft(rest, "\n")
	}
	return markdown
}

func stripCodeFences(markdown string) string {
	lines := strings.Split(markdown, "\n")
	cleaned := make([]string, 0, len(lines))
	inFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func stripZennBoilerplate(markdown string) string {
	lines := strings.Split(markdown, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, ":::"):
			continue
		case strings.HasPrefix(trimmed, "@[card]("), strings.HasPrefix(trimmed, "@[gist]("):
			continue
		default:
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}

func containsAny(text string, values []string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
