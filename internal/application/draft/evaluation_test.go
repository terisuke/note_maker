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

func TestEvaluateStyleScoresCloudiaZennTechnicalSignals(t *testing.T) {
	reference := cloudiaZennTechnicalSections(9)
	profile := AuthorStyleProfile{
		ID:                   "profile-cloudia-zenn",
		Metrics:              articledomain.AnalyzeStyle(reference),
		PreferredFirstPerson: "クラウディア",
	}
	articleDraft, err := articledomain.NewDraftForFormat(
		zennArticleWithBody(cloudiaZennTechnicalSections(14)),
		outputformat.IDZennArticle,
	)
	if err != nil {
		t.Fatalf("new zenn draft: %v", err)
	}

	evaluation := EvaluateStyle(profile, ArticleBrief{
		PersonaID:      personadomain.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
	}, articleDraft)

	if !evaluation.Passed {
		t.Fatalf("expected Cloudia/Zenn technical draft to pass style evaluation: %#v", evaluation)
	}
	if evaluation.Comparison.MetricScores["keyword_overlap"] < StrictThresholds.KeywordOverlap {
		t.Fatalf("keyword overlap score = %d, want technical persona signals to count", evaluation.Comparison.MetricScores["keyword_overlap"])
	}
	if evaluation.Comparison.MetricScores["heading_structure"] < 75 {
		t.Fatalf("heading score = %d, want long Zenn outline to stay reviewable", evaluation.Comparison.MetricScores["heading_structure"])
	}
}

func TestEvaluateStyleIgnoresMarkdownBlogFrontmatterAndCodeForStyleMetrics(t *testing.T) {
	body := corBlogBody()
	profile := AuthorStyleProfile{
		ID:                   "profile-cor-blog",
		Metrics:              articledomain.AnalyzeStyle(body),
		PreferredFirstPerson: "私",
	}
	raw := markdownBlogArticleWithBody(body) + "\n\n" +
		"```go\n" +
		"fmt.Println(\"検証用のコード例\")\n" +
		"```\n"
	articleDraft, err := articledomain.NewDraftForFormat(raw, outputformat.IDMarkdownBlog)
	if err != nil {
		t.Fatalf("new markdown blog draft: %v", err)
	}

	evaluation := EvaluateStyle(profile, ArticleBrief{
		PersonaID:      personadomain.IDTerisuke,
		OutputFormatID: outputformat.IDMarkdownBlog,
	}, articleDraft)

	if !evaluation.Passed {
		t.Fatalf("expected evaluation to pass with metadata and code ignored: %#v", evaluation)
	}
	expected := articledomain.AnalyzeStyle(body)
	if evaluation.Comparison.Candidate.ParagraphCount != expected.ParagraphCount {
		t.Fatalf("candidate paragraphs = %d, want body-only %d", evaluation.Comparison.Candidate.ParagraphCount, expected.ParagraphCount)
	}
	if evaluation.Comparison.Candidate.KeywordCounts["検証"] != expected.KeywordCounts["検証"] {
		t.Fatalf("candidate verification count = %d, want body-only %d", evaluation.Comparison.Candidate.KeywordCounts["検証"], expected.KeywordCounts["検証"])
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

func TestEvaluateStyleDoesNotUseExcludedPersonaAsFirstPersonOverride(t *testing.T) {
	body := corBlogBody()
	profile := AuthorStyleProfile{
		ID:                   "profile-terisuke",
		Metrics:              articledomain.AnalyzeStyle(body),
		PreferredFirstPerson: "僕",
	}
	draft, err := articledomain.NewDraft(body)
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}

	evaluation := EvaluateStyle(profile, ArticleBrief{
		PersonaID:      personadomain.IDTerisuke,
		OutputFormatID: outputformat.IDNoteArticle,
		MustInclude:    "一人称密度を参照文体に近づける",
		Exclusions:     "クラウディア口調",
	}, draft)

	if evaluation.RequiredFirstPerson != "僕" {
		t.Fatalf("required first person = %q, want 僕", evaluation.RequiredFirstPerson)
	}
	if failureContains(evaluation.Failures, "クラウディア") {
		t.Fatalf("excluded persona leaked into failures: %#v", evaluation.Failures)
	}
}

func corBlogBody() string {
	intro := "# AI開発の検証知見\n\n"
	intro += strings.Repeat("私はEvo X2の推論経路を検証し、会社ブログとして再現できる判断材料を残す。検証では速度、品質、運用負荷を分けて記録する。\n\n", 6)
	steps := "## 実装判断\n\n"
	steps += strings.Repeat("私は実装の前提をADRとissueに結び、社員が同じ文脈で判断できるようにする。検証結果は成功だけでなく失敗条件も残す。\n\n", 6)
	wrap := "## 次の行動\n\n"
	wrap += strings.Repeat("私は次のフェーズでシナリオを変え、媒体ごとの文体差が保てるかを確認する。検証の粒度をそろえることで改善点を見つける。\n\n", 4)
	return intro + steps + wrap
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

func cloudiaZennTechnicalSections(count int) string {
	var builder strings.Builder
	for i := 0; i < count; i++ {
		builder.WriteString("## 手順\n\n")
		builder.WriteString("クラウディアはGoのCLIでZenn向けMarkdownとfrontmatter、topics、コード、実装手順を検証するばい。")
		builder.WriteString("Qiitaと混ぜず、媒体別プロンプト、再現、初心者のつまずきを自分の手元で整理するとよ。\n\n")
	}
	return builder.String()
}

func markdownBlogArticleWithBody(body string) string {
	return "---\n" +
		"title: \"AI開発の検証知見\"\n" +
		"description: \"Evo X2を使った記事生成パイプラインの判断材料を共有する\"\n" +
		"pubDate: 2026-05-03\n" +
		"author: \"Terisuke\"\n" +
		"category: \"engineering\"\n" +
		"tags: [\"AI\", \"検証\", \"開発\"]\n" +
		"lang: \"ja\"\n" +
		"featured: false\n" +
		"isDraft: true\n" +
		"---\n\n" +
		body
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
