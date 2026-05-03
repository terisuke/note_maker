package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
)

const defaultOutputDir = "tmp/draft_generation"

func main() {
	if os.Getenv("RUN_LOCAL_LLM_SCENARIO") != "1" {
		fatalf("set RUN_LOCAL_LLM_SCENARIO=1 to run local LLM draft scenario")
	}

	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	profilePath := envOrDefault("AUTHOR_PROFILE_PATH", "tmp/author_style/profile.json")
	guidePath := envOrDefault("WRITING_GUIDE_PATH", "tmp/author_style/guide.json")
	briefPath := envOrDefault("ARTICLE_BRIEF_PATH", "tmp/brief_interview/brief.json")

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	var profile authordomain.AuthorStyleProfile
	var guide authordomain.WritingStyleGuide
	var brief briefdomain.ArticleBrief
	readJSON(profilePath, &profile)
	readJSON(guidePath, &guide)
	readJSON(briefPath, &brief)
	if err := validateScenarioInputs(profile, guide, brief); err != nil {
		fatalf("invalid scenario inputs: %v", err)
	}

	baseURL := envFirst("http://127.0.0.1:8081/v1", "LLM_BASE_URL", "LLAMACPP_BASE_URL")
	model := envFirst("gemma4:31b", "DRAFT_LLM_MODEL", "LLM_MODEL", "LLAMACPP_MODEL")
	verifyModel := envFirst("gemma4:latest", "VERIFY_LLM_MODEL", "LLM_MODEL", "LLAMACPP_MODEL")
	failureContext := failureAttemptContext{
		LLMBaseURL:     baseURL,
		LLMModel:       model,
		VerifyModel:    verifyModel,
		PersonaID:      brief.PersonaID,
		OutputFormatID: brief.OutputFormatID,
	}
	minStyleScore := envFloat("SCENARIO_MIN_STYLE_SCORE", 80)
	minDraftRunes := envInt("SCENARIO_MIN_DRAFT_RUNES", 2400)
	maxAttempts := envInt("DRAFT_MAX_ATTEMPTS", 2)
	timeout := scenarioTimeout()
	streamDraft := os.Getenv("SCENARIO_STREAM_DRAFT") == "1"
	client, err := llamacpp.NewClientFromEnvForPurpose("DRAFT")
	if err != nil {
		fatalf("create local llm client: %v", err)
	}
	verifyClient, err := llamacpp.NewClientFromEnvForPurpose("VERIFY")
	if err != nil {
		fatalf("create verification llm client: %v", err)
	}
	service := draftapp.NewServiceWithVerifier(client, draftapp.NewLightweightVerifier(verifyClient))

	var result draftapp.GenerateResult
	var finalElapsed time.Duration
	var finalFirstChunk time.Duration
	var finalChunks int
	finalAttempt := 0
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		started := time.Now()
		request := draftapp.GenerateRequest{
			StyleGuide:    guide,
			Brief:         brief,
			AuthorProfile: profile,
		}
		chunkCount := 0
		var firstChunk time.Duration
		if streamDraft {
			result, err = service.GenerateStream(ctx, request, draftapp.StreamEvents{
				OnChunk: func(chunk string) error {
					chunkCount++
					if firstChunk == 0 {
						firstChunk = time.Since(started)
					}
					return nil
				},
			})
		} else {
			result, err = service.Generate(ctx, request)
		}
		elapsed := time.Since(started)
		cancel()
		metrics := attemptRuntimeMetrics{
			ElapsedSeconds: elapsed.Seconds(),
			TimeoutSeconds: timeout.Seconds(),
			Streaming:      streamDraft,
			FirstChunkMs:   finalFirstChunkMs(firstChunk),
			Chunks:         chunkCount,
		}
		if err != nil {
			attempts := generationAttemptsFromError(err)
			artifacts := writeRawAttemptArtifacts(outputDir, attempt, attempts)
			failurePath := writeFailureAttempt(outputDir, attempt, err, metrics, failureContext, artifacts)
			fatalf("generate draft attempt %d: %v (failure=%s)", attempt, err, failurePath)
		}
		finalElapsed = elapsed
		finalFirstChunk = firstChunk
		finalChunks = chunkCount
		finalAttempt = attempt
		writeRawAttemptArtifacts(outputDir, attempt, result.Attempts)
		writeFile(filepath.Join(outputDir, fmt.Sprintf("draft_attempt_%d.md", attempt)), result.Draft.Markdown()+"\n")
		writeJSON(filepath.Join(outputDir, fmt.Sprintf("evaluation_attempt_%d.json", attempt)), result.Evaluation)
		writeJSON(filepath.Join(outputDir, fmt.Sprintf("verification_attempt_%d.json", attempt)), result.Verification)
		if result.Evaluation.Comparison.Score >= minStyleScore && len([]rune(result.Draft.Markdown())) >= minDraftRunes && verificationGatePassed(result.Verification) {
			break
		}
	}

	writeFile(filepath.Join(outputDir, "draft.md"), result.Draft.Markdown()+"\n")
	writeJSON(filepath.Join(outputDir, "evaluation.json"), result.Evaluation)
	writeJSON(filepath.Join(outputDir, "verification.json"), result.Verification)

	runes := len([]rune(result.Draft.Markdown()))
	passesScenario := result.Evaluation.Comparison.Score >= minStyleScore && runes >= minDraftRunes && verificationGatePassed(result.Verification)
	fmt.Printf("draft generation scenario completed\n")
	fmt.Printf("scenario_passed=%v\n", passesScenario)
	fmt.Printf("attempt=%d\n", finalAttempt)
	fmt.Printf("passed=%v\n", result.Evaluation.Passed)
	fmt.Printf("score=%.1f\n", result.Evaluation.Comparison.Score)
	fmt.Printf("min_style_score=%.1f\n", minStyleScore)
	fmt.Printf("runes=%d\n", runes)
	fmt.Printf("min_draft_runes=%d\n", minDraftRunes)
	fmt.Printf("verification_performed=%v\n", result.Verification.Performed)
	fmt.Printf("verification_passed=%v\n", result.Verification.Passed)
	fmt.Printf("verification_summary=%s\n", result.Verification.Summary)
	fmt.Printf("elapsed_seconds=%.2f\n", finalElapsed.Seconds())
	fmt.Printf("streaming=%v\n", streamDraft)
	if streamDraft {
		fmt.Printf("first_chunk_ms=%d\n", finalFirstChunk.Milliseconds())
		fmt.Printf("chunks=%d\n", finalChunks)
	}
	fmt.Printf("llm_base_url=%s\n", baseURL)
	fmt.Printf("llm_model=%s\n", model)
	fmt.Printf("verify_model=%s\n", verifyModel)
	fmt.Printf("draft=%s\n", filepath.Join(outputDir, "draft.md"))
	fmt.Printf("evaluation=%s\n", filepath.Join(outputDir, "evaluation.json"))
	fmt.Printf("verification=%s\n", filepath.Join(outputDir, "verification.json"))
	if result.Evaluation.Comparison.Score < minStyleScore {
		fatalf("style score %.1f below scenario minimum %.1f", result.Evaluation.Comparison.Score, minStyleScore)
	}
	if runes < minDraftRunes {
		fatalf("draft length %d below scenario minimum %d", runes, minDraftRunes)
	}
	if result.Verification.Performed && !result.Verification.Passed {
		fatalf("final verification failed: %s", result.Verification.Summary)
	}
}

