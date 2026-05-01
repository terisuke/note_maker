package note

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	domain "github.com/teradakousuke/note_maker/internal/domain/article"
)

const userAgent = "note-maker/2026.05 (+https://github.com/terisuke/note_maker)"

// Fetcher retrieves public note.com article content.
type Fetcher struct {
	client *http.Client
}

// NewFetcher creates a Note fetcher.
func NewFetcher() *Fetcher {
	return NewFetcherWithClient(&http.Client{Timeout: 15 * time.Second})
}

// NewFetcherWithClient creates a Note fetcher with a caller-provided HTTP client.
func NewFetcherWithClient(client *http.Client) *Fetcher {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &Fetcher{client: client}
}

// FetchArticle retrieves one public note.com article.
func (f *Fetcher) FetchArticle(ctx context.Context, articleURL string) (*domain.Article, error) {
	normalizedURL, err := validateNoteURL(articleURL)
	if err != nil {
		return nil, err
	}
	if article, err := f.fetchArticlePage(ctx, normalizedURL); err == nil && strings.TrimSpace(article.Content) != "" {
		return article, nil
	}

	noteID := extractNoteID(normalizedURL)
	if noteID == "" {
		return nil, fmt.Errorf("could not extract note id from %s", normalizedURL)
	}
	return f.fetchArticleFromCompatibilityAPI(ctx, normalizedURL, noteID)
}

// FetchUserLatestArticles retrieves recent public articles through RSS first.
func (f *Fetcher) FetchUserLatestArticles(ctx context.Context, username string, limit int) ([]domain.Article, error) {
	username = strings.Trim(strings.TrimSpace(username), "/")
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if limit <= 0 {
		limit = 3
	}

	articles, err := f.fetchUserRSS(ctx, username, limit)
	if err == nil && len(articles) > 0 {
		return articles, nil
	}
	return f.fetchUserFromCompatibilityAPI(ctx, username, limit)
}

func (f *Fetcher) fetchArticlePage(ctx context.Context, articleURL string) (*domain.Article, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, articleURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create article page request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)

	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch article page: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch article page: unexpected status %s", response.Status)
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		return nil, fmt.Errorf("parse article page: %w", err)
	}
	title := firstNonEmpty(
		selectionAttr(doc, `meta[property="og:title"]`, "content"),
		selectionText(doc, "h1"),
		selectionText(doc, "title"),
	)
	content := firstNonEmpty(
		selectionBlockText(doc, `article`),
		selectionBlockText(doc, `[data-testid="note-body"]`),
		selectionBlockText(doc, `.o-noteContentText`),
		selectionBlockText(doc, `.note-common-styles__textnote-body`),
		selectionBlockText(doc, `main`),
	)
	content = normalizeParagraphs(content)
	if content == "" {
		return nil, fmt.Errorf("article page did not contain extractable body")
	}
	return &domain.Article{URL: articleURL, Title: normalizeWhitespace(title), Content: content}, nil
}

func (f *Fetcher) fetchUserRSS(ctx context.Context, username string, limit int) ([]domain.Article, error) {
	rssURL := fmt.Sprintf("https://note.com/%s/rss", url.PathEscape(username))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rssURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create rss request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch rss: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch rss: unexpected status %s", response.Status)
	}

	var feed rssFeed
	if err := xml.NewDecoder(response.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("decode rss: %w", err)
	}
	articles := make([]domain.Article, 0, limit)
	for _, item := range feed.Channel.Items {
		if len(articles) >= limit {
			break
		}
		if strings.TrimSpace(item.Link) == "" {
			continue
		}
		article, err := f.FetchArticle(ctx, item.Link)
		if err != nil {
			description := htmlToParagraphText(item.Description)
			if description == "" {
				continue
			}
			article = &domain.Article{URL: item.Link, Title: item.Title, Content: description}
		}
		articles = append(articles, *article)
	}
	return articles, nil
}

func (f *Fetcher) fetchArticleFromCompatibilityAPI(ctx context.Context, articleURL, noteID string) (*domain.Article, error) {
	apiURL := fmt.Sprintf("https://note.com/api/v3/notes/%s", url.PathEscape(noteID))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create compatibility API request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch compatibility API article: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch compatibility API article: unexpected status %s", response.Status)
	}

	var payload struct {
		Data struct {
			Name string `json:"name"`
			Body string `json:"body"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode compatibility API article: %w", err)
	}
	content := htmlToParagraphText(payload.Data.Body)
	if content == "" {
		return nil, fmt.Errorf("compatibility API article body was empty")
	}
	return &domain.Article{URL: articleURL, Title: payload.Data.Name, Content: content}, nil
}

func (f *Fetcher) fetchUserFromCompatibilityAPI(ctx context.Context, username string, limit int) ([]domain.Article, error) {
	apiURL := fmt.Sprintf("https://note.com/api/v2/creators/%s/contents?kind=note&page=1", url.PathEscape(username))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create compatibility user request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch compatibility user articles: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch compatibility user articles: unexpected status %s", response.Status)
	}

	var payload struct {
		Data struct {
			Contents []struct {
				Name    string `json:"name"`
				NoteURL string `json:"noteUrl"`
				Status  string `json:"status"`
			} `json:"contents"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode compatibility user articles: %w", err)
	}
	articles := make([]domain.Article, 0, limit)
	for _, content := range payload.Data.Contents {
		if len(articles) >= limit {
			break
		}
		if content.Status != "" && content.Status != "published" {
			continue
		}
		article, err := f.FetchArticle(ctx, content.NoteURL)
		if err != nil {
			continue
		}
		if article.Title == "" {
			article.Title = content.Name
		}
		articles = append(articles, *article)
	}
	if len(articles) == 0 {
		return nil, fmt.Errorf("no public articles found for user %s", username)
	}
	return articles, nil
}

func validateNoteURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("invalid note URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("note URL must use http or https")
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "note.com" && !strings.HasSuffix(host, ".note.com") {
		return "", fmt.Errorf("URL host must be note.com")
	}
	return parsed.String(), nil
}

func extractNoteID(articleURL string) string {
	parsed, err := url.Parse(articleURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if part == "n" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
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
	selection.Find("h1,h2,h3,h4,p,li,blockquote,pre").Each(func(_ int, s *goquery.Selection) {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func stripHTML(value string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(value))
	if err != nil {
		return value
	}
	text := strings.TrimSpace(doc.Text())
	if text == "" && !strings.Contains(value, "<") {
		return value
	}
	return text
}

func htmlToParagraphText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(value))
	if err != nil {
		return normalizeParagraphs(value)
	}
	parts := make([]string, 0)
	doc.Find("h1,h2,h3,h4,p,li,blockquote,pre").Each(func(_ int, s *goquery.Selection) {
		text := normalizeWhitespace(s.Text())
		if text != "" {
			parts = append(parts, text)
		}
	})
	if len(parts) == 0 {
		return normalizeParagraphs(doc.Text())
	}
	return normalizeParagraphs(strings.Join(parts, "\n\n"))
}

func normalizeParagraphs(value string) string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n"), "\n")
	normalized := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = normalizeWhitespace(line)
		if line == "" {
			if !blank {
				normalized = append(normalized, "")
			}
			blank = true
			continue
		}
		blank = false
		normalized = append(normalized, line)
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
}
