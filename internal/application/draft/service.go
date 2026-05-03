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

// StreamingTextGenerator can stream a draft while returning the final assembled text.
type StreamingTextGenerator interface {
	GenerateStream(ctx context.Context, prompt string, onChunk func(string) error) (string, error)
}

// DraftVerifier checks the final draft with a separate lightweight model.
type DraftVerifier interface {
	VerifyDraft(ctx context.Context, req VerificationRequest) (FinalVerification, error)
}

// VerificationRequest contains all inputs needed for final consistency review.
type VerificationRequest struct {
	StyleGuide    WritingStyleGuide
	Brief         ArticleBrief
	AuthorProfile AuthorStyleProfile
	Persona       personadomain.Persona
	OutputFormat  outputformat.OutputFormat
	DraftMarkdown string
	Evaluation    StyleEvaluation
}

// StreamEvents receives long-running draft generation progress.
type StreamEvents struct {
	OnStatus func(string) error
	OnChunk  func(string) error
}

// Service coordinates prompt building, generation, Markdown validation, and style evaluation.
type Service struct {
	generator TextGenerator
	verifier  DraftVerifier
}

// UnusableDraftError reports a validation failure while preserving every raw generation attempt.
type UnusableDraftError struct {
	FormatID string
	Attempts []GenerationAttempt
	Err      error
}

func (e *UnusableDraftError) Error() string {
	if e == nil || e.Err == nil {
		return "local llm returned an unusable draft"
	}
	return "local llm returned an unusable draft: " + e.Err.Error()
}