type attemptRuntimeMetrics struct {
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	TimeoutSeconds float64 `json:"timeout_seconds"`
	Streaming      bool    `json:"streaming"`
	FirstChunkMs   int64   `json:"first_chunk_ms,omitempty"`
	Chunks         int     `json:"chunks,omitempty"`
}

type rawAttemptArtifact struct {
	GenerationAttempt int    `json:"generation_attempt"`
	Kind              string `json:"kind"`
	Path              string `json:"path"`
	ValidationError   string `json:"validation_error,omitempty"`
}

type failureAttemptContext struct {
	LLMBaseURL     string `json:"llm_base_url"`
	LLMModel       string `json:"llm_model"`
	VerifyModel    string `json:"verify_model"`
	PersonaID      string `json:"persona_id"`
	OutputFormatID string `json:"output_format_id"`
}

type failureAttemptReport struct {
	Attempt         int                   `json:"attempt"`
	Error           string                `json:"error"`
	ValidationError string                `json:"validation_error,omitempty"`
	RuntimeMetrics  attemptRuntimeMetrics `json:"runtime_metrics"`
	Context         failureAttemptContext `json:"context"`
	RawOutputs      []rawAttemptArtifact  `json:"raw_outputs"`
}

func generationAttemptsFromError(err error) []draftapp.GenerationAttempt {
	var unusable *draftapp.UnusableDraftError
	if errors.As(err, &unusable) {
		return unusable.Attempts
	}
	return nil
}

