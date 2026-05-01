package note

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	domain "github.com/teradakousuke/note_maker/internal/domain/article"
)

// APIClient accesses note.com's JSON endpoints for explicit compatibility tests.
type APIClient struct {
	client *http.Client
}

// NewAPIClient creates a client for note.com's JSON endpoints.
func NewAPIClient(client *http.Client) *APIClient {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &APIClient{client: client}
}

// FetchUserArticles retrieves public articles through note.com's v2/v3 JSON endpoints.
func (c *APIClient) FetchUserArticles(ctx context.Context, username string, limit int) ([]domain.Article, error) {
	username = strings.Trim(strings.TrimSpace(username), "/")
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}
	if limit <= 0 {
		limit = 5
	}

	metas, err := c.fetchUserContents(ctx, username)
	if err != nil {
		return nil, err
	}
	articles := make([]domain.Article, 0, limit)
	for _, meta := range metas {
		if len(articles) >= limit {
			break
		}
		if meta.Status != "" && meta.Status != "published" {
			continue
		}
		key := firstNonEmpty(meta.Key, extractNoteID(meta.NoteURL))
		if key == "" {
			continue
		}
		article, err := c.fetchNote(ctx, key, meta.NoteURL)
		if err != nil {
			continue
		}
		if article.Title == "" {
			article.Title = meta.Name
		}
		if strings.TrimSpace(article.Content) == "" {
			continue
		}
		articles = append(articles, *article)
	}
	if len(articles) == 0 {
		return nil, fmt.Errorf("no public articles found for user %s through note API", username)
	}
	return articles, nil
}

func (c *APIClient) fetchUserContents(ctx context.Context, username string) ([]noteContentMeta, error) {
	apiURL := fmt.Sprintf("https://note.com/api/v2/creators/%s/contents?kind=note&page=1", url.PathEscape(username))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create note API contents request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch note API contents: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch note API contents: unexpected status %s", response.Status)
	}

	var payload struct {
		Data struct {
			Contents []noteContentMeta `json:"contents"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode note API contents: %w", err)
	}
	return payload.Data.Contents, nil
}

func (c *APIClient) fetchNote(ctx context.Context, key, fallbackURL string) (*domain.Article, error) {
	apiURL := fmt.Sprintf("https://note.com/api/v3/notes/%s", url.PathEscape(key))
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create note API detail request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)

	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch note API detail: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch note API detail: unexpected status %s", response.Status)
	}

	var payload struct {
		Data struct {
			Name    string `json:"name"`
			Body    string `json:"body"`
			NoteURL string `json:"noteUrl"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode note API detail: %w", err)
	}
	content := htmlToParagraphText(payload.Data.Body)
	if content == "" {
		return nil, fmt.Errorf("note API detail body was empty")
	}
	return &domain.Article{
		URL:     firstNonEmpty(payload.Data.NoteURL, fallbackURL),
		Title:   normalizeWhitespace(payload.Data.Name),
		Content: content,
	}, nil
}

type noteContentMeta struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	NoteURL string `json:"noteUrl"`
	Status  string `json:"status"`
}
