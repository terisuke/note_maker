package article

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	domain "github.com/teradakousuke/note_maker/internal/domain/article"
)

// SourceFetcher loads reference articles for generation.
type SourceFetcher interface {
	FetchArticle(ctx context.Context, articleURL string) (*domain.Article, error)
	FetchUserLatestArticles(ctx context.Context, username string, limit int) ([]domain.Article, error)
}

// TextGenerator generates article drafts from prompts.
type TextGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Service coordinates article retrieval, prompt building, and local LLM generation.
type Service struct {
	fetcher   SourceFetcher
	generator TextGenerator
	logger    *slog.Logger
}

// NewService creates an article generation application service.
func NewService(fetcher SourceFetcher, generator TextGenerator, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		fetcher:   fetcher,
		generator: generator,
		logger:    logger,
	}
}

// GenerateArticle generates a Markdown draft from the request.
func (s *Service) GenerateArticle(ctx context.Context, req domain.GenerationRequest) (string, error) {
	req.ApplyDefaults()
	if err := req.Validate(); err != nil {
		return "", err
	}

	references, err := s.loadReferenceArticles(ctx, req)
	if err != nil {
		return "", err
	}
	if len(references) == 0 {
		s.logger.WarnContext(ctx, "no reference articles found; generating from user instructions only")
	}

	prompt := BuildPrompt(req, references)
	draft, err := s.generator.Generate(ctx, prompt)
	if err != nil {
		return "", fmt.Errorf("generate draft with local llm: %w", err)
	}
	finalDraft, err := domain.NewDraft(draft)
	if err != nil {
		return "", fmt.Errorf("local llm returned an unusable draft: %w", err)
	}
	return finalDraft.Markdown(), nil
}

func (s *Service) loadReferenceArticles(ctx context.Context, req domain.GenerationRequest) ([]domain.Article, error) {
	if strings.TrimSpace(req.Username) != "" {
		articles, err := s.fetcher.FetchUserLatestArticles(ctx, req.Username, req.ReferenceArticleLimit)
		if err != nil {
			return nil, fmt.Errorf("fetch user articles: %w", err)
		}
		return filterArticlesWithContent(articles), nil
	}

	article, err := s.fetcher.FetchArticle(ctx, req.NoteURL)
	if err != nil {
		return nil, fmt.Errorf("fetch article: %w", err)
	}
	if article == nil || strings.TrimSpace(article.Content) == "" {
		return nil, nil
	}
	return []domain.Article{*article}, nil
}

func filterArticlesWithContent(articles []domain.Article) []domain.Article {
	filtered := make([]domain.Article, 0, len(articles))
	for _, article := range articles {
		if strings.TrimSpace(article.Content) != "" {
			filtered = append(filtered, article)
		}
	}
	return filtered
}
