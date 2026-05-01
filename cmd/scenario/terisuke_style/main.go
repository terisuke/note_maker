package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	articleapp "github.com/teradakousuke/note_maker/internal/application/article"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
	notenote "github.com/teradakousuke/note_maker/internal/infrastructure/note"
)

const (
	defaultUsername = "cor_instrument"
	defaultBaseURL  = "http://127.0.0.1:11434/v1"
	defaultOutput   = "tmp/terisuke_style_scenario"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	username := envOrDefault("TERISUKE_NOTE_USERNAME", defaultUsername)
	baseURL := envOrDefault("LLAMACPP_BASE_URL", defaultBaseURL)
	model := envOrDefault("LLAMACPP_MODEL", "gemma4:31b")
	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutput)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	httpClient := &http.Client{Timeout: 45 * time.Second}
	noteClient := notenote.NewAPIClient(httpClient)
	references, err := noteClient.FetchUserArticles(ctx, username, 6)
	if err != nil {
		fatalf("fetch Terisuke articles through note API: %v", err)
	}
	referenceProfile := articledomain.AnalyzeStyleCorpus(references)

	llmClient, err := llamacpp.NewClient(baseURL, model, &http.Client{Timeout: 10 * time.Minute})
	if err != nil {
		fatalf("create local LLM client: %v", err)
	}
	models, err := llmClient.ListModels(ctx)
	if err != nil {
		fatalf("list local LLM models: %v", err)
	}
	if !slices.Contains(models, model) {
		fatalf("model %q is not available from %s; available=%v", model, baseURL, models)
	}

	service := articleapp.NewService(staticFetcher{articles: references}, llmClient, nil)
	request := articledomain.GenerationRequest{
		Username:              username,
		Keywords:              []string{"ローカルLLM", "文章資産", "Note", "AI", "自分の思想"},
		Theme:                 "ローカルLLMで自分の記事の思想と文体を再現できるのかを検証した話",
		TargetAudience:        "AIを使って自分の発信を資産化したい個人開発者・発信者",
		StyleChoice:           "参考記事に近い一人称のエッセイ調",
		ToneChoice:            "内省的だが、技術検証の事実も具体的に書く",
		WordCount:             1800,
		ReferenceArticleLimit: 6,
		ArticlePurpose:        "Noteにそのまま貼り付けて公開前編集に進められる、Terisuke本人らしい検証記事の下書きを作る",
		DesiredContent: strings.Join([]string{
			buildStyleGuide(referenceProfile),
			"導入は個人的な違和感から始める。",
			"単なるツール紹介ではなく、自分の文章をAIに渡すことへの期待と怖さを書く。",
			"Note APIで過去記事を取得し、ローカルLLMに参照させ、生成結果を比較したという検証の流れを入れる。",
			"読者には、自分の発信を外部プラットフォームだけに預けず、自分の思想として再利用可能にする視点を渡す。",
			"参考記事の語り口に近づけるため、体験、内省、技術、読者への橋渡しを混ぜる。",
		}, "\n"),
		IntroductionPoints: "『自分の記事をAIに読ませたら、それは本当に自分の文章になるのか』という問いから始める。",
		MainPoints:         "Note APIで取得した過去記事、ローカルLLMのgemma4:31b、生成結果の比較、まだ足りない点。",
		ConclusionMessage:  "AIに任せるのではなく、自分の思想を取り戻すためにローカルで検証できる状態を持つ。",
		Exclusions:         "過度な宣伝、根拠のない性能断言、Gemini依存、コードフェンス、生成プロンプトの説明",
	}

	draft, err := service.GenerateArticle(ctx, request)
	if err != nil {
		fatalf("generate scenario draft: %v", err)
	}

	candidateProfile := articledomain.AnalyzeStyle(draft)
	comparison := articledomain.CompareStyle(referenceProfile, candidateProfile)

	draftPath := filepath.Join(outputDir, "generated_draft.md")
	reportPath := filepath.Join(outputDir, "report.md")
	jsonPath := filepath.Join(outputDir, "comparison.json")

	if err := os.WriteFile(draftPath, []byte(draft+"\n"), 0o644); err != nil {
		fatalf("write draft: %v", err)
	}
	if err := writeJSON(jsonPath, comparison); err != nil {
		fatalf("write comparison json: %v", err)
	}
	if err := os.WriteFile(reportPath, []byte(buildReport(username, baseURL, model, references, draftPath, jsonPath, comparison)), 0o644); err != nil {
		fatalf("write report: %v", err)
	}

	fmt.Printf("scenario completed\n")
	fmt.Printf("note_username=%s\n", username)
	fmt.Printf("articles=%d\n", len(references))
	fmt.Printf("llm_base_url=%s\n", baseURL)
	fmt.Printf("llm_model=%s\n", model)
	fmt.Printf("style_score=%.1f\n", comparison.Score)
	fmt.Printf("report=%s\n", reportPath)
	fmt.Printf("draft=%s\n", draftPath)
}

type staticFetcher struct {
	articles []articledomain.Article
}

