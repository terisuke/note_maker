package draft

import (
	"strings"
	"testing"

	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
)

func TestParseFinalVerificationReportPass(t *testing.T) {
	report := "PASS\nSummary: 要件に沿っています"
	verification := ParseFinalVerificationReport(report)
	if !verification.Performed || !verification.Passed {
		t.Fatalf("unexpected verification: %#v", verification)
	}
	if verification.Summary != "要件に沿っています" {
		t.Fatalf("unexpected summary: %q", verification.Summary)
	}
}

func TestParseFinalVerificationReportNeedsReview(t *testing.T) {
	report := "NEEDS_REVIEW\nSummary: 根拠が不足しています\n- 実測値の根拠が本文にありません\n- 除外条件に触れています"
	verification := ParseFinalVerificationReport(report)
	if verification.Passed {
		t.Fatalf("verification should fail: %#v", verification)
	}
	if len(verification.Failures) != 2 {
		t.Fatalf("unexpected failures: %#v", verification.Failures)
	}
}

func TestBuildFinalVerificationPromptIncludesZennAllowedNotation(t *testing.T) {
	format := outputformat.DefaultRegistry().MustGet(outputformat.IDZennArticle)
	prompt := BuildFinalVerificationPrompt(VerificationRequest{
		OutputFormat: format,
		Brief: ArticleBrief{
			Theme: "Zenn記法を検証する",
		},
		DraftMarkdown: "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n## 本文\n\n:::message alert\n注意\n:::\n\n:::details 補足\n本文\n:::",
	})

	for _, want := range []string{
		"Use this guide only for `zenn_article` output.",
		"Warnings: `:::message alert`.",
		"Collapsible details: `:::details`.",
		"選択された出力先で許可された記法は問題扱いしない",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("verification prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuildFinalVerificationPromptIncludesQiitaForbiddenDistinction(t *testing.T) {
	format := outputformat.DefaultRegistry().MustGet(outputformat.IDQiitaArticle)
	prompt := BuildFinalVerificationPrompt(VerificationRequest{
		OutputFormat: format,
		Brief: ArticleBrief{
			Theme: "Qiita記法を検証する",
		},
		DraftMarkdown: "---\ntitle: \"T\"\ntags:\n  - Go\n---\n\n## 本文\n\n:::note warn\n注意\n:::\n",
	})

	for _, want := range []string{
		"Use this guide only for `qiita_article` output.",
		"`:::note warn`",
		"Zenn `:::message`.",
		"Zenn `:::details`.",
		"Zenn diff fences such as `diff ts`.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("verification prompt missing %q:\n%s", want, prompt)
		}
	}
	for _, notWant := range []string{
		"Warnings: `:::message alert`.",
		"Collapsible details: `:::details`.",
	} {
		if strings.Contains(prompt, notWant) {
			t.Fatalf("verification prompt included Zenn allowed guidance %q:\n%s", notWant, prompt)
		}
	}
}
