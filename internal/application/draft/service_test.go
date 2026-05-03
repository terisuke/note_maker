package draft

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
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
			CustomAnswers: []BriefAnswer{
				{QuestionID: briefdomain.QuestionIDReaderProblem, Content: "媒体ごとの書き分けが難しい"},
				{QuestionID: briefdomain.QuestionIDTitleKeywords, Content: "下書き、Evo X2、検証"},
			},
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
		"strict style calibration",
		"一人称密度",
		"読者の困りごと: 媒体ごとの書き分けが難しい",
		"タイトル候補・見出し語: 下書き、Evo X2、検証",
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

func TestGenerateRunsControlledRevisionWhenStrictStyleFails(t *testing.T) {
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	generator := &sequenceGenerator{drafts: []string{
		"# Draft\n\nこれは短い説明です。",
		matchingDraft(),
	}}

	result, err := NewService(generator).Generate(context.Background(), GenerateRequest{
		StyleGuide: styleGuide,
		Brief: ArticleBrief{
			StyleProfileID:        profile.ID,
			Theme:                 "改善する",
			TargetLengthStructure: "3000字、導入・本論・結論",
		},
		AuthorProfile: profile,
	})
	if err != nil {
		t.Fatalf("generate with revision: %v", err)
	}
	if generator.calls != 2 {
		t.Fatalf("calls = %d, want 2", generator.calls)
	}
	if !strings.Contains(generator.prompts[1], "strict style evaluation failures") {
		t.Fatalf("revision prompt missing failures:\n%s", generator.prompts[1])
	}
	expectedDraft, err := articledomain.NewDraft(matchingDraft())
	if err != nil {
		t.Fatalf("new expected draft: %v", err)
	}
	if result.Draft.Markdown() != expectedDraft.Markdown() {
		t.Fatalf("expected revised draft to be returned")
	}
}

