package source

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

// HTMLFetcher extracts readable content from public HTML pages.
type HTMLFetcher struct {
	client *http.Client
}

// NewHTMLFetcher creates a semantic HTML fetcher.
func NewHTMLFetcher(client *http.Client) *HTMLFetcher {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &HTMLFetcher{client: client}
}

// FetchArticle retrieves one public HTML article.
func (f *HTMLFetcher) FetchArticle(ctx context.Context, ref sourcedomain.Ref) (*sourcedomain.ArticleSnapshot, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(ref.URL), nil)
	if err != nil {
		return nil, fmt.Errorf("create html request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch html: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch html: unexpected status %s", response.Status)
	}
	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}
	title := firstNonEmpty(
		selectionAttr(doc, `meta[property="og:title"]`, "content"),
		selectionAttr(doc, `meta[name="twitter:title"]`, "content"),
		selectionText(doc, "h1"),
		selectionText(doc, "title"),
	)
	content := firstNonEmpty(
		selectionBlockText(doc, "article"),
		selectionBlockText(doc, "main"),
		selectionBlockText(doc, `[role="main"]`),
		selectionBlockText(doc, ".content"),
	)
	if content == "" {
		return nil, fmt.Errorf("html did not contain extractable article content")
	}
	now := time.Now().UTC()
	return &sourcedomain.ArticleSnapshot{
		ID:        ref.URL,
		Kind:      sourcedomain.KindHTML,
		URL:       ref.URL,
		Title:     normalizeWhitespace(title),
		Content:   content,
		FetchedAt: now,
	}, nil
}

// FetchList is intentionally unsupported for arbitrary HTML because page-list
// extraction is site-specific and would silently scrape the wrong surface.
func (f *HTMLFetcher) FetchList(ctx context.Context, ref sourcedomain.Ref, limit int) ([]sourcedomain.ArticleSnapshot, error) {
	article, err := f.FetchArticle(ctx, ref)
	if err != nil {
		return nil, err
	}
	return []sourcedomain.ArticleSnapshot{*article}, nil
}

func selectionText(doc *goquery.Document, selector string) string {
	return strings.TrimSpace(doc.Find(selector).First().Text())
}

func selectionBlockText(doc *goquery.Document, selector string) string {
	selection := doc.Find(selector).First()
	if selection.Length() == 0 {
		return ""
	}
	parts := make([]string, 0)
	selection.Find("h1,h2,h3,h4,p,li,blockquote,pre,code").Each(func(_ int, s *goquery.Selection) {
		text := normalizeWhitespace(s.Text())
		if text != "" {
			parts = append(parts, text)
		}
	})
	if len(parts) == 0 {
		return normalizeParagraphs(selection.Text())
	}
	return normalizeParagraphs(strings.Join(parts, "\n\n"))
}

func selectionAttr(doc *goquery.Document, selector, attr string) string {
	value, _ := doc.Find(selector).First().Attr(attr)
	return strings.TrimSpace(value)
}
