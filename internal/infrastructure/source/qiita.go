package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

// QiitaFetcher reads public Qiita posts through Qiita API v2.
type QiitaFetcher struct {
	client *http.Client
}

// NewQiitaFetcher creates a Qiita API fetcher.
func NewQiitaFetcher(client *http.Client) *QiitaFetcher {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &QiitaFetcher{client: client}
}

// FetchList fetches public posts from a user in newest order.
func (f *QiitaFetcher) FetchList(ctx context.Context, ref sourcedomain.Ref, limit int) ([]sourcedomain.ArticleSnapshot, error) {
	userID := strings.Trim(strings.TrimSpace(ref.Ref), "/")
	if userID == "" && strings.TrimSpace(ref.URL) != "" {
		parsed, err := url.Parse(strings.TrimSpace(ref.URL))
		if err != nil {
			return nil, fmt.Errorf("parse qiita url: %w", err)
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) > 0 {
			userID = parts[0]
		}
	}
	if userID == "" {
		return nil, fmt.Errorf("qiita user ref is required")
	}
	if limit <= 0 {
		limit = 5
	}
	apiURL := fmt.Sprintf("https://qiita.com/api/v2/users/%s/items?page=1&per_page=%d", url.PathEscape(userID), min(limit, 100))
	var payload []qiitaItem
	if err := f.getJSON(ctx, apiURL, &payload); err != nil {
		return nil, err
	}
	return qiitaSnapshots(payload, limit), nil
}

// FetchArticle fetches one public Qiita item by item id parsed from its URL.
func (f *QiitaFetcher) FetchArticle(ctx context.Context, ref sourcedomain.Ref) (*sourcedomain.ArticleSnapshot, error) {
	itemID := qiitaItemID(ref.URL)
	if itemID == "" {
		return nil, fmt.Errorf("could not extract qiita item id from %s", ref.URL)
	}
	apiURL := fmt.Sprintf("https://qiita.com/api/v2/items/%s", url.PathEscape(itemID))
	var payload qiitaItem
	if err := f.getJSON(ctx, apiURL, &payload); err != nil {
		return nil, err
	}
	snapshots := qiitaSnapshots([]qiitaItem{payload}, 1)
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("qiita item had no body")
	}
	return &snapshots[0], nil
}

func (f *QiitaFetcher) getJSON(ctx context.Context, apiURL string, out any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("create qiita request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/json")
	response, err := f.client.Do(request)
	if err != nil {
		return fmt.Errorf("fetch qiita api: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("fetch qiita api: unexpected status %s", response.Status)
	}
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		return fmt.Errorf("decode qiita api: %w", err)
	}
	return nil
}

func qiitaSnapshots(items []qiitaItem, limit int) []sourcedomain.ArticleSnapshot {
	now := time.Now().UTC()
	snapshots := make([]sourcedomain.ArticleSnapshot, 0, min(limit, len(items)))
	for _, item := range items {
		if len(snapshots) >= limit {
			break
		}
		content := strings.TrimSpace(item.Body)
		if content == "" {
			content = htmlToParagraphText(item.RenderedBody)
		}
		if content == "" {
			continue
		}
		createdAt := parseFeedTime(item.CreatedAt)
		updatedAt := parseFeedTime(item.UpdatedAt)
		snapshots = append(snapshots, sourcedomain.ArticleSnapshot{
			ID:          item.ID,
			Kind:        sourcedomain.KindQiita,
			URL:         item.URL,
			Title:       item.Title,
			Content:     content,
			PublishedAt: createdAt,
			UpdatedAt:   updatedAt,
			FetchedAt:   now,
		})
	}
	return snapshots
}

func qiitaItemID(rawURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if part == "items" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

type qiitaItem struct {
	ID           string `json:"id"`
	URL          string `json:"url"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	RenderedBody string `json:"rendered_body"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}