func TestGenerateStreamEmitsStatusAndChunks(t *testing.T) {
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	generator := &streamingFakeGenerator{chunks: []string{
		"# AIと違和感を小さく言語化する\n\n",
		strings.Repeat("僕はAIと起業の挑戦について、「違和感」を言語化しながら自分の判断を見直しました。\n\n", 18),
		"## 体験から始める\n\n" + strings.Repeat("僕は音楽とエンジニアの経験を行き来し、読者が小さくアウトプットできる形にします。\n\n", 12),
		"## 次の一歩\n\n" + strings.Repeat("僕は抽象論で終わらせず、今日試せる判断基準としてAIとの向き合い方を置き直します。\n\n", 8),
	}}
	var statuses []string
	var streamed strings.Builder

	result, err := NewService(generator).GenerateStream(context.Background(), GenerateRequest{
		StyleGuide: styleGuide,
		Brief: ArticleBrief{
			StyleProfileID:        profile.ID,
			Theme:                 "ストリーミングする",
			TargetLengthStructure: "3000字、導入・本論・結論",
		},
		AuthorProfile: profile,
	}, StreamEvents{
		OnStatus: func(status string) error {
			statuses = append(statuses, status)
			return nil
		},
		OnChunk: func(chunk string) error {
			streamed.WriteString(chunk)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("generate stream: %v", err)
	}
	if !generator.streamed {
		t.Fatal("expected streaming generator to be used")
	}
	if strings.TrimSpace(streamed.String()) != result.Draft.Markdown() {
		t.Fatalf("streamed chunks differ from final draft")
	}
	if strings.Join(statuses, ",") != "draft_generation_started,draft_validation_started" {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}

func TestGenerateRunsLightweightVerification(t *testing.T) {
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	generator := &fakeGenerator{draft: matchingDraft()}
	verifierModel := &fakeGenerator{draft: "PASS\nSummary: ブリーフと文体に沿っています"}

	result, err := NewServiceWithVerifier(generator, NewLightweightVerifier(verifierModel)).Generate(context.Background(), GenerateRequest{
		StyleGuide: styleGuide,
		Brief: ArticleBrief{
			StyleProfileID: profile.ID,
			Theme:          "最終検証する",
			Reader:         "AIで記事を書く人",
			MustInclude:    "軽量モデルで検証する",
		},
		AuthorProfile: profile,
	})
	if err != nil {
		t.Fatalf("generate with verification: %v", err)
	}
	if !result.Verification.Performed || !result.Verification.Passed {
		t.Fatalf("unexpected verification: %#v", result.Verification)
	}
	for _, want := range []string{"最終検証者", "最終検証する", "軽量モデルで検証する", matchingDraft()[:30]} {
		if !strings.Contains(verifierModel.prompt, want) {
			t.Fatalf("verification prompt missing %q:\n%s", want, verifierModel.prompt)
		}
	}
}

func TestGenerateReturnsPromptlyWhenFinalVerificationBlocks(t *testing.T) {
	t.Setenv("DRAFT_FINAL_VERIFICATION_TIMEOUT_SECONDS", "1")
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	verifier := &blockingVerifier{started: make(chan struct{})}

	start := time.Now()
	result, err := NewServiceWithVerifier(&fakeGenerator{draft: matchingDraft()}, verifier).Generate(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: profile.ID, Theme: "検証タイムアウト"},
		AuthorProfile: profile,
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("generate with blocking verification: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("generate took %s, want bounded final verification", elapsed)
	}
	select {
	case <-verifier.started:
	default:
		t.Fatal("expected final verifier to be called")
	}
	if !result.Verification.Performed || result.Verification.Passed {
		t.Fatalf("expected performed failed timeout verification: %#v", result.Verification)
	}
	for _, want := range []string{"timed out", "1s"} {
		if !strings.Contains(result.Verification.Summary, want) {
			t.Fatalf("timeout summary missing %q: %#v", want, result.Verification)
		}
	}
	if len(result.Verification.Failures) == 0 || !strings.Contains(result.Verification.Failures[0], "timed out") {
		t.Fatalf("timeout failure not recorded: %#v", result.Verification)
	}
}

func TestGenerateStreamReturnsPromptlyWhenFinalVerificationBlocks(t *testing.T) {
	t.Setenv("DRAFT_FINAL_VERIFICATION_TIMEOUT_SECONDS", "1")
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	generator := &streamingFakeGenerator{chunks: []string{matchingDraft()}}
	verifier := &blockingVerifier{started: make(chan struct{})}
	var statuses []string
	var streamed strings.Builder

	start := time.Now()
	result, err := NewServiceWithVerifier(generator, verifier).GenerateStream(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: profile.ID, Theme: "ストリーミング検証タイムアウト"},
		AuthorProfile: profile,
	}, StreamEvents{
		OnStatus: func(status string) error {
			statuses = append(statuses, status)
			return nil
		},
		OnChunk: func(chunk string) error {
			streamed.WriteString(chunk)
			return nil
		},
	})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("generate stream with blocking verification: %v", err)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("generate stream took %s, want bounded final verification", elapsed)
	}
	if !generator.streamed {
		t.Fatal("expected streaming generator to be used")
	}
	select {
	case <-verifier.started:
	default:
		t.Fatal("expected final verifier to be called")
	}
	if strings.TrimSpace(streamed.String()) != result.Draft.Markdown() {
		t.Fatalf("streamed chunks differ from final draft")
	}
	if !result.Verification.Performed || result.Verification.Passed || !strings.Contains(result.Verification.Summary, "timed out after 1s") {
		t.Fatalf("expected performed failed timeout verification: %#v", result.Verification)
	}
	if strings.Join(statuses, ",") != "draft_generation_started,draft_validation_started,draft_lightweight_verification_started" {
		t.Fatalf("unexpected statuses: %#v", statuses)
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

func TestGenerateRunsOneFormatRepairRetry(t *testing.T) {
	invalidZenn := "---\n" +
		"title: \"Goで検証する\"\n" +
		"emoji: \"🧪\"\n" +
		"type: \"tech\"\n" +
		"topics: [\"go\", \"test\"]\n" +
		"published: false\n" +
		"---\n\n" +
		"## 実装\n\n" +
		":::note info\nQiitaの補足です\n:::\n"
	repairedZenn := strings.ReplaceAll(invalidZenn, ":::note info", ":::message")
	generator := &sequenceGenerator{drafts: []string{invalidZenn, repairedZenn}}
	profile, styleGuide := profileAndGuideFromDraft(t, styleEvaluationMarkdown(repairedZenn, outputformat.IDZennArticle))
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
		t.Fatalf("generate with format repair: %v", err)
	}
	if generator.calls != 2 {
		t.Fatalf("calls = %d, want 2", generator.calls)
	}
	if result.Draft.Markdown() != strings.TrimSpace(repairedZenn) {
		t.Fatalf("unexpected repaired draft:\n%s", result.Draft.Markdown())
	}
	if len(result.Attempts) != 2 {
		t.Fatalf("attempts = %#v, want 2 attempts", result.Attempts)
	}
	if result.Attempts[0].Kind != "initial" || result.Attempts[0].RawOutput != invalidZenn || !strings.Contains(result.Attempts[0].ValidationError, "Qiita :::note") {
		t.Fatalf("initial attempt did not preserve validation failure: %#v", result.Attempts[0])
	}
	if result.Attempts[1].Kind != "format_repair" || result.Attempts[1].RawOutput != repairedZenn || result.Attempts[1].ValidationError != "" {
		t.Fatalf("repair attempt not preserved correctly: %#v", result.Attempts[1])
	}
	for _, want := range []string{
		invalidZenn,
		"zenn article must use :::message, not Qiita :::note",
		"Use this guide only for `zenn_article` output.",
		"修正版の記事本文だけ",
	} {
		if !strings.Contains(generator.prompts[1], want) {
			t.Fatalf("repair prompt missing %q:\n%s", want, generator.prompts[1])
		}
	}
}

func TestGenerateRepairsQiitaDraftMissingFrontmatter(t *testing.T) {
	invalidQiita := matchingDraft()
	repairedQiita := "---\n" +
		"title: \"AIと違和感を小さく言語化する\"\n" +
		"tags: [\"AI\", \"Go\"]\n" +
		"---\n\n" +
		invalidQiita
	generator := &sequenceGenerator{drafts: []string{invalidQiita, repairedQiita}}
	profile, styleGuide := profileAndGuideFromDraft(t, styleEvaluationMarkdown(repairedQiita, outputformat.IDQiitaArticle))
	persona, _ := personadomain.DefaultRegistry().Get(personadomain.IDTerisuke)
	format, _ := outputformat.DefaultRegistry().Get(outputformat.IDQiitaArticle)

	result, err := NewService(generator).Generate(context.Background(), GenerateRequest{
		StyleGuide: styleGuide,
		Brief: ArticleBrief{
			StyleProfileID: profile.ID,
			PersonaID:      persona.ID,
			OutputFormatID: format.ID,
			Theme:          "AIと違和感を小さく言語化する",
		},
		AuthorProfile: profile,
		Persona:       persona,
		OutputFormat:  format,
	})
	if err != nil {
		t.Fatalf("generate with qiita frontmatter repair: %v", err)
	}
	if generator.calls != 2 {
		t.Fatalf("calls = %d, want 2", generator.calls)
	}
	if result.Draft.Markdown() != strings.TrimSpace(repairedQiita) {
		t.Fatalf("unexpected repaired draft:\n%s", result.Draft.Markdown())
	}
	if len(result.Attempts) != 2 {
		t.Fatalf("attempts = %#v, want 2 attempts", result.Attempts)
	}
	if result.Attempts[0].Kind != "initial" || result.Attempts[0].RawOutput != invalidQiita || !strings.Contains(result.Attempts[0].ValidationError, "qiita article requires YAML frontmatter") {
		t.Fatalf("initial attempt did not preserve missing frontmatter failure: %#v", result.Attempts[0])
	}
	if result.Attempts[1].Kind != "format_repair" || result.Attempts[1].RawOutput != repairedQiita || result.Attempts[1].ValidationError != "" {
		t.Fatalf("repair attempt not preserved correctly: %#v", result.Attempts[1])
	}
	for _, want := range []string{
		invalidQiita,
		"qiita article requires YAML frontmatter",
		"Use this guide only for `qiita_article` output.",
		"修正版の記事本文だけ",
	} {
		if !strings.Contains(generator.prompts[1], want) {
			t.Fatalf("repair prompt missing %q:\n%s", want, generator.prompts[1])
		}
	}
}

func TestRecoverableFormatValidationErrorsAreBoundedToKnownCases(t *testing.T) {
	tests := []struct {
		name     string
		formatID string
		err      error
		want     bool
	}{
		{
			name:     "preamble",
			formatID: outputformat.IDNoteArticle,
			err:      errors.New("draft appears to contain preamble before the article"),
			want:     true,
		},
		{
			name:     "markdown blog missing frontmatter",
			formatID: outputformat.IDMarkdownBlog,
			err:      errors.New("company blog article requires YAML frontmatter"),
			want:     true,
		},
		{
			name:     "markdown blog missing frontmatter key",
			formatID: outputformat.IDMarkdownBlog,
			err:      errors.New("company blog frontmatter missing title"),
			want:     true,
		},
		{
			name:     "zenn missing frontmatter",
			formatID: outputformat.IDZennArticle,
			err:      errors.New("zenn article requires YAML frontmatter"),
			want:     true,
		},
		{
			name:     "zenn missing frontmatter key",
			formatID: outputformat.IDZennArticle,
			err:      errors.New("zenn frontmatter missing emoji"),
			want:     true,
		},
		{
			name:     "zenn qiita note",
			formatID: outputformat.IDZennArticle,
			err:      errors.New("zenn article must use :::message, not Qiita :::note"),
			want:     true,
		},
		{
			name:     "qiita missing frontmatter",
			formatID: outputformat.IDQiitaArticle,
			err:      errors.New("qiita article requires YAML frontmatter"),
			want:     true,
		},
		{
			name:     "qiita missing frontmatter key",
			formatID: outputformat.IDQiitaArticle,
			err:      errors.New("qiita frontmatter missing tags"),
			want:     true,
		},
		{
			name:     "qiita zenn notation",
			formatID: outputformat.IDQiitaArticle,
			err:      errors.New("qiita article must not contain Zenn-specific notation"),
			want:     true,
		},
		{
			name:     "missing title stays strict",
			formatID: outputformat.IDNoteArticle,
			err:      errors.New("note article must start with a level-1 Markdown title"),
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRecoverableFormatValidationError(tt.formatID, tt.err); got != tt.want {
				t.Fatalf("recoverable = %v, want %v", got, tt.want)
			}
		})
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

func TestBuildPromptCalibratesFirstPersonDensityFromReferenceProfile(t *testing.T) {
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	persona, _ := personadomain.DefaultRegistry().Get(personadomain.IDTerisuke)
	format, _ := outputformat.DefaultRegistry().Get(outputformat.IDNoteArticle)

	prompt := BuildPromptForModeWithProfile(styleGuide, ArticleBrief{
		StyleProfileID:        profile.ID,
		PersonaID:             persona.ID,
		OutputFormatID:        format.ID,
		Theme:                 "一人称密度を合わせる",
		TargetLengthStructure: "3000字、導入・本論・結論",
	}, profile, persona, format)

	for _, want := range []string{"strict style calibration", "参照文体の一人称密度", "一人称「僕」", "参照文体の鉤括弧密度", "参照文体の主要キーワード候補", "今回の目標長さ"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, prompt)
		}
	}
	for _, forbidden := range []string{"全文で6〜8回程度", "4〜6箇所ほど"} {
		if strings.Contains(prompt, forbidden) {
			t.Fatalf("prompt still contains fixed density guidance %q:\n%s", forbidden, prompt)
		}
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
	generator := &sequenceGenerator{drafts: []string{"承知しました。記事を書きます。", matchingDraft()}}
	service := NewService(generator)
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())

	_, err := service.Generate(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: profile.ID, Theme: "ローカルLLM"},
		AuthorProfile: profile,
	})
	if err == nil {
		t.Fatal("expected unusable draft error")
	}
	if generator.calls != 1 {
		t.Fatalf("calls = %d, want no repair retry", generator.calls)
	}
	var unusable *UnusableDraftError
	if !errors.As(err, &unusable) {
		t.Fatalf("expected UnusableDraftError, got %T", err)
	}
	if len(unusable.Attempts) != 1 || unusable.Attempts[0].RawOutput != "承知しました。記事を書きます。" {
		t.Fatalf("raw validation failure was not preserved: %#v", unusable.Attempts)
	}
}

