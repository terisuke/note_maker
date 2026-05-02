package draft

import (
	"context"
	"strings"
	"testing"
	"time"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestGenerateBuildsPromptFromGuideAndBriefOnly(t *testing.T) {
	generator := &fakeGenerator{draft: matchingDraft()}
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	req := GenerateRequest{
		StyleGuide: styleGuide,
		Brief: ArticleBrief{
			StyleProfileID:        profile.ID,
			Theme:                 "ローカルLLMで記事を書く",
			OpeningEpisode:        "生成が遅くて設計を見直した経験",
			Reader:                "個人開発者",
			ExpectedReaderAction:  "小さく検証する",
			MustInclude:           "プロンプトを短くする。評価を返す。",
			PersonalContext:       "音楽家からエンジニアになった経験を入れる。",
			Exclusions:            "Note記事本文の再取得",
			TargetLengthStructure: "1200字、導入・本論・結論",
		},
		AuthorProfile: profile,
	}
	service := NewService(generator)

	result, err := service.Generate(context.Background(), req)
	if err != nil {
		t.Fatalf("generate draft: %v", err)
	}
	expectedDraft, err := articledomain.NewDraft(matchingDraft())
	if err != nil {
		t.Fatalf("new expected draft: %v", err)
	}
	if result.Draft.Markdown() != expectedDraft.Markdown() {
		t.Fatalf("unexpected draft: %q", result.Draft.Markdown())
	}
	for _, want := range []string{
		styleGuide.ParagraphRhythm,
		"ローカルLLMで記事を書く",
		"音楽家からエンジニアになった経験",
		"Note記事本文の再取得",
		"参考記事本文は与えられていません",
	} {
		if !strings.Contains(generator.prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, generator.prompt)
		}
	}
	if strings.Contains(generator.prompt, "参考本文") {
		t.Fatalf("prompt unexpectedly included raw article content:\n%s", generator.prompt)
	}
}

func TestGenerateReturnsFailedEvaluationWithoutError(t *testing.T) {
	service := NewService(&fakeGenerator{draft: "# Draft\n\nこれは短い説明です。"})
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())

	result, err := service.Generate(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: profile.ID, Theme: "短い失敗例"},
		AuthorProfile: profile,
	})
	if err != nil {
		t.Fatalf("generate draft should return low-score evaluation, not error: %v", err)
	}
	if result.Evaluation.Passed {
		t.Fatalf("expected failed evaluation: %#v", result.Evaluation)
	}
	if len(result.Evaluation.Failures) == 0 {
		t.Fatalf("expected evaluation failures: %#v", result.Evaluation)
	}
}

func TestGenerateUsesPersonaAndOutputFormat(t *testing.T) {
	zennDraft := "---\ntitle: \"Goで検証する\"\nemoji: \"🧪\"\ntype: \"tech\"\ntopics: [\"go\", \"test\"]\npublished: false\n---\n\n## 実装\n\n```go\nfmt.Println(\"ok\")\n```"
	generator := &fakeGenerator{draft: zennDraft}
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	persona, _ := personadomain.DefaultRegistry().Get(personadomain.IDCloudia)
	format, _ := outputformat.DefaultRegistry().Get(outputformat.IDZennArticle)

	result, err := NewService(generator).Generate(context.Background(), GenerateRequest{
		StyleGuide: styleGuide,
		Brief: ArticleBrief{
			StyleProfileID: profile.ID,
			PersonaID:      persona.ID,
			OutputFormatID: format.ID,
			Theme:          "Goで検証する",
		},
		AuthorProfile: profile,
		Persona:       persona,
		OutputFormat:  format,
	})
	if err != nil {
		t.Fatalf("generate zenn draft: %v", err)
	}
	if result.Draft.Markdown() != zennDraft {
		t.Fatalf("unexpected zenn draft:\n%s", result.Draft.Markdown())
	}
	for _, want := range []string{"宇宙野クラウディア", "Zenn記事", "title, emoji, type, topics, published", "## 媒体別Markdownガイド", ":::message", "@[card](https://example.com)"} {
		if !strings.Contains(generator.prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, generator.prompt)
		}
	}
}

func TestPromptIncludesFormatGuideForEveryRegisteredFormat(t *testing.T) {
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	persona, _ := personadomain.DefaultRegistry().Get(personadomain.IDTerisuke)

	tests := []struct {
		formatID string
		want     string
	}{
		{outputformat.IDNoteArticle, "Use this guide only for `note_article` output."},
		{outputformat.IDMarkdownBlog, "corsweb2024/src/content/blog/ja/{slug}.md"},
		{outputformat.IDZennArticle, "Use this guide only for `zenn_article` output."},
		{outputformat.IDQiitaArticle, "Use this guide only for `qiita_article` output."},
		{outputformat.IDHomepageSection, "Return HTML only. Do not output Markdown."},
	}

	for _, tt := range tests {
		t.Run(tt.formatID, func(t *testing.T) {
			format, ok := outputformat.DefaultRegistry().Get(tt.formatID)
			if !ok {
				t.Fatalf("missing format %s", tt.formatID)
			}
			prompt := BuildPromptForMode(styleGuide, ArticleBrief{
				StyleProfileID: profile.ID,
				PersonaID:      persona.ID,
				OutputFormatID: tt.formatID,
				Theme:          "形式別ガイド",
			}, persona, format)
			for _, want := range []string{"## 媒体別Markdownガイド", tt.want} {
				if !strings.Contains(prompt, want) {
					t.Fatalf("prompt does not contain %q:\n%s", want, prompt)
				}
			}
		})
	}
}

