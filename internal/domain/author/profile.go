package author

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/teradakousuke/note_maker/internal/domain/article"
)

const maxRecurringKeywords = 10

// BuildAuthorStyleProfile derives an author profile from fetched articles and source metadata.
func BuildAuthorStyleProfile(source AuthorSource, articles []article.Article) (AuthorStyleProfile, error) {
	articles = articlesWithContent(articles)
	if len(articles) == 0 {
		return AuthorStyleProfile{}, fmt.Errorf("author style profile requires at least one article with content")
	}
	if len(source.Articles) == 0 {
		source.Articles = SourceArticlesFromArticles(articles, source.FetchedAt)
	}
	if source.FetchedAt.IsZero() {
		source.FetchedAt = firstFetchedAt(source.Articles)
	}
	if err := source.Validate(); err != nil {
		return AuthorStyleProfile{}, err
	}

	metrics := article.AnalyzeStyleCorpus(articles)
	profile := AuthorStyleProfile{
		ID:                   ProfileID(source),
		Version:              defaultProfileVersion,
		Source:               source,
		StyleProfile:         metrics,
		Metrics:              metrics,
		ArticleCount:         len(articles),
		PreferredFirstPerson: preferredFirstPerson(metrics),
		RecurringKeywords:    recurringKeywords(metrics.KeywordCounts, maxRecurringKeywords),
		Warnings:             profileWarnings(metrics, len(articles)),
	}
	if err := profile.Validate(); err != nil {
		return AuthorStyleProfile{}, err
	}
	return profile, nil
}

// SourceArticlesFromArticles builds source metadata from fetched articles.
func SourceArticlesFromArticles(articles []article.Article, fetchedAt time.Time) []SourceArticle {
	sources := make([]SourceArticle, 0, len(articles))
	for _, fetched := range articles {
		if strings.TrimSpace(fetched.Content) == "" {
			continue
		}
		sourceURL := strings.TrimSpace(fetched.URL)
		sources = append(sources, SourceArticle{
			ID:    ArticleIDFromURL(sourceURL),
			URL:   sourceURL,
			Title: strings.TrimSpace(fetched.Title),
			At:    fetchedAt,
		})
	}
	return sources
}

// ProfileID returns a stable ID for source metadata.
func ProfileID(source AuthorSource) string {
	parts := []string{strings.TrimSpace(source.Username)}
	for _, article := range source.Articles {
		parts = append(parts, strings.TrimSpace(article.ID), strings.TrimSpace(article.URL))
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return "asp_" + hex.EncodeToString(sum[:])[:12]
}

// GuideID returns a stable guide ID for a profile.
func GuideID(profile AuthorStyleProfile) string {
	sum := sha1.Sum([]byte(profile.ID + "|" + strings.Join(profile.RecurringKeywords, "|")))
	return "wsg_" + hex.EncodeToString(sum[:])[:12]
}

// ArticleIDFromURL extracts a durable article-ish identifier from a URL.
func ArticleIDFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	segments := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		if segment := strings.TrimSpace(segments[i]); segment != "" {
			return segment
		}
	}
	return ""
}

func articlesWithContent(articles []article.Article) []article.Article {
	filtered := make([]article.Article, 0, len(articles))
	for _, fetched := range articles {
		if strings.TrimSpace(fetched.Content) != "" {
			filtered = append(filtered, fetched)
		}
	}
	return filtered
}

func firstFetchedAt(articles []SourceArticle) time.Time {
	for _, article := range articles {
		if !article.At.IsZero() {
			return article.At
		}
	}
	return time.Time{}
}

func preferredFirstPerson(metrics article.StyleProfile) string {
	counts := map[string]int{
		"僕": metrics.KeywordCounts["僕"],
		"私": metrics.KeywordCounts["私"],
	}
	if counts["僕"] == 0 && counts["私"] == 0 && metrics.FirstPersonCount > 0 {
		return "一人称あり"
	}
	if counts["僕"] >= counts["私"] && counts["僕"] > 0 {
		return "僕"
	}
	if counts["私"] > 0 {
		return "私"
	}
	return "明示しない"
}

func recurringKeywords(counts map[string]int, limit int) []string {
	type keywordCount struct {
		keyword string
		count   int
	}
	ranked := make([]keywordCount, 0, len(counts))
	for keyword, count := range counts {
		if count > 0 {
			ranked = append(ranked, keywordCount{keyword: keyword, count: count})
		}
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count == ranked[j].count {
			return ranked[i].keyword < ranked[j].keyword
		}
		return ranked[i].count > ranked[j].count
	})
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	keywords := make([]string, 0, len(ranked))
	for _, item := range ranked {
		keywords = append(keywords, item.keyword)
	}
	if len(keywords) == 0 {
		return []string{"体験", "学び"}
	}
	return keywords
}

func profileWarnings(metrics article.StyleProfile, articleCount int) []string {
	var warnings []string
	if articleCount < 3 {
		warnings = append(warnings, "source_article_count_low")
	}
	if metrics.CharCount < 1500 {
		warnings = append(warnings, "source_corpus_short")
	}
	if metrics.SentenceCount == 0 {
		warnings = append(warnings, "source_has_no_sentences")
	}
	return warnings
}