func (e *UnusableDraftError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// NewService creates a draft generation service.
func NewService(generator TextGenerator) *Service {
	return &Service{generator: generator}
}

// NewServiceWithVerifier creates a draft service with a final lightweight verification step.
func NewServiceWithVerifier(generator TextGenerator, verifier DraftVerifier) *Service {
	return &Service{generator: generator, verifier: verifier}
}

// Generate builds a prompt from the style guide and brief, validates the generated Markdown,
// and returns the draft with strict style evaluation.
func (s *Service) Generate(ctx context.Context, req GenerateRequest) (GenerateResult, error) {
	return s.generate(ctx, req, StreamEvents{})
}

// GenerateStream generates a draft while streaming model deltas through events.
func (s *Service) GenerateStream(ctx context.Context, req GenerateRequest, events StreamEvents) (GenerateResult, error) {
	return s.generate(ctx, req, events)
}

func (s *Service) generate(ctx context.Context, req GenerateRequest, events StreamEvents) (GenerateResult, error) {
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

	prompt := BuildPromptForModeWithProfile(req.StyleGuide, req.Brief, req.AuthorProfile, persona, format)
	attempts := make([]GenerationAttempt, 0, 2)
	if err := emitStatus(events, "draft_generation_started"); err != nil {
		return GenerateResult{}, err
	}
	rawDraft, err := s.generateRaw(ctx, prompt, events.OnChunk)
	if err != nil {
		return GenerateResult{}, fmt.Errorf("generate draft with local llm: %w", err)
	}
	if err := emitStatus(events, "draft_validation_started"); err != nil {
		return GenerateResult{}, err
	}
	articleDraft, err := articledomain.NewDraftForFormat(rawDraft, format.ID)
	attempts = appendGenerationAttempt(attempts, "initial", rawDraft, err)
	if err != nil {
		if !isRecoverableFormatValidationError(format.ID, err) {
			return GenerateResult{}, &UnusableDraftError{FormatID: format.ID, Attempts: attempts, Err: err}
		}
		if err := emitStatus(events, "draft_format_repair_started"); err != nil {
			return GenerateResult{}, err
		}
		repairedRaw, repairErr := s.generator.Generate(ctx, BuildFormatRepairPrompt(format, rawDraft, err))
		if repairErr != nil {
			return GenerateResult{}, fmt.Errorf("repair draft format with local llm: %w", repairErr)
		}
		articleDraft, repairErr = articledomain.NewDraftForFormat(repairedRaw, format.ID)
		attempts = appendGenerationAttempt(attempts, "format_repair", repairedRaw, repairErr)
		if repairErr != nil {
			return GenerateResult{}, &UnusableDraftError{FormatID: format.ID, Attempts: attempts, Err: repairErr}
		}
	}
	evaluation := EvaluateStyle(req.AuthorProfile, req.Brief, articleDraft)
	if shouldReviseForStrictStyle(evaluation) {
		if err := emitStatus(events, "style_revision_started"); err != nil {
			return GenerateResult{}, err
		}
		revisedDraft, revisedEvaluation, revisionAttempt, ok := s.reviseOnce(ctx, prompt, articleDraft, evaluation, format.ID, req)
		if revisionAttempt.RawOutput != "" || revisionAttempt.ValidationError != "" {
			revisionAttempt.Index = len(attempts) + 1
			attempts = append(attempts, revisionAttempt)
		}
		if ok {
			articleDraft = revisedDraft
			evaluation = revisedEvaluation
		}
	}
	verification := s.verifyFinalDraft(ctx, VerificationRequest{
		StyleGuide:    req.StyleGuide,
		Brief:         req.Brief,
		AuthorProfile: req.AuthorProfile,
		Persona:       persona,
		OutputFormat:  format,
		DraftMarkdown: articleDraft.Markdown(),
		Evaluation:    evaluation,
	}, events)

	return GenerateResult{
		Draft:        articleDraft,
		Evaluation:   evaluation,
		Verification: verification,
		Attempts:     attempts,
	}, nil
}

func (s *Service) verifyFinalDraft(ctx context.Context, req VerificationRequest, events StreamEvents) FinalVerification {
	if s.verifier == nil {
		return FinalVerification{}
	}
	if err := emitStatus(events, "draft_lightweight_verification_started"); err != nil {
		return FinalVerification{Performed: true, Passed: false, Summary: "final verification was interrupted", Report: err.Error(), Failures: []string{err.Error()}}
	}
	verification, err := s.verifier.VerifyDraft(ctx, req)
	if err != nil {
		return FinalVerification{
			Performed: true,
			Passed:    false,
			Summary:   "final verification failed",
			Report:    err.Error(),
			Failures:  []string{err.Error()},
		}
	}
	verification.Performed = true
	return verification
}

func (s *Service) generateRaw(ctx context.Context, prompt string, onChunk func(string) error) (string, error) {
	if onChunk != nil {
		if streamingGenerator, ok := s.generator.(StreamingTextGenerator); ok {
			return streamingGenerator.GenerateStream(ctx, prompt, onChunk)
		}
	}
	return s.generator.Generate(ctx, prompt)
}

func (s *Service) reviseOnce(ctx context.Context, originalPrompt string, articleDraft articledomain.Draft, evaluation StyleEvaluation, formatID string, req GenerateRequest) (articledomain.Draft, StyleEvaluation, GenerationAttempt, bool) {
	revisionPrompt := BuildStyleRevisionPrompt(originalPrompt, articleDraft.Markdown(), evaluation)
	rawDraft, err := s.generator.Generate(ctx, revisionPrompt)
	if err != nil {
		return articledomain.Draft{}, StyleEvaluation{}, GenerationAttempt{}, false
	}
	revisedDraft, err := articledomain.NewDraftForFormat(rawDraft, formatID)
	if err != nil {
		return articledomain.Draft{}, StyleEvaluation{}, generationAttempt("style_revision", rawDraft, err), false
	}
	attempt := generationAttempt("style_revision", rawDraft, nil)
	revisedEvaluation := EvaluateStyle(req.AuthorProfile, req.Brief, revisedDraft)
	if revisedEvaluation.Passed || len(revisedEvaluation.Failures) <= len(evaluation.Failures) || revisedEvaluation.Comparison.Score >= evaluation.Comparison.Score {
		return revisedDraft, revisedEvaluation, attempt, true
	}
	return articledomain.Draft{}, StyleEvaluation{}, attempt, false
}

func emitStatus(events StreamEvents, status string) error {
	if events.OnStatus == nil {
		return nil
	}
	return events.OnStatus(status)
}

func shouldReviseForStrictStyle(evaluation StyleEvaluation) bool {
	if evaluation.Passed {
		return false
	}
	for _, failure := range evaluation.Failures {
		if strings.Contains(failure, "first_person") || strings.Contains(failure, "total_style_score") {
			return true
		}
	}
	return false
}

func appendGenerationAttempt(attempts []GenerationAttempt, kind, raw string, validationErr error) []GenerationAttempt {
	attempt := generationAttempt(kind, raw, validationErr)
	attempt.Index = len(attempts) + 1
	return append(attempts, attempt)
}

func generationAttempt(kind, raw string, validationErr error) GenerationAttempt {
	attempt := GenerationAttempt{
		Kind:      kind,
		RawOutput: raw,
	}
	if validationErr != nil {
		attempt.ValidationError = validationErr.Error()
	}
	return attempt
}

func isRecoverableFormatValidationError(formatID string, err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	if strings.Contains(message, "preamble before the article") {
		return true
	}
	switch formatID {
	case outputformat.IDZennArticle:
		return strings.Contains(message, "Qiita :::note")
	case outputformat.IDQiitaArticle:
		return strings.Contains(message, "Zenn-specific notation")
	default:
		return false
	}
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
