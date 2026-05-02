package source

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
	notenote "github.com/teradakousuke/note_maker/internal/infrastructure/note"
)

// Router dispatches public source refs to concrete fetchers.
type Router struct {
	note  *notenote.Fetcher
	zenn  *ZennFetcher
	qiita *QiitaFetcher
	rss   *RSSFetcher
	html  *HTMLFetcher
}

// NewRouter creates a source router with shared HTTP settings.
func NewRouter() *Router {
	return NewRouterWithClient(nil)
}

// NewRouterWithClient creates a source router with a caller-provided client.
func NewRouterWithClient(client *http.Client) *Router {
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	return &Router{
		note:  notenote.NewFetcherWithClient(client),
		zenn:  NewZennFetcher(client),
		qiita: NewQiitaFetcher(client),
		rss:   NewRSSFetcher(client),
		html:  NewHTMLFetcher(client),
	}
}

// FetchList fetches recent articles for an author/feed ref.
func (r *Router) FetchList(ctx context.Context, ref sourcedomain.Ref, limit int) ([]sourcedomain.ArticleSnapshot, error) {
	ref = normalizeRef(ref)
	switch ref.Kind {
	case sourcedomain.KindNote:
		articles, err := r.note.FetchUserLatestArticles(ctx, ref.Ref, limit)
		return snapshotsFromArticles(sourcedomain.KindNote, articles), err
	case sourcedomain.KindZenn:
		return r.zenn.FetchList(ctx, ref, limit)
	case sourcedomain.KindQiita:
		return r.qiita.FetchList(ctx, ref, limit)
	case sourcedomain.KindRSS:
		return r.rss.FetchList(ctx, ref, limit)
	case sourcedomain.KindHTML:
		return r.html.FetchList(ctx, ref, limit)
	default:
		return nil, fmt.Errorf("unsupported source kind %q", ref.Kind)
	}
}

// FetchArticle fetches one article by URL.
func (r *Router) FetchArticle(ctx context.Context, ref sourcedomain.Ref) (*sourcedomain.ArticleSnapshot, error) {
	ref = normalizeRef(ref)
	switch ref.Kind {
	case sourcedomain.KindNote:
		article, err := r.note.FetchArticle(ctx, ref.URL)
		if err != nil {
			return nil, err
		}
		snapshot := snapshotFromArticle(sourcedomain.KindNote, *article)
		return &snapshot, nil
	case sourcedomain.KindZenn:
		return r.zenn.FetchArticle(ctx, ref)
	case sourcedomain.KindQiita:
		return r.qiita.FetchArticle(ctx, ref)
	case sourcedomain.KindRSS:
		return r.rss.FetchArticle(ctx, ref)
	case sourcedomain.KindHTML:
		return r.html.FetchArticle(ctx, ref)
	default:
		return nil, fmt.Errorf("unsupported source kind %q", ref.Kind)
	}
}

// AuthorStyleFetcher adapts Router to application/authorstyle.SourceFetcher.
type AuthorStyleFetcher struct {
	router *Router
}

// NewAuthorStyleFetcher creates an author-style source adapter.
func NewAuthorStyleFetcher() *AuthorStyleFetcher {
	return &AuthorStyleFetcher{router: NewRouter()}
}

// NewAuthorStyleFetcherWithClient creates a testable author-style source adapter.
func NewAuthorStyleFetcherWithClient(client *http.Client) *AuthorStyleFetcher {
	return &AuthorStyleFetcher{router: NewRouterWithClient(client)}
}

// FetchArticle routes by URL host. note.com remains enforced inside the note fetcher only.
func (f *AuthorStyleFetcher) FetchArticle(ctx context.Context, articleURL string) (*articledomain.Article, error) {
	snapshot, err := f.router.FetchArticle(ctx, RefFromURL(articleURL))
	if err != nil {
		return nil, err
	}
	article := snapshot.ToArticle()
	return &article, nil
}

