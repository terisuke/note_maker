package draft

import (
	"fmt"
	"strings"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
)

// EvaluateStyle compares a validated draft against the author's style profile.
func EvaluateStyle(profile AuthorStyleProfile, brief ArticleBrief, articleDraft articledomain.Draft) StyleEvaluation {
	candidate := articledomain.AnalyzeStyle(articleDraft.Markdown())
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

	if evaluation.RequiredFirstPerson != "" && !strings.Contains(articleDraft.Markdown(), evaluation.RequiredFirstPerson) {
		evaluation.Passed = false
		evaluation.Failures = append(evaluation.Failures, fmt.Sprintf("preferred_first_person=%q missing", evaluation.RequiredFirstPerson))
	}

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

func requiredFirstPerson(profile AuthorStyleProfile, brief ArticleBrief) string {
	if override := explicitFirstPersonOverride(brief); override != "" {
		return override
	}
	return normalizedFirstPerson(profile.PreferredFirstPerson)
}
