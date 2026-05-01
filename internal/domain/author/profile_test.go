package author

import (
	"testing"
	"time"

	"github.com/teradakousuke/note_maker/internal/domain/article"
)

func TestBuildAuthorStyleProfileDerivesMetricsAndMetadata(t *testing.T) {
	fetchedAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	articles := []article.Article{
		{
			URL:   "https://note.com/tera/n/n111",
			Title: "AIと違和感",
			Content: `僕はAIについて考えている。
「違和感」を言語化することが、次の挑戦になる。`,
		},
		{
			URL:   "https://note.com/tera/n/n222",
			Title: "音楽と起業",
			Content: `# 音楽から起業へ

僕は音楽の現場で学んだことを、起業にも持ち込んでいる。`,
		},
	}

	profile, err := BuildAuthorStyleProfile(AuthorSource{
		Username:  "tera",
		FetchedAt: fetchedAt,
	}, articles)
	if err != nil {
		t.Fatalf("BuildAuthorStyleProfile returned error: %v", err)
	}

	if profile.ID == "" {
		t.Fatal("expected stable profile id")
	}
	if profile.Source.Username != "tera" {
		t.Fatalf("unexpected source username: %q", profile.Source.Username)
	}
	if len(profile.Source.Articles) != 2 {
		t.Fatalf("unexpected source articles: %#v", profile.Source.Articles)
	}
	if profile.Source.Articles[0].ID != "n111" {
		t.Fatalf("unexpected source article id: %#v", profile.Source.Articles[0])
	}
	if profile.ArticleCount != 2 {
		t.Fatalf("unexpected article count: %d", profile.ArticleCount)
	}
	if profile.Metrics.KeywordCounts["AI"] == 0 || profile.Metrics.KeywordCounts["音楽"] == 0 {
		t.Fatalf("expected article style corpus metrics: %#v", profile.Metrics.KeywordCounts)
	}
	if profile.PreferredFirstPerson != "僕" {
		t.Fatalf("unexpected preferred first person: %q", profile.PreferredFirstPerson)
	}
}

func TestBuildWritingStyleGuideValidatesRequiredGuidance(t *testing.T) {
	profile, err := BuildAuthorStyleProfile(AuthorSource{
		Username: "tera",
		Articles: []SourceArticle{
			{ID: "n111", URL: "https://note.com/tera/n/n111"},
		},
	}, []article.Article{
		{
			URL:     "https://note.com/tera/n/n111",
			Title:   "違和感",
			Content: "僕はAIの違和感を書く。\n\n「これは大事だ」と思った。",
		},
	})
	if err != nil {
		t.Fatalf("BuildAuthorStyleProfile returned error: %v", err)
	}

	guide, err := BuildWritingStyleGuide(profile)
	if err != nil {
		t.Fatalf("BuildWritingStyleGuide returned error: %v", err)
	}
	if guide.ID == "" || guide.ProfileID != profile.ID {
		t.Fatalf("unexpected guide ids: %#v", guide)
	}
	if guide.PreferredFirstPerson != "僕" {
		t.Fatalf("unexpected first person guidance: %q", guide.PreferredFirstPerson)
	}
	if len(guide.RecurringThemes) == 0 || guide.ParagraphRhythm == "" || guide.QuoteGuidance == "" || guide.Markdown == "" {
		t.Fatalf("expected complete guide: %#v", guide)
	}

	guide.ProfileID = ""
	if err := guide.Validate(); err == nil {
		t.Fatal("expected missing profile id to be invalid")
	}
}
