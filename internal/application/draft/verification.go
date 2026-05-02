package draft

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// TextVerificationModel generates a final consistency review from a compact prompt.
type TextVerificationModel interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// SystemPromptVerificationModel can suppress model reasoning with a verifier-specific system prompt.
type SystemPromptVerificationModel interface {
	GenerateWithSystem(ctx context.Context, systemPrompt, prompt string) (string, error)
}

// LightweightVerifier asks a separate lightweight model to review the final draft.
type LightweightVerifier struct {
	model TextVerificationModel
}

// NewLightweightVerifier creates a lightweight final consistency verifier.
func NewLightweightVerifier(model TextVerificationModel) *LightweightVerifier {
	return &LightweightVerifier{model: model}
}

// VerifyDraft checks whether the final draft is consistent with the brief, style guide, and format.
func (v *LightweightVerifier) VerifyDraft(ctx context.Context, req VerificationRequest) (FinalVerification, error) {
	if v == nil || v.model == nil {
		return FinalVerification{}, fmt.Errorf("verification model is required")
	}
	prompt := BuildFinalVerificationPrompt(req)
	var report string
	var err error
	if model, ok := v.model.(SystemPromptVerificationModel); ok {
		report, err = model.GenerateWithSystem(ctx, finalVerificationSystemPrompt, prompt)
	} else {
		report, err = v.model.Generate(ctx, prompt)
	}
	if err != nil {
		return FinalVerification{}, err
	}
	return ParseFinalVerificationReport(report), nil
}

const finalVerificationSystemPrompt = `<|nothink|>
あなたは日本語記事の最終検証者です。思考過程を出さず、指定された検証結果だけを返してください。`

// BuildFinalVerificationPrompt builds a compact Markdown review prompt.
func BuildFinalVerificationPrompt(req VerificationRequest) string {
	return strings.TrimSpace(fmt.Sprintf(`あなたは日本語記事の最終検証者です。
下書きを書き換えず、公開前チェックだけを行ってください。

出力ルール:
- 1行目は必ず PASS または NEEDS_REVIEW のどちらかにする
- 2行目は Summary: から始め、50字以内で要約する
- 問題がある場合は "- " 箇条書きで具体的に書く
- 本文の再生成、前置き、コードフェンスは禁止

検証観点:
1. 記事ブリーフの要件を満たしているか
2. 文体ガイドと一人称が大きく外れていないか
3. 出力先のMarkdown/記法ルールに違反していないか
4. 事実として与えられていない内容を断定していないか
5. 論理の飛躍、矛盾、読者に誤解される表現がないか

出力先:
%s

記事ブリーフ:
- Theme: %s
- Reader: %s
- Expected reader action: %s
- Must include: %s
- Exclusions: %s
- Tone stance: %s

文体ガイド:
%s

機械評価:
- passed: %v
- score: %.1f
- failures: %s

下書き:
%s
`,
		req.OutputFormat.DisplayName,
		req.Brief.Theme,
		req.Brief.Reader,
		req.Brief.ExpectedReaderAction,
		req.Brief.MustInclude,
		req.Brief.Exclusions,
		req.Brief.ToneStance,
		req.StyleGuide.Markdown,
		req.Evaluation.Passed,
		req.Evaluation.Comparison.Score,
		strings.Join(req.Evaluation.Failures, " / "),
		req.DraftMarkdown,
	))
}

// ParseFinalVerificationReport normalizes the lightweight model's Markdown report.
func ParseFinalVerificationReport(report string) FinalVerification {
	report = strings.TrimSpace(report)
	if report == "" {
		return FinalVerification{Performed: true, Passed: false, Summary: "empty verification report", Failures: []string{"empty verification report"}}
	}
	lines := nonEmptyLines(report)
	first := strings.ToUpper(strings.TrimSpace(lines[0]))
	passed := strings.HasPrefix(first, "PASS")
	failures := extractVerificationFailures(lines)
	summary := extractVerificationSummary(lines)
	if !passed && len(failures) == 0 {
		failures = []string{"verification marked the draft as needing review"}
	}
	return FinalVerification{
		Performed: true,
		Passed:    passed,
		Summary:   summary,
		Report:    report,
		Failures:  failures,
	}
}

func nonEmptyLines(value string) []string {
	raw := strings.Split(value, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		if cleaned := strings.TrimSpace(line); cleaned != "" {
			lines = append(lines, cleaned)
		}
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func extractVerificationSummary(lines []string) string {
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(line), "summary:") {
			return strings.TrimSpace(line[len("summary:"):])
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(regexp.MustCompile(`^(PASS|NEEDS_REVIEW)\s*:?\s*`).ReplaceAllString(lines[0], ""))
	}
	return ""
}

func extractVerificationFailures(lines []string) []string {
	failures := make([]string, 0)
	for _, line := range lines {
		cleaned := strings.TrimSpace(line)
		if strings.HasPrefix(cleaned, "- ") {
			failures = append(failures, strings.TrimSpace(strings.TrimPrefix(cleaned, "- ")))
		}
	}
	return failures
}