func TestGenerateRejectsMismatchedStyleArtifacts(t *testing.T) {
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())
	styleGuide.ProfileID = "other_profile"

	_, err := NewService(&fakeGenerator{draft: matchingDraft()}).Generate(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: profile.ID, Theme: "不一致を検出する"},
		AuthorProfile: profile,
	})
	if err == nil || !strings.Contains(err.Error(), "does not match author profile id") {
		t.Fatalf("expected guide/profile mismatch error, got %v", err)
	}

	profile, styleGuide = profileAndGuideFromDraft(t, matchingDraft())
	_, err = NewService(&fakeGenerator{draft: matchingDraft()}).Generate(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: "wrong_profile", Theme: "不一致を検出する"},
		AuthorProfile: profile,
	})
	if err == nil || !strings.Contains(err.Error(), "article brief style profile id") {
		t.Fatalf("expected brief/profile mismatch error, got %v", err)
	}
}

func TestGenerateRejectsUnknownBriefOutputFormat(t *testing.T) {
	profile, styleGuide := profileAndGuideFromDraft(t, matchingDraft())

	_, err := NewService(&fakeGenerator{draft: matchingDraft()}).Generate(context.Background(), GenerateRequest{
		StyleGuide:    styleGuide,
		Brief:         ArticleBrief{StyleProfileID: profile.ID, Theme: "未知形式", OutputFormatID: "missing_format"},
		AuthorProfile: profile,
	})
	if err == nil || !strings.Contains(err.Error(), `unknown output format "missing_format"`) {
		t.Fatalf("expected unknown output format error, got %v", err)
	}
}

