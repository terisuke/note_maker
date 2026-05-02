package draft

import (
	"context"
	"fmt"
	"strings"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

// TextGenerator generates a draft from a prompt.
type TextGenerator interface {
	Generate(ctx context.Context, prompt string) (string, error)
}

// Service coordinates prompt building, generation, Markdown validation, and style evaluation.
type Service struct {
	generator TextGenerator
}

// NewService creates a draft generation service.
func NewService(generator TextGenerator) *Service {
	return &Service{generator: generator}
}

// Generate builds a prompt from the style guide and brief, validates the generated Markdown,
// and returns the draft with strict style evaluation.
func (s *Service) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	if s.generator == nil {
		return GenerateResult{}, fmt.Errorf("text generator is required")
	}
	if err := validateRequest(req); err != nil {
		return GenerateResult{}, err
	}

	persona := req.Persona
	if persona.ID == "" {
		persona, _ = personadomain.DefaultRegistry().Get(req.Brief.PersonaID)
	}
	format := req.OutputFormat
	if format.ID == "" {
		var ok bool
		format, ok = outputformat.DefaultRegistry().Get(req.Brief.OutputFormatID)
		if !ok {
			format, _ = outputformat.DefaultRegistry().Get(outputformat.IDNoteArticle)
		}
	}

	prompt := BuildPromptForMode(req.StyleGuide, req.Brief, persona, format)
	rawDraft, err := s.generator.Generate(ctx, prompt)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate draft with local llm: %w", err)
	}
	articleDraft, err := articledomain.NewDraftForFormat(rawDraft, format.ID)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("local llm returned an unusable draft: %w", err)
	}

	return GenerateResult{
		Draft:      articleDraft,
		Evaluation: EvaluateStyle(req.AuthorProfile, req.Brief, articleDraft),
	}, nil
}

func validateRequest(req GenerateRequest) error {
	if err := req.StyleGuide.Validate(); err != nil {
		return fmt.Errorf("writing style guide is invalid: %w", err)
	}
	if strings.TrimSpace(req.Brief.Theme) == "" {
		return fmt.Errorf("article brief theme is required")
	}
	if err := req.AuthorProfile.Validate(); err != nil {
		return fmt.Errorf("author style profile is invalid: %w", err)
	}
	return nil
}
