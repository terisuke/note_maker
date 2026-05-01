package article

import (
	"fmt"
	"strings"
)

// Article is source material collected from note.com or another public source.
type Article struct {
	URL     string
	Title   string
	Content string
}

// GenerationRequest contains the normalized article generation inputs.
type GenerationRequest struct {
	NoteURL               string
	Username              string
	Keywords              []string
	Theme                 string
	TargetAudience        string
	Exclusions            string
	StyleChoice           string
	ToneChoice            string
	WordCount             int
	ReferenceArticleLimit int
	ArticlePurpose        string
	DesiredContent        string
	IntroductionPoints    string
	MainPoints            string
	ConclusionMessage     string
}

// ApplyDefaults fills optional request fields with the public API defaults.
func (r *GenerationRequest) ApplyDefaults() {
	if r.StyleChoice == "" {
		r.StyleChoice = "ですます調"
	}
	if r.ToneChoice == "" {
		r.ToneChoice = "客観的"
	}
	if r.WordCount <= 0 {
		r.WordCount = 1500
	}
	if r.ReferenceArticleLimit <= 0 {
		r.ReferenceArticleLimit = 3
	}
	if r.ReferenceArticleLimit > 10 {
		r.ReferenceArticleLimit = 10
	}
}

// Validate checks the public API contract for article generation.
func (r GenerationRequest) Validate() error {
	if strings.TrimSpace(r.NoteURL) == "" && strings.TrimSpace(r.Username) == "" {
		return fmt.Errorf("note_url or username is required")
	}
	if strings.TrimSpace(r.Theme) == "" {
		return fmt.Errorf("theme is required")
	}
	return nil
}