func (f staticFetcher) FetchArticle(ctx context.Context, articleURL string) (*articledomain.Article, error) {
	for _, article := range f.articles {
		if article.URL == articleURL {
			copy := article
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("article not found: %s", articleURL)
}

func (f staticFetcher) FetchUserLatestArticles(ctx context.Context, username string, limit int) ([]articledomain.Article, error) {
	if limit <= 0 || limit > len(f.articles) {
		limit = len(f.articles)
	}
	return f.articles[:limit], nil
}

func buildReport(username, baseURL, model string, references []articledomain.Article, draftPath, jsonPath string, comparison articledomain.StyleComparison) string {
	var builder strings.Builder
	builder.WriteString("# Terisuke Style Scenario Test\n\n")
	builder.WriteString(fmt.Sprintf("- Note username: `%s`\n", username))
	builder.WriteString("- Note acquisition: `https://note.com/api/v2/creators/{username}/contents` + `https://note.com/api/v3/notes/{key}`\n")
	builder.WriteString(fmt.Sprintf("- Local LLM endpoint: `%s`\n", baseURL))
	builder.WriteString(fmt.Sprintf("- Local LLM model: `%s`\n", model))
	builder.WriteString(fmt.Sprintf("- Reference articles: `%d`\n", len(references)))
	builder.WriteString(fmt.Sprintf("- Generated draft: `%s`\n", draftPath))
	builder.WriteString(fmt.Sprintf("- Raw comparison JSON: `%s`\n\n", jsonPath))

	builder.WriteString("## Reference Articles\n\n")
	for i, article := range references {
		builder.WriteString(fmt.Sprintf("%d. %s", i+1, article.Title))
		if article.URL != "" {
			builder.WriteString(fmt.Sprintf(" - %s", article.URL))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\n## Verdict\n\n")
	verdict := "PASS"
	if comparison.Score < 70 || len(comparison.Risks) >= 3 {
		verdict = "NEEDS_WORK"
	}
	builder.WriteString(fmt.Sprintf("- Verdict: `%s`\n", verdict))
	builder.WriteString(fmt.Sprintf("- Style score: `%.1f / 100`\n", comparison.Score))
	builder.WriteString(fmt.Sprintf("- Matched signals: `%s`\n", strings.Join(comparison.MatchedSignals, ", ")))
	builder.WriteString(fmt.Sprintf("- Risks: `%s`\n\n", strings.Join(comparison.Risks, ", ")))

	builder.WriteString("## Metric Scores\n\n")
	for _, name := range sortedMetricNames(comparison.MetricScores) {
		builder.WriteString(fmt.Sprintf("- `%s`: `%d`\n", name, comparison.MetricScores[name]))
	}

	builder.WriteString("\n## Profiles\n\n")
	builder.WriteString(fmt.Sprintf("- Reference: chars=%d paragraphs=%d avg_sentence=%.1f avg_paragraph=%.1f headings=%d quotes=%d first_person=%d\n",
		comparison.Reference.CharCount,
		comparison.Reference.ParagraphCount,
		comparison.Reference.AverageSentenceRunes,
		comparison.Reference.AverageParagraphRunes,
		comparison.Reference.HeadingCount,
		comparison.Reference.QuoteCount,
		comparison.Reference.FirstPersonCount,
	))
	builder.WriteString(fmt.Sprintf("- Candidate: chars=%d paragraphs=%d avg_sentence=%.1f avg_paragraph=%.1f headings=%d quotes=%d first_person=%d\n",
		comparison.Candidate.CharCount,
		comparison.Candidate.ParagraphCount,
		comparison.Candidate.AverageSentenceRunes,
		comparison.Candidate.AverageParagraphRunes,
		comparison.Candidate.HeadingCount,
		comparison.Candidate.QuoteCount,
		comparison.Candidate.FirstPersonCount,
	))
	return builder.String()
}

func buildStyleGuide(profile articledomain.StyleProfile) string {
	firstPerson := "僕"
	if profile.KeywordCounts["私"] > profile.KeywordCounts["僕"] {
		firstPerson = "私"
	}
	keywords := topKeywords(profile.KeywordCounts, 8)
	return fmt.Sprintf(
		"参考記事から抽出した文体ガイド: 一人称は `%s` を基本にする。頻出テーマは `%s`。段落は平均 %.1f 文字、文は平均 %.1f 文字に近づける。引用や内心の声を使うが、一人称を連呼しすぎない。音楽家からエンジニア、起業、LT、AI活用といった本人の文脈が自然に接続できる場合は、記事の体験軸として使う。",
		firstPerson,
		strings.Join(keywords, ", "),
		profile.AverageParagraphRunes,
		profile.AverageSentenceRunes,
	)
}

func topKeywords(counts map[string]int, limit int) []string {
	type pair struct {
		key   string
		count int
	}
	pairs := make([]pair, 0, len(counts))
	for key, count := range counts {
		if count > 0 {
			pairs = append(pairs, pair{key: key, count: count})
		}
	}
	slices.SortFunc(pairs, func(a, b pair) int {
		if a.count == b.count {
			return strings.Compare(a.key, b.key)
		}
		return b.count - a.count
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	keywords := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		keywords = append(keywords, pair.key)
	}
	return keywords
}

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func sortedMetricNames(metrics map[string]int) []string {
	names := make([]string, 0, len(metrics))
	for name := range metrics {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}