func TestGeneratePreservesInitialAttemptWhenRepairTransportFails(t *testing.T) {
	invalidZenn := "---\n" +
		"title: \"Goで検証する\"\n" +
		"emoji: \"🧪\"\n" +
		"type: \"tech\"\n" +
		"topics: [\"go\", \"test\"]\n" +
		"published: false\n" +
		"---\n\n" +
		"## 実装\n\n" +
		":::note info\nQiitaの補足です\n:::\n"
	profile, styleGuide := profileAndGuideFromDraft(t, strings.ReplaceAll(invalidZenn, ":::note info", ":::message"))
	persona, _ := personadomain.DefaultRegistry().Get(personadomain.IDCloudia)
	format, _ := outputformat.DefaultRegistry().Get(outputformat.IDZennArticle)
	generator := &sequenceGenerator{drafts: []string{invalidZenn}, errAtCall: 2, err: errors.New("repair endpoint unavailable")}

	_, err := NewService(generator).Generate(context.Background(), GenerateRequest{
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
	if err == nil {
		t.Fatal("expected repair transport error")
	}
	var unusable *UnusableDraftError
	if !errors.As(err, &unusable) {
		t.Fatalf("expected UnusableDraftError, got %T: %v", err, err)
	}
	if len(unusable.Attempts) != 1 || unusable.Attempts[0].RawOutput != invalidZenn {
		t.Fatalf("initial raw attempt was not preserved: %#v", unusable.Attempts)
	}
	if !strings.Contains(unusable.Error(), "repair endpoint unavailable") {
		t.Fatalf("repair error not surfaced: %v", unusable)
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

type sequenceGenerator struct {
	prompts   []string
	drafts    []string
	calls     int
	errAtCall int
	err       error
}

func (g *sequenceGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	g.prompts = append(g.prompts, prompt)
	if g.errAtCall > 0 && g.calls+1 == g.errAtCall {
		g.calls++
		return "", g.err
	}
	if g.calls >= len(g.drafts) {
		g.calls++
		return g.drafts[len(g.drafts)-1], nil
	}
	draft := g.drafts[g.calls]
	g.calls++
	return draft, nil
}

type streamingFakeGenerator struct {
	chunks   []string
	prompt   string
	streamed bool
}

func (g *streamingFakeGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	g.prompt = prompt
	return strings.Join(g.chunks, ""), nil
}

func (g *streamingFakeGenerator) GenerateStream(ctx context.Context, prompt string, onChunk func(string) error) (string, error) {
	g.prompt = prompt
	g.streamed = true
	for _, chunk := range g.chunks {
		if err := onChunk(chunk); err != nil {
			return "", err
		}
	}
	return strings.Join(g.chunks, ""), nil
}

type blockingVerifier struct {
	started chan struct{}
}

func (v *blockingVerifier) VerifyDraft(ctx context.Context, req VerificationRequest) (FinalVerification, error) {
	close(v.started)
	select {}
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
