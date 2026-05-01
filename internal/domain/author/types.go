package author

import (
	"fmt"
	"strings"
	"time"

	"github.com/teradakousuke/note_maker/internal/domain/article"
)

const defaultProfileVersion = "author-style/v1"

// SourceArticle identifies one fetched article that contributed to an author profile.
type SourceArticle struct {
	ID    string    `json:"id"`
	URL   string    `json:"url"`
	Title string    `json:"title"`
	At    time.Time `json:"fetched_at"`
}

// AuthorSource describes where the profile input articles came from.
type AuthorSource struct {
	Username  string          `json:"username,omitempty"`
	Articles  []SourceArticle `json:"articles"`
	FetchedAt time.Time       `json:"fetched_at"`
}

// Validate checks that the source has enough provenance for later review.
func (s AuthorSource) Validate() error {
	if len(s.Articles) == 0 {
		return fmt.Errorf("author source requires at least one article")
	}
	for i, sourceArticle := range s.Articles {
		if strings.TrimSpace(sourceArticle.URL) == "" && strings.TrimSpace(sourceArticle.ID) == "" {
			return fmt.Errorf("author source article %d requires url or id", i)
		}
	}
	return nil
}

// AuthorStyleProfile is a durable, author-level style asset derived from source articles.
type AuthorStyleProfile struct {
	ID                   string               `json:"id"`
	Version              string               `json:"version"`
	Source               AuthorSource         `json:"source"`
	StyleProfile         article.StyleProfile `json:"style_profile"`
	Metrics              article.StyleProfile `json:"metrics"`
	ArticleCount         int                  `json:"article_count"`
	PreferredFirstPerson string               `json:"preferred_first_person"`
	RecurringKeywords    []string             `json:"recurring_keywords"`
	Warnings             []string             `json:"warnings,omitempty"`
}

// Validate checks whether the profile can safely be used to build a guide.
func (p AuthorStyleProfile) Validate() error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("author style profile id is required")
	}
	if p.Version == "" {
		return fmt.Errorf("author style profile version is required")
	}
	if p.ArticleCount <= 0 {
		return fmt.Errorf("author style profile requires at least one article")
	}
	if err := p.Source.Validate(); err != nil {
		return err
	}
	if p.Metrics.CharCount <= 0 && p.StyleProfile.CharCount <= 0 {
		return fmt.Errorf("author style profile requires non-empty metrics")
	}
	return nil
}

// WritingStyleGuide is compact guidance used by later draft generation.
type WritingStyleGuide struct {
	ID                   string   `json:"id"`
	ProfileID            string   `json:"profile_id"`
	Markdown             string   `json:"markdown"`
	PreferredFirstPerson string   `json:"preferred_first_person"`
	RecurringThemes      []string `json:"recurring_themes"`
	ParagraphRhythm      string   `json:"paragraph_rhythm"`
	SentenceRhythm       string   `json:"sentence_rhythm"`
	HeadingGuidance      string   `json:"heading_guidance"`
	QuoteGuidance        string   `json:"quote_guidance"`
	OpeningPatterns      []string `json:"opening_patterns"`
	ConclusionPatterns   []string `json:"conclusion_patterns"`
	Warnings             []string `json:"warnings,omitempty"`
}

// Validate checks whether the guide has the minimum guidance needed for draft generation.
func (g WritingStyleGuide) Validate() error {
	if strings.TrimSpace(g.ID) == "" {
		return fmt.Errorf("writing style guide id is required")
	}
	if strings.TrimSpace(g.ProfileID) == "" {
		return fmt.Errorf("writing style guide profile id is required")
	}
	if strings.TrimSpace(g.PreferredFirstPerson) == "" {
		return fmt.Errorf("writing style guide preferred first person is required")
	}
	if len(g.RecurringThemes) == 0 {
		return fmt.Errorf("writing style guide requires recurring themes")
	}
	if strings.TrimSpace(g.ParagraphRhythm) == "" {
		return fmt.Errorf("writing style guide paragraph rhythm is required")
	}
	if strings.TrimSpace(g.SentenceRhythm) == "" {
		return fmt.Errorf("writing style guide sentence rhythm is required")
	}
	if strings.TrimSpace(g.HeadingGuidance) == "" {
		return fmt.Errorf("writing style guide heading guidance is required")
	}
	if strings.TrimSpace(g.QuoteGuidance) == "" {
		return fmt.Errorf("writing style guide quote guidance is required")
	}
	if len(g.OpeningPatterns) == 0 {
		return fmt.Errorf("writing style guide requires opening patterns")
	}
	if len(g.ConclusionPatterns) == 0 {
		return fmt.Errorf("writing style guide requires conclusion patterns")
	}
	return nil
}
