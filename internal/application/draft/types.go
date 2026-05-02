package draft

import (
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

// WritingStyleGuide is the compact author style guide used for draft generation.
type WritingStyleGuide = authordomain.WritingStyleGuide

// AuthorStyleProfile is the reference author style profile used for strict evaluation.
type AuthorStyleProfile = authordomain.AuthorStyleProfile

// ArticleBrief is the completed interview output used for draft generation.
type ArticleBrief = briefdomain.ArticleBrief

// BriefAnswer is a deep-dive answer carried by an ArticleBrief.
type BriefAnswer = briefdomain.BriefAnswer

// GenerateRequest contains the completed style and brief inputs for draft generation.
type GenerateRequest struct {
	StyleGuide    WritingStyleGuide
	Brief         ArticleBrief
	AuthorProfile AuthorStyleProfile
	Persona       personadomain.Persona
	OutputFormat  outputformat.OutputFormat
}

// GenerateResult returns the validated draft and its strict style evaluation.
type GenerateResult struct {
	Draft        articledomain.Draft
	Evaluation   StyleEvaluation
	Verification FinalVerification
}

// FinalVerification reports the lightweight model's final consistency review.
type FinalVerification struct {
	Performed bool     `json:"performed"`
	Passed    bool     `json:"passed"`
	Summary   string   `json:"summary"`
	Report    string   `json:"report"`
	Failures  []string `json:"failures,omitempty"`
}

// StyleThresholds are the strict draft acceptance thresholds from the implementation plan.
type StyleThresholds struct {
	TotalScore      float64
	ParagraphLength int
	SentenceLength  int
	KeywordOverlap  int
	QuoteDensity    int
	FirstPerson     int
}

// StrictThresholds are the milestone-4 acceptance thresholds for draft generation.
var StrictThresholds = StyleThresholds{
	TotalScore:      82,
	ParagraphLength: 75,
	SentenceLength:  75,
	KeywordOverlap:  70,
	QuoteDensity:    55,
	FirstPerson:     60,
}

// StyleEvaluation reports whether the generated draft meets strict local style checks.
type StyleEvaluation struct {
	Passed              bool
	Comparison          articledomain.StyleComparison
	Thresholds          StyleThresholds
	Failures            []string
	RequiredFirstPerson string
}
