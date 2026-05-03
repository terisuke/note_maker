package draft

import (
	"strings"
	"testing"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestEvaluateStyleIgnoresZennBoilerplateForStyleMetrics(t *testing.T) {
	body := cloudiaTechnicalBody("クラウディアは", "ばい")
	profile := AuthorStyleProfile{
		ID:                   "profile-cloudia",
		Metrics:              articledomain.AnalyzeStyle(body),
		PreferredFirstPerson: "クラウディア",
	}
	raw := zennArticleWithBody(body) + "\n\n" +
		"```ts:src/main.ts\n" +
		"const persona = 'クラウディア';\n" +
		"console.log('AIの違和感を言語化するコード例');\n" +
		"```\n\n" +
		"@[card](https://example.com/cloudia-reference)"
	articleDraft, err := articledomain.NewDraftForFormat(raw, outputformat.IDZennArticle)
	if err != nil {
		t.Fatalf("new zenn draft: %v", err)
	}

	evaluation := EvaluateStyle(profile, ArticleBrief{
		PersonaID:      personadomain.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
	}, articleDraft)

	if !evaluation.Passed {
		t.Fatalf("expected evaluation to pass with Zenn boilerplate ignored: %#v", evaluation)
	}
	expected := articledomain.AnalyzeStyle(body)
	if evaluation.Comparison.Candidate.ParagraphCount != expected.ParagraphCount {
		t.Fatalf("candidate paragraphs = %d, want body-only %d", evaluation.Comparison.Candidate.ParagraphCount, expected.ParagraphCount)
	}
	if evaluation.Comparison.Candidate.KeywordCounts["AI"] != expected.KeywordCounts["AI"] {
		t.Fatalf("candidate AI count = %d, want body-only %d", evaluation.Comparison.Candidate.KeywordCounts["AI"], expected.KeywordCounts["AI"])
	}
}

func TestEvaluateStyleRequiresCloudiaSignalInArticleBodyNotFrontmatter(t *testing.T) {
	profile := AuthorStyleProfile{
		ID:                   "profile-cloudia",
		Metrics:              articledomain.AnalyzeStyle(cloudiaTechnicalBody("クラウディアは", "ばい")),
		PreferredFirstPerson: "明示しない",
	}
	neutralBody := cloudiaTechnicalBody("この記事では", "です")
	frontmatterOnlyDraft, err := articledomain.NewDraftForFormat(
		zennArticleWithBody(neutralBody),
		outputformat.IDZennArticle,
	)
	if err != nil {
		t.Fatalf("new zenn draft: %v", err)
	}

	evaluation := EvaluateStyle(profile, ArticleBrief{
		PersonaID:      personadomain.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
	}, frontmatterOnlyDraft)

	if evaluation.Passed {
		t.Fatalf("expected Cloudia signal failure when only frontmatter names Cloudia: %#v", evaluation)
	}
	if !failureContains(evaluation.Failures, "cloudia_first_person_signal missing from article body") {
		t.Fatalf("expected actionable Cloudia signal failure: %#v", evaluation.Failures)
	}

	bodySignalDraft, err := articledomain.NewDraftForFormat(
		zennArticleWithBody(strings.Replace(neutralBody, "この記事では", "うちは", 1)),
		outputformat.IDZennArticle,
	)
	if err != nil {
		t.Fatalf("new zenn draft with body signal: %v", err)
	}
	evaluation = EvaluateStyle(profile, ArticleBrief{
		PersonaID:      personadomain.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
	}, bodySignalDraft)

	if !evaluation.Passed {
		t.Fatalf("expected body Cloudia signal to satisfy persona check: %#v", evaluation)
	}
}

func cloudiaTechnicalBody(subject, ending string) string {
	intro := "## はじめに\n\n"
	intro += strings.Repeat(subject+"AIの実装で違和感を言語化しながら、自分の手元で小さく検証する"+ending+"。音楽の練習みたいに、まずログを見て挑戦の入口をそろえる"+ending+"。\n\n", 6)
	steps := "## 手順\n\n"
	steps += strings.Repeat(subject+"エンジニア向けに、アウトプットを急がず仮説、コード、結果を順番に見せる"+ending+"。起業の現場でも救いになる判断基準を残す"+ending+"。\n\n", 6)
	wrap := "## まとめ\n\n"
	wrap += strings.Repeat(subject+"最後にAIの結果を鵜呑みにせず、違和感をメモして次の自分に渡す"+ending+"。\n\n", 4)
	return intro + steps + wrap
}

func zennArticleWithBody(body string) string {
	return "---\n" +
		"title: \"クラウディア流！AI探検記\"\n" +
		"emoji: \"🛸\"\n" +
		"type: \"tech\"\n" +
		"topics: [\"ai\", \"go\", \"zenn\"]\n" +
		"published: false\n" +
		"---\n\n" +
		body
}

func failureContains(failures []string, want string) bool {
	for _, failure := range failures {
		if strings.Contains(failure, want) {
			return true
		}
	}
	return false
}