// FetchUserLatestArticles supports explicit refs such as zenn:cloudia,
// qiita:Cloudia_Cor_Inc, rss:https://example.com/feed.xml, and legacy note usernames.
func (f *AuthorStyleFetcher) FetchUserLatestArticles(ctx context.Context, username string, limit int) ([]articledomain.Article, error) {
	snapshots, err := f.router.FetchList(ctx, RefFromSelector(username), limit)
	if err != nil {
		return nil, err
	}
	return sourcedomain.Articles(snapshots), nil
}

// RefFromSelector parses UI/API source selectors.
func RefFromSelector(selector string) sourcedomain.Ref {
	selector = strings.TrimSpace(selector)
	if parsed, ok := parseKindSelector(selector); ok {
		return parsed
	}
	if looksLikeURL(selector) {
		return RefFromURL(selector)
	}
	return sourcedomain.Ref{Kind: sourcedomain.KindNote, Ref: strings.Trim(selector, "/")}
}

// RefFromURL routes article/feed URLs by host.
func RefFromURL(rawURL string) sourcedomain.Ref {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return sourcedomain.Ref{Kind: sourcedomain.KindHTML, URL: rawURL}
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case host == "note.com" || strings.HasSuffix(host, ".note.com"):
		return sourcedomain.Ref{Kind: sourcedomain.KindNote, URL: rawURL}
	case host == "zenn.dev" || strings.HasSuffix(host, ".zenn.dev"):
		return sourcedomain.Ref{Kind: sourcedomain.KindZenn, URL: rawURL}
	case host == "qiita.com" || strings.HasSuffix(host, ".qiita.com"):
		return sourcedomain.Ref{Kind: sourcedomain.KindQiita, URL: rawURL}
	case strings.Contains(strings.ToLower(parsed.Path), "rss") || strings.Contains(strings.ToLower(parsed.Path), "feed") || strings.HasSuffix(strings.ToLower(parsed.Path), ".xml"):
		return sourcedomain.Ref{Kind: sourcedomain.KindRSS, URL: rawURL}
	default:
		return sourcedomain.Ref{Kind: sourcedomain.KindHTML, URL: rawURL}
	}
}

func normalizeRef(ref sourcedomain.Ref) sourcedomain.Ref {
	if ref.Kind == "" {
		if strings.TrimSpace(ref.URL) != "" {
			return RefFromURL(ref.URL)
		}
		return RefFromSelector(ref.Ref)
	}
	return ref
}

func parseKindSelector(selector string) (sourcedomain.Ref, bool) {
	prefix, value, ok := strings.Cut(selector, ":")
	if !ok {
		return sourcedomain.Ref{}, false
	}
	value = strings.TrimSpace(value)
	switch strings.ToLower(strings.TrimSpace(prefix)) {
	case "note":
		return sourcedomain.Ref{Kind: sourcedomain.KindNote, Ref: strings.Trim(value, "/")}, true
	case "zenn":
		return sourcedomain.Ref{Kind: sourcedomain.KindZenn, Ref: strings.Trim(value, "/")}, true
	case "qiita":
		return sourcedomain.Ref{Kind: sourcedomain.KindQiita, Ref: strings.Trim(value, "/")}, true
	case "rss":
		return sourcedomain.Ref{Kind: sourcedomain.KindRSS, URL: value}, true
	case "html":
		return sourcedomain.Ref{Kind: sourcedomain.KindHTML, URL: value}, true
	default:
		return sourcedomain.Ref{}, false
	}
}

func looksLikeURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme != "" && parsed.Host != ""
}

func snapshotsFromArticles(kind sourcedomain.Kind, articles []articledomain.Article) []sourcedomain.ArticleSnapshot {
	snapshots := make([]sourcedomain.ArticleSnapshot, 0, len(articles))
	for _, article := range articles {
		snapshots = append(snapshots, snapshotFromArticle(kind, article))
	}
	return snapshots
}

func snapshotFromArticle(kind sourcedomain.Kind, article articledomain.Article) sourcedomain.ArticleSnapshot {
	return sourcedomain.ArticleSnapshot{
		ID:      firstNonEmpty(article.URL, article.Title),
		Kind:    kind,
		URL:     article.URL,
		Title:   article.Title,
		Content: article.Content,
	}
}
