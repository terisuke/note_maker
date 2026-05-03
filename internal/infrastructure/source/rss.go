package source

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

const userAgent = "note-maker/2026.05 (+https://github.com/terisuke/note_maker)"

// RSSFetcher reads RSS 2.0 and Atom feeds.
type RSSFetcher struct {
	client *http.Client
}

// NewRSSFetcher creates a feed fetcher.
func NewRSSFetcher(client *http.Client) *RSSFetcher {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &RSSFetcher{client: client}
}

// FetchList returns feed entries in feed order, normally newest first.
func (f *RSSFetcher) FetchList(ctx context.Context, ref sourcedomain.Ref, limit int) ([]sourcedomain.ArticleSnapshot, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 5
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSpace(ref.URL), nil)
	if err != nil {
		return nil, fmt.Errorf("create feed request: %w", err)
	}
	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.8")
	response, err := f.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch feed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch feed: unexpected status %s", response.Status)
	}
	return parseFeed(response.Body, ref, limit, time.Now().UTC())
}

// FetchArticle returns the first matching feed item when the ref URL is a feed.
func (f *RSSFetcher) FetchArticle(ctx context.Context, ref sourcedomain.Ref) (*sourcedomain.ArticleSnapshot, error) {
	items, err := f.FetchList(ctx, ref, 1)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("feed contained no articles")
	}
	return &items[0], nil
}

func parseFeed(reader io.Reader, ref sourcedomain.Ref, limit int, fetchedAt time.Time) ([]sourcedomain.ArticleSnapshot, error) {
	encoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read feed: %w", err)
	}
	var rss rssDocument
	if err := xml.Unmarshal(encoded, &rss); err == nil && len(rss.Channel.Items) > 0 {
		return rssSnapshots(rss.Channel.Items, ref, limit, fetchedAt), nil
	}
	var atom atomFeed
	if err := xml.Unmarshal(encoded, &atom); err == nil && len(atom.Entries) > 0 {
		return atomSnapshots(atom.Entries, ref, limit, fetchedAt), nil
	}
	return nil, fmt.Errorf("feed contained no RSS items or Atom entries")
}

func rssSnapshots(items []rssItem, ref sourcedomain.Ref, limit int, fetchedAt time.Time) []sourcedomain.ArticleSnapshot {
	snapshots := make([]sourcedomain.ArticleSnapshot, 0, min(limit, len(items)))
	for _, item := range items {
		if len(snapshots) >= limit {
			break
		}
		link := strings.TrimSpace(item.Link)
		if link == "" {
			link = strings.TrimSpace(item.GUID.Value)
		}
		content := firstNonEmpty(item.EncodedContent, item.Description)
		content = htmlToParagraphText(content)
		if content == "" {
			continue
		}
		publishedAt := parseFeedTime(firstNonEmpty(item.PubDate, item.Date))
		snapshots = append(snapshots, sourcedomain.ArticleSnapshot{
			ID:          firstNonEmpty(item.GUID.Value, link),
			Kind:        ref.Kind,
			URL:         link,
			Title:       normalizeWhitespace(item.Title),
			Content:     content,
			PublishedAt: publishedAt,
			UpdatedAt:   publishedAt,
			FetchedAt:   fetchedAt,
		})
	}
	return snapshots
}

func atomSnapshots(entries []atomEntry, ref sourcedomain.Ref, limit int, fetchedAt time.Time) []sourcedomain.ArticleSnapshot {
	snapshots := make([]sourcedomain.ArticleSnapshot, 0, min(limit, len(entries)))
	for _, entry := range entries {
		if len(snapshots) >= limit {
			break
		}
		link := atomEntryLink(entry)
		content := htmlToParagraphText(cleanXMLText(firstNonEmpty(entry.Content.Value, entry.Summary.Value)))
		if content == "" {
			continue
		}
		publishedAt := parseFeedTime(firstNonEmpty(entry.Published, entry.Updated))
		updatedAt := parseFeedTime(entry.Updated)
		snapshots = append(snapshots, sourcedomain.ArticleSnapshot{
			ID:          firstNonEmpty(entry.ID, link),
			Kind:        ref.Kind,
			URL:         link,
			Title:       normalizeWhitespace(entry.Title),
			Content:     content,
			PublishedAt: publishedAt,
			UpdatedAt:   updatedAt,
			FetchedAt:   fetchedAt,
		})
	}
	return snapshots
}

func atomEntryLink(entry atomEntry) string {
	for _, link := range entry.Links {
		if strings.TrimSpace(link.Rel) == "" || link.Rel == "alternate" {
			if strings.TrimSpace(link.Href) != "" {
				return strings.TrimSpace(link.Href)
			}
		}
	}
	if len(entry.Links) > 0 {
		return strings.TrimSpace(entry.Links[0].Href)
	}
	return ""
}

func parseFeedTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

type rssDocument struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title          string  `xml:"title"`
	Link           string  `xml:"link"`
	Description    string  `xml:"description"`
	PubDate        string  `xml:"pubDate"`
	Date           string  `xml:"http://purl.org/dc/elements/1.1/ date"`
	EncodedContent string  `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	GUID           rssGUID `xml:"guid"`
}

type rssGUID struct {
	Value string `xml:",chardata"`
}

type atomFeed struct {
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	ID        string       `xml:"id"`
	Title     string       `xml:"title"`
	Updated   string       `xml:"updated"`
	Published string       `xml:"published"`
	Links     []atomLink   `xml:"link"`
	Summary   atomTextNode `xml:"summary"`
	Content   atomTextNode `xml:"content"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomTextNode struct {
	Value string `xml:",innerxml"`
}

func cleanXMLText(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "<![CDATA[")
	value = strings.TrimSuffix(value, "]]>")
	return strings.TrimSpace(value)
}
