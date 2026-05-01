package authorstyle

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/teradakousuke/note_maker/internal/domain/article"
)

type fakeFetcher struct {
	latestArticles []article.Article
	articlesByURL  map[string]*article.Article
	latestUsername string
	latestLimit    int
	err            error
}

func (f *fakeFetcher) FetchArticle(_ context.Context, articleURL string) (*article.Article, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.articlesByURL[articleURL], nil
}

func (f *fakeFetcher) FetchUserLatestArticles(_ context.Context, username string, limit int) ([]article.Article, error) {
	f.latestUsername = username
	f.latestLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.latestArticles, nil
}

func TestAnalyzeFetchesLatestArticlesAndBuildsResult(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	fetcher := &fakeFetcher{
		latestArticles: []article.Article{
			{
				URL:   "https://note.com/tera/n/n111",
				Title: "AIと起業",
				Content: `# AIと起業

僕はAIで起業の違和感を言語化する。
「これなら進める」と思った。`,
			},
			{
				URL:     "https://note.com/tera/n/n222",
				Title:   "音楽",
				Content: "僕は音楽から学んだテンポを、記事にも持ち込む。",
			},
			{URL: "https://note.com/tera/n/empty", Title: "empty"},
		},
	}
	service := NewAnalyzeAuthorStyleService(fetcher, testLogger())

	result, err := service.Analyze(context.Background(), AnalyzeRequest{
		Username:  " tera ",
		Limit:     2,
		FetchedAt: fetchedAt,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if fetcher.latestUsername != "tera" || fetcher.latestLimit != 2 {
		t.Fatalf("unexpected fetch args: username=%q limit=%d", fetcher.latestUsername, fetcher.latestLimit)
	}
	if result.ID == "" || result.Profile.ID == "" || result.Guide.ID == "" {
		t.Fatalf("expected stable ids: %#v", result)
	}
	if result.ArticleCount != 2 {
		t.Fatalf("unexpected article count: %d", result.ArticleCount)
	}
	if result.Source.FetchedAt != fetchedAt || len(result.Source.Articles) != 2 {
		t.Fatalf("unexpected source metadata: %#v", result.Source)
	}
	if result.Guide.ProfileID != result.Profile.ID {
		t.Fatalf("guide does not reference profile: %#v", result.Guide)
	}
	if result.Profile.Metrics.KeywordCounts["AI"] == 0 {
		t.Fatalf("expected AnalyzeStyleCorpus metrics: %#v", result.Profile.Metrics.KeywordCounts)
	}
}

func TestAnalyzeFetchesExplicitArticleURLs(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	target := "https://note.com/tera/n/n333"
	fetcher := &fakeFetcher{
		articlesByURL: map[string]*article.Article{
			target: {
				URL:     target,
				Title:   "違和感",
				Content: "私が起業で感じた違和感を書く。\n\n読者の挑戦につなげたい。",
			},
		},
	}
	service := NewAnalyzeAuthorStyleService(fetcher, testLogger())

	result, err := service.Analyze(context.Background(), AnalyzeRequest{
		ArticleURLs: []string{"", " " + target + " "},
		FetchedAt:   fetchedAt,
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if result.ArticleCount != 1 {
		t.Fatalf("unexpected article count: %d", result.ArticleCount)
	}
	if result.Source.Articles[0].ID != "n333" {
		t.Fatalf("unexpected article source: %#v", result.Source.Articles[0])
	}
	if result.Profile.PreferredFirstPerson != "私" {
		t.Fatalf("unexpected preferred first person: %q", result.Profile.PreferredFirstPerson)
	}
}

func TestAnalyzeReturnsFetcherErrors(t *testing.T) {
	service := NewAnalyzeAuthorStyleService(&fakeFetcher{err: errors.New("offline")}, testLogger())

	_, err := service.Analyze(context.Background(), AnalyzeRequest{Username: "tera"})
	if err == nil {
		t.Fatal("expected fetcher error")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