func writeRawAttemptArtifacts(outputDir string, scenarioAttempt int, attempts []draftapp.GenerationAttempt) []rawAttemptArtifact {
	artifacts := make([]rawAttemptArtifact, 0, len(attempts))
	for _, attempt := range attempts {
		if strings.TrimSpace(attempt.RawOutput) == "" {
			continue
		}
		kind := sanitizeArtifactPart(attempt.Kind)
		if kind == "" {
			kind = "generation"
		}
		index := attempt.Index
		if index <= 0 {
			index = len(artifacts) + 1
		}
		path := filepath.Join(outputDir, fmt.Sprintf("raw_attempt_%d_generation_%d_%s.txt", scenarioAttempt, index, kind))
		writeFile(path, strings.TrimRight(attempt.RawOutput, "\n")+"\n")
		artifacts = append(artifacts, rawAttemptArtifact{
			GenerationAttempt: index,
			Kind:              attempt.Kind,
			Path:              path,
			ValidationError:   attempt.ValidationError,
		})
	}
	return artifacts
}

func writeFailureAttempt(outputDir string, attempt int, err error, metrics attemptRuntimeMetrics, context failureAttemptContext, artifacts []rawAttemptArtifact) string {
	report := failureAttemptReport{
		Attempt:         attempt,
		Error:           err.Error(),
		ValidationError: validationErrorFromGenerateError(err),
		RuntimeMetrics:  metrics,
		Context:         context,
		RawOutputs:      artifacts,
	}
	path := filepath.Join(outputDir, fmt.Sprintf("failure_attempt_%d.json", attempt))
	writeJSON(path, report)
	return path
}

func validationErrorFromGenerateError(err error) string {
	var unusable *draftapp.UnusableDraftError
	if errors.As(err, &unusable) && unusable.Err != nil {
		return unusable.Err.Error()
	}
	return ""
}

func verificationGatePassed(verification draftapp.FinalVerification) bool {
	return !verification.Performed || verification.Passed
}

func validateScenarioInputs(profile authordomain.AuthorStyleProfile, guide authordomain.WritingStyleGuide, brief briefdomain.ArticleBrief) error {
	if strings.TrimSpace(guide.ProfileID) != strings.TrimSpace(profile.ID) {
		return fmt.Errorf("writing guide profile id %q does not match author profile id %q", guide.ProfileID, profile.ID)
	}
	if strings.TrimSpace(brief.StyleProfileID) != "" && strings.TrimSpace(brief.StyleProfileID) != strings.TrimSpace(profile.ID) {
		return fmt.Errorf("article brief style profile id %q does not match author profile id %q", brief.StyleProfileID, profile.ID)
	}
	return nil
}

func finalFirstChunkMs(firstChunk time.Duration) int64 {
	if firstChunk <= 0 {
		return 0
	}
	return firstChunk.Milliseconds()
}

func sanitizeArtifactPart(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	return strings.Trim(builder.String(), "_")
}

func readJSON(path string, out any) {
	encoded, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(encoded, out); err != nil {
		fatalf("decode %s: %v", path, err)
	}
}

func writeJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", path, err)
	}
	writeFile(path, string(encoded)+"\n")
}

func writeFile(path, content string) {
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fatalf("write %s: %v", path, err)
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envFirst(fallback string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func scenarioTimeout() time.Duration {
	seconds := envInt("SCENARIO_DRAFT_TIMEOUT_SECONDS", 0)
	if seconds == 0 {
		seconds = envInt("LLM_TIMEOUT_SECONDS", 720)
	}
	return time.Duration(seconds) * time.Second
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
