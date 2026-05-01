package authorstyle

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teradakousuke/note_maker/internal/domain/article"
	"github.com/teradakousuke/note_maker/internal/domain/author"
)

const defaultArticleLimit = 5

// SourceFetcher loads author source articles for style analysis.
type SourceFetcher interface {
	FetchArticle(ctx context.Context, articleURL string) (*article.Article, error)
	FetchUserLatestArticles(ctx context.Context, username string, limit int) ([]article.Article, error)
}

// AnalyzeRequest contains the source selection for author style analysis.
type AnalyzeRequest struct {
	Username    string
	ArticleURLs []string
	Limit       int
	FetchedAt   time.Time
}

// ApplyDefaults fills optional request values.
func (r *AnalyzeRequest) ApplyDefaults() {
	if r.Limit <= 0 {
		r.Limit = defaultArticleLimit
	}
	if r.FetchedAt.IsZero() {
		r.FetchedAt = time.Now().UTC()
	}
}

// Validate checks that at least one source selector was supplied.
func (r AnalyzeRequest) Validate() error {
	if strings.TrimSpace(r.Username) == "" && len(trimmedStrings(r.ArticleURLs)) == 0 {
		return fmt.Errorf("username or article_urls is required")
	}
	if r.Limit <= 0 {
		return fmt.Errorf("limit must be positive")
	}
	return nil
}

// AnalyzeResult is the complete output of author style analysis.
type AnalyzeResult struct {
	ID           string                    `json:"id"`
	Source       author.AuthorSource       `json:"source"`
	Profile      author.AuthorStyleProfile `json:"profile"`
	Guide        author.WritingStyleGuide  `json:"guide"`
	ArticleCount int                       `json:"article_count"`
	CreatedAt    time.Time                 `json:"created_at"`
}

// AnalyzeAuthorStyleService coordinates article retrieval and guide construction.
type AnalyzeAuthorStyleService struct {
	fetcher SourceFetcher
	logger  *slog.Logger
}

// NewAnalyzeAuthorStyleService creates an author style application service.
func NewAnalyzeAuthorStyleService(fetcher SourceFetcher, logger *slog.Logger) *AnalyzeAuthorStyleService {
	if logger == nil {
		logger = slog.Default()
	}
	return &AnalyzeAuthorStyleService{
		fetcher: fetcher,
		logger:  logger,
	}
}

// Analyze fetches source articles and returns a durable profile plus writing guide.
func (s *AnalyzeAuthorStyleService) Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResult, error) {
	if s.fetcher == nil {
		return AnalyzeResult{}, fmt.Errorf("source fetcher is required")
	}
	req.ApplyDefaults()
	if err := req.Validate(); err != nil {
		return AnalyzeResult{}, err
	}

	articles, err := s.fetchArticles(ctx, req)
	if err != nil {
		return AnalyzeResult{}, err
	}
	articles = filterArticlesWithContent(articles)
	if len(articles) == 0 {
		return AnalyzeResult{}, fmt.Errorf("no source articles with content")
	}

	source := author.AuthorSource{
		Username:  strings.TrimSpace(req.Username),
		Articles:  author.SourceArticlesFromArticles(articles, req.FetchedAt),
		FetchedAt: req.FetchedAt,
	}
	profile, err := author.BuildAuthorStyleProfile(source, articles)
	if err != nil {
		return AnalyzeResult{}, fmt.Errorf("build author style profile: %w", err)
	}
	guide, err := author.BuildWritingStyleGuide(profile)
	if err != nil {
		return AnalyzeResult{}, fmt.Errorf("build writing style guide: %w", err)
	}

	result := AnalyzeResult{
		ID:           ResultID(source, profile.ID, guide.ID),
		Source:       source,
		Profile:      profile,
		Guide:        guide,
		ArticleCount: len(articles),
		CreatedAt:    req.FetchedAt,
	}
	s.logger.InfoContext(ctx, "analyzed author style", "result_id", result.ID, "article_count", result.ArticleCount)
	return result, nil
}

// ResultID returns a stable ID for an analysis result.
func ResultID(source author.AuthorSource, profileID, guideID string) string {
	parts := []string{strings.TrimSpace(source.Username), profileID, guideID}
	for _, article := range source.Articles {
		parts = append(parts, strings.TrimSpace(article.ID), strings.TrimSpace(article.URL))
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return "asr_" + hex.EncodeToString(sum[:])[:12]
}

func (s *AnalyzeAuthorStyleService) fetchArticles(ctx context.Context, req AnalyzeRequest) ([]article.Article, error) {
	if strings.TrimSpace(req.Username) != "" {
		articles, err := s.fetcher.FetchUserLatestArticles(ctx, strings.TrimSpace(req.Username), req.Limit)
		if err != nil {
			return nil, fmt.Errorf("fetch latest author articles: %w", err)
		}
		return articles, nil
	}

	urls := trimmedStrings(req.ArticleURLs)
	articles := make([]article.Article, 0, len(urls))
	for _, articleURL := range urls {
		fetched, err := s.fetcher.FetchArticle(ctx, articleURL)
		if err != nil {
			return nil, fmt.Errorf("fetch author article %q: %w", articleURL, err)
		}
		if fetched != nil {
			articles = append(articles, *fetched)
		}
	}
	return articles, nil
}

func filterArticlesWithContent(articles []article.Article) []article.Article {
	filtered := make([]article.Article, 0, len(articles))
	for _, fetched := range articles {
		if strings.TrimSpace(fetched.Content) != "" {
			filtered = append(filtered, fetched)
		}
	}
	return filtered
}

func trimmedStrings(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}
