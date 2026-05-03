package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
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
	feedSnapshots, err := f.rss.FetchList(ctx, sourcedomain.Ref{Kind: sourcedomain.KindZenn, Ref: user, URL: feedURL}, limit)
	if err != nil {
		return nil, err
	}
	snapshots := make([]sourcedomain.ArticleSnapshot, 0, len(feedSnapshots))
	for _, feedSnapshot := range feedSnapshots {
		feedSnapshot.Kind = sourcedomain.KindZenn
		article, err := f.FetchArticle(ctx, sourcedomain.Ref{Kind: sourcedomain.KindZenn, URL: feedSnapshot.URL})
		if err != nil || article == nil || strings.TrimSpace(article.Content) == "" {
			snapshots = append(snapshots, feedSnapshot)
			continue
		}
		article.ID = firstNonEmpty(feedSnapshot.ID, article.ID)
		article.Title = firstNonEmpty(feedSnapshot.Title, article.Title)
		article.PublishedAt = firstNonZeroTime(feedSnapshot.PublishedAt, article.PublishedAt)
		article.UpdatedAt = firstNonZeroTime(feedSnapshot.UpdatedAt, article.UpdatedAt)
		article.FetchedAt = firstNonZeroTime(article.FetchedAt, feedSnapshot.FetchedAt)
		snapshots = append(snapshots, *article)
	}
	return snapshots, nil
}

// FetchArticle fetches one Zenn article page.
func (f *ZennFetcher) FetchArticle(ctx context.Context, ref sourcedomain.Ref) (*sourcedomain.ArticleSnapshot, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(ref.URL), nil)
	if err != nil {
		return nil, fmt.Errorf("create zenn article request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	response, err := f.html.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch zenn article: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch zenn article: unexpected status %s", response.Status)
	}
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, fmt.Errorf("parse zenn article html: %w", err)
	}
	title := firstNonEmpty(
		selectionAttr(doc, `meta[property="og:title"]`, "content"),
		selectionText(doc, "h1"),
		selectionText(doc, "title"),
	)
	nextData := zennNextDataFromDocument(doc)
	if strings.TrimSpace(nextData.Props.PageProps.Article.BodyHTML) != "" {
		content := htmlToParagraphText(nextData.Props.PageProps.Article.BodyHTML)
		if content != "" {
			return &sourcedomain.ArticleSnapshot{
				ID:        ref.URL,
				Kind:      sourcedomain.KindZenn,
				URL:       ref.URL,
				Title:     normalizeWhitespace(firstNonEmpty(nextData.Props.PageProps.Article.Title, title)),
				Content:   content,
				FetchedAt: time.Now().UTC(),
			}, nil
		}
	}
	article, err := f.html.FetchArticle(ctx, sourcedomain.Ref{Kind: sourcedomain.KindHTML, URL: ref.URL})
	if err != nil {
		return nil, err
	}
	article.Kind = sourcedomain.KindZenn
	return article, nil
}

func zennNextDataFromDocument(doc *goquery.Document) zennNextData {
	var data zennNextData
	raw := strings.TrimSpace(doc.Find("script#__NEXT_DATA__").First().Text())
	if raw == "" {
		return data
	}
	_ = json.Unmarshal([]byte(raw), &data)
	return data
}

type zennNextData struct {
	Props struct {
		PageProps struct {
			Article struct {
				Title    string `json:"title"`
				BodyHTML string `json:"bodyHtml"`
			} `json:"article"`
		} `json:"pageProps"`
	} `json:"props"`
}
