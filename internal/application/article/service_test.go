package article

import (
	"context"
	"strings"
	"testing"

	domain "github.com/teradakousuke/note_maker/internal/domain/article"
)

func TestGenerateArticleFetchesArticleAndBuildsPrompt(t *testing.T) {
	fetcher := &fakeFetcher{
		article: &domain.Article{URL: "https://note.com/u/n/n1", Title: "Ref", Content: "参考本文"},
	}
	generator := &fakeGenerator{draft: "# 生成結果"}
	service := NewService(fetcher, generator, nil)

	draft, err := service.GenerateArticle(context.Background(), domain.GenerationRequest{
		NoteURL: "https://note.com/u/n/n1",
		Theme:   "ローカルLLM",
	})
	if err != nil {
		t.Fatalf("generate article: %v", err)
	}
	if draft != "# 生成結果" {
		t.Fatalf("unexpected draft: %q", draft)
	}
	if !strings.Contains(generator.prompt, "参考本文") {
		t.Fatalf("prompt did not include reference article: %s", generator.prompt)
	}
	if !strings.Contains(generator.prompt, "文体: ですます調") {
		t.Fatalf("prompt did not include default style: %s", generator.prompt)
	}
}

func TestGenerateArticleRequiresSourceAndTheme(t *testing.T) {
	service := NewService(&fakeFetcher{}, &fakeGenerator{}, nil)

	if _, err := service.GenerateArticle(context.Background(), domain.GenerationRequest{Theme: "x"}); err == nil {
		t.Fatal("expected missing source error")
	}
	if _, err := service.GenerateArticle(context.Background(), domain.GenerationRequest{NoteURL: "https://note.com/u/n/n1"}); err == nil {
		t.Fatal("expected missing theme error")
	}
}

func TestGenerateArticleRejectsUnusableDraft(t *testing.T) {
	service := NewService(
		&fakeFetcher{article: &domain.Article{Content: "参考本文"}},
		&fakeGenerator{draft: "承知しました。記事を書きます。"},
		nil,
	)

	if _, err := service.GenerateArticle(context.Background(), domain.GenerationRequest{
		NoteURL: "https://note.com/u/n/n1",
		Theme:   "ローカルLLM",
	}); err == nil {
		t.Fatal("expected unusable draft error")
	}
}

func TestBuildPromptIncludesPasteReadyConstraints(t *testing.T) {
	prompt := BuildPrompt(domain.GenerationRequest{
		Theme:       "ローカルLLM",
		StyleChoice: "ですます調",
		ToneChoice:  "客観的",
	}, []domain.Article{{Content: strings.Repeat("あ", 7000)}})

	for _, want := range []string{
		"# タイトル",
		"コードフェンスで囲まない",
		"Noteに貼り付けるMarkdown本文だけ",
		"参考記事は長いためここで省略",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt does not contain %q:\n%s", want, prompt)
		}
	}
}

type fakeFetcher struct {
	article  *domain.Article
	articles []domain.Article
	err      error
}

func (f *fakeFetcher) FetchArticle(ctx context.Context, articleURL string) (*domain.Article, error) {
	return f.article, f.err
}

func (f *fakeFetcher) FetchUserLatestArticles(ctx context.Context, username string, limit int) ([]domain.Article, error) {
	return f.articles, f.err
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