func TestGenerateUsesCorBlogOutputRules(t *testing.T) {
	companyBlogDraft := "---\n" +
		"title: \"AI開発の知見\"\n" +
		"description: \"AI駆動開発で得た実装判断と検証結果を共有する\"\n" +
		"pubDate: 2026-05-02\n" +
		"author: \"Terisuke\"\n" +
		"category: \"engineering\"\n" +
		"tags: [\"AI\", \"開発\", \"検証\"]\n" +
		"lang: \"ja\"\n" +
		"featured: false\n" +
		"isDraft: true\n" +
		"---\n\n" +
		"# AI開発の知見\n\n" +
		"会社の実装判断を共有する記事だ。\n\n" +
		"```go\nfmt.Println(\"ok\")\n```"
	generator := &fakeGenerator{draft: companyBlogDraft}
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	persona, _ := personadomain.DefaultRegistry().Get(personadomain.IDTerisuke)
	format, _ := outputformat.DefaultRegistry().Get(outputformat.IDMarkdownBlog)

	result, err := NewService(generator).Generate(context.Background(), GenerateRequest{
		StyleGuide: styleGuide,
		Brief: ArticleBrief{
			StyleProfileID: profile.ID,
			PersonaID:      persona.ID,
			OutputFormatID: format.ID,
			Theme:          "AI開発の知見",
		},
		AuthorProfile: profile,
		Persona:       persona,
		OutputFormat:  format,
	})
	if err != nil {
		t.Fatalf("generate company blog draft: %v", err)
	}
	if result.Draft.Markdown() != companyBlogDraft {
		t.Fatalf("unexpected company blog draft:\n%s", result.Draft.Markdown())
	}
	for _, want := range []string{"corsweb2024", "category は ai / engineering / founder / lab", "langは必ず ja", "社員へのビジョン共有"} {
		if !strings.Contains(generator.prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, generator.prompt)
		}
	}
}

func TestGenerateRejectsUnusableMarkdown(t *testing.T) {
	service := NewService(&fakeGenerator{draft: "承知しました。記事を書きます。"})
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())

	_, err := service.Generate(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: profile.ID, Theme: "ローカルLLM"},
		AuthorProfile: profile,
	})
	if err == nil {
		t.Fatal("expected unusable draft error")
	}
}

func TestEvaluateStylePassesStrictThresholdsAndOverride(t *testing.T) {
	text := strings.ReplaceAll(matchingDraft(), "僕", "私")
	articleDraft, err := articledomain.NewDraft(text)
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}
	profile, _ := profileAndGuideFromDraft(t, text)
	profile.PreferredFirstPerson = "僕"

	evaluation := EvaluateStyle(
		profile,
		ArticleBrief{StyleProfileID: profile.ID, Theme: "override", ToneStance: "一人称は私で書く"},
		articleDraft,
	)

	if !evaluation.Passed {
		t.Fatalf("expected evaluation to pass: %#v", evaluation)
	}
	if evaluation.RequiredFirstPerson != "私" {
		t.Fatalf("expected override to be required, got %q", evaluation.RequiredFirstPerson)
	}
}

type fakeGenerator struct {
	prompt string
	draft  string
	err    error
}

func (g *fakeGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	g.prompt = prompt
	return g.draft, g.err
}

func profileAndGuideFromDraft(t *testing.T, text string) (AuthorStyleProfile, WritingStyleGuide) {
	t.Helper()

	fetchedAt := time.Unix(1700000000, 0).UTC()
	profile, err := authordomain.BuildAuthorStyleProfile(
		authordomain.AuthorSource{Username: "test_author", FetchedAt: fetchedAt},
		[]articledomain.Article{{
			URL:     "https://example.com/articles/ref",
			Title:   "Reference",
			Content: text,
		}},
	)
	if err != nil {
		t.Fatalf("build author profile: %v", err)
	}
	guide, err := authordomain.BuildWritingStyleGuide(profile)
	if err != nil {
		t.Fatalf("build writing style guide: %v", err)
	}
	return profile, guide
}

func matchingDraft() string {
	intro := "# AIと違和感を小さく言語化する\n\n"
	body := strings.Repeat("僕はAIと起業の挑戦について、「違和感」を言語化しながら自分の判断を見直しました。\n\n", 18)
	middle := "## 体験から始める\n\n"
	middle += strings.Repeat("僕は音楽とエンジニアの経験を行き来し、読者が小さくアウトプットできる形にします。\n\n", 12)
	end := "## 次の一歩\n\n"
	end += strings.Repeat("僕は抽象論で終わらせず、今日試せる判断基準としてAIとの向き合い方を置き直します。\n\n", 8)
	return intro + body + middle + end
}
