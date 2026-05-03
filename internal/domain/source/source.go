package source

import (
	"fmt"
	"strings"
	"time"

	"github.com/teradakousuke/note_maker/internal/domain/article"
)

// Kind identifies a public writing source.
type Kind string

const (
	KindNote   Kind = "note"
	KindZenn   Kind = "zenn"
	KindQiita  Kind = "qiita"
	KindRSS    Kind = "rss"
	KindHTML   Kind = "html"
	KindGitHub Kind = "github"
)

// Ref points to a public author, feed, or article.
type Ref struct {
	Kind Kind   `json:"kind"`
	Ref  string `json:"ref,omitempty"`
	URL  string `json:"url,omitempty"`
}

// Validate rejects empty or unknown references.
func (r Ref) Validate() error {
	switch r.Kind {
	case KindNote, KindZenn, KindQiita, KindGitHub:
		if strings.TrimSpace(r.Ref) == "" && strings.TrimSpace(r.URL) == "" {
			return fmt.Errorf("%s source requires ref or url", r.Kind)
		}
	case KindRSS, KindHTML:
		if strings.TrimSpace(r.URL) == "" {
			return fmt.Errorf("%s source requires url", r.Kind)
		}
	default:
		return fmt.Errorf("unsupported source kind %q", r.Kind)
	}
	return nil
}

// ProfileSnapshot describes a source account/feed at fetch time.
type ProfileSnapshot struct {
	Kind      Kind      `json:"kind"`
	Ref       string    `json:"ref,omitempty"`
	URL       string    `json:"url,omitempty"`
	Title     string    `json:"title,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

// ArticleSnapshot is normalized source material from any public source.
type ArticleSnapshot struct {
	ID          string    `json:"id,omitempty"`
	Kind        Kind      `json:"kind"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	PublishedAt time.Time `json:"published_at,omitempty"`
	UpdatedAt   time.Time `json:"updated_at,omitempty"`
	FetchedAt   time.Time `json:"fetched_at"`
}

// ToArticle converts normalized source material into the existing article domain.
func (a ArticleSnapshot) ToArticle() article.Article {
	return article.Article{
		URL:     strings.TrimSpace(a.URL),
		Title:   strings.TrimSpace(a.Title),
		Content: strings.TrimSpace(a.Content),
	}
}

// Articles converts snapshots into article domain values.
func Articles(snapshots []ArticleSnapshot) []article.Article {
	articles := make([]article.Article, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if strings.TrimSpace(snapshot.Content) == "" {
			continue
		}
		articles = append(articles, snapshot.ToArticle())
	}
	return articles
}
