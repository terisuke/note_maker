package source

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

// ZennFetcher reads Zenn public articles through the official RSS feed shape.
type ZennFetcher struct {
	rss  *RSSFetcher
	html *HTMLFetcher
}

// NewZennFetcher creates a Zenn fetcher.
func NewZennFetcher(client *http.Client) *ZennFetcher {
	return &ZennFetcher{
		rss:  NewRSSFetcher(client),
		html: NewHTMLFetcher(client),
	}
}

// FetchList fetches a user's public Zenn feed. `all=1` is used so older public
// posts are not silently hidden by the default feed size.
func (f *ZennFetcher) FetchList(ctx context.Context, ref sourcedomain.Ref, limit int) ([]sourcedomain.ArticleSnapshot, error) {
	user := strings.Trim(strings.TrimSpace(ref.Ref), "/")
	if user == "" && strings.TrimSpace(ref.URL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(ref.URL))
		if err != nil {
			return nil, fmt.Errorf("parse zenn url: %w", err)
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) > 0 {
			user = parts[0]
		}
	}
	if user == "" {
		return nil, fmt.Errorf("zenn user ref is required")
	}
	feedURL := fmt.Sprintf("https://zenn.dev/%s/feed?all=1", url.PathEscape(user))
	snapshots, err := f.rss.FetchList(ctx, sourcedomain.Ref{Kind: sourcedomain.KindZenn, Ref: user, URL: feedURL}, limit)
	if err != nil {
		return nil, err
	}
	for i := range snapshots {
		snapshots[i].Kind = sourcedomain.KindZenn
	}
	return snapshots, nil
}

// FetchArticle fetches one Zenn article page.
func (f *ZennFetcher) FetchArticle(ctx context.Context, ref sourcedomain.Ref) (*sourcedomain.ArticleSnapshot, error) {
	article, err := f.html.FetchArticle(ctx, sourcedomain.Ref{Kind: sourcedomain.KindHTML, URL: ref.URL})
	if err != nil {
		return nil, err
	}
	article.Kind = sourcedomain.KindZenn
	return article, nil
}
