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
	var bestAttempt scenarioAttemptResult
	var currentAttempt int
	retryFeedback := ""
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		started := time.Now()
		attemptBrief := briefWithScenarioRetryFeedback(brief, retryFeedback)
		request := draftapp.GenerateRequest{
			StyleGuide:    guide,
			Brief:         attemptBrief,
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
		currentAttempt = attempt
		writeRawAttemptArtifacts(outputDir, attempt, result.Attempts)
		writeFile(filepath.Join(outputDir, fmt.Sprintf("draft_attempt_%d.md", attempt)), result.Draft.Markdown()+"\n")
		writeJSON(filepath.Join(outputDir, fmt.Sprintf("evaluation_attempt_%d.json", attempt)), result.Evaluation)
		writeJSON(filepath.Join(outputDir, fmt.Sprintf("verification_attempt_%d.json", attempt)), result.Verification)
		runes := len([]rune(result.Draft.Markdown()))
		candidate := scenarioAttemptResult{
			Attempt:         attempt,
			Result:          result,
			Runes:           runes,
			Elapsed:         elapsed,
			FirstChunk:      firstChunk,
			StreamingChunks: chunkCount,
		}
		if betterScenarioAttempt(candidate, bestAttempt, minDraftRunes, minStyleScore) {
			bestAttempt = candidate
		}
		if scenarioAttemptPassed(result, runes, minDraftRunes, minStyleScore) {
			break
		}
		retryFeedback = scenarioRetryFeedback(result, runes, minDraftRunes, minStyleScore)
	}
	if bestAttempt.Attempt == 0 {
		fatalf("no draft generation attempts completed")
	}

	result = bestAttempt.Result
	writeFile(filepath.Join(outputDir, "draft.md"), result.Draft.Markdown()+"\n")
	writeJSON(filepath.Join(outputDir, "evaluation.json"), result.Evaluation)
	writeJSON(filepath.Join(outputDir, "verification.json"), result.Verification)

	runes := bestAttempt.Runes
	passesScenario := scenarioAttemptPassed(result, runes, minDraftRunes, minStyleScore)
	fmt.Printf("draft generation scenario completed\n")
	fmt.Printf("scenario_passed=%v\n", passesScenario)
	fmt.Printf("attempt=%d\n", bestAttempt.Attempt)
	fmt.Printf("selected_attempt=%d\n", bestAttempt.Attempt)
	fmt.Printf("current_attempt=%d\n", currentAttempt)
	fmt.Printf("passed=%v\n", result.Evaluation.Passed)
	fmt.Printf("score=%.1f\n", result.Evaluation.Comparison.Score)
	fmt.Printf("min_style_score=%.1f\n", minStyleScore)
	fmt.Printf("runes=%d\n", runes)
	fmt.Printf("min_draft_runes=%d\n", minDraftRunes)
	fmt.Printf("verification_performed=%v\n", result.Verification.Performed)
	fmt.Printf("verification_passed=%v\n", result.Verification.Passed)
	fmt.Printf("verification_summary=%s\n", result.Verification.Summary)
	fmt.Printf("elapsed_seconds=%.2f\n", bestAttempt.Elapsed.Seconds())
	fmt.Printf("streaming=%v\n", streamDraft)
	if streamDraft {
		fmt.Printf("first_chunk_ms=%d\n", bestAttempt.FirstChunk.Milliseconds())
		fmt.Printf("chunks=%d\n", bestAttempt.StreamingChunks)
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

type scenarioAttemptResult struct {
	Attempt         int
	Result          draftapp.GenerateResult
	Runes           int
	Elapsed         time.Duration
	FirstChunk      time.Duration
	StreamingChunks int
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

func scenarioAttemptPassed(result draftapp.GenerateResult, runes, minRunes int, minStyleScore float64) bool {
	return result.Evaluation.Comparison.Score >= minStyleScore && runes >= minRunes && verificationGatePassed(result.Verification)
}

func betterScenarioAttempt(candidate, current scenarioAttemptResult, minRunes int, minStyleScore float64) bool {
	if candidate.Attempt == 0 {
		return false
	}
	if current.Attempt == 0 {
		return true
	}

	candidatePassed := scenarioAttemptPassed(candidate.Result, candidate.Runes, minRunes, minStyleScore)
	currentPassed := scenarioAttemptPassed(current.Result, current.Runes, minRunes, minStyleScore)
	if candidatePassed != currentPassed {
		return candidatePassed
	}

	candidateGateScore := scenarioAttemptGateScore(candidate.Result, candidate.Runes, minRunes, minStyleScore)
	currentGateScore := scenarioAttemptGateScore(current.Result, current.Runes, minRunes, minStyleScore)
	if candidateGateScore != currentGateScore {
		return candidateGateScore > currentGateScore
	}

	candidateStyleScore := candidate.Result.Evaluation.Comparison.Score
	currentStyleScore := current.Result.Evaluation.Comparison.Score
	if candidateStyleScore != currentStyleScore {
		return candidateStyleScore > currentStyleScore
	}
	if candidate.Runes != current.Runes {
		return candidate.Runes > current.Runes
	}
	return candidate.Attempt > current.Attempt
}

func scenarioAttemptGateScore(result draftapp.GenerateResult, runes, minRunes int, minStyleScore float64) int {
	score := 0
	if result.Evaluation.Comparison.Score >= minStyleScore {
		score++
	}
	if runes >= minRunes {
		score++
	}
	if verificationGatePassed(result.Verification) {
		score++
	}
	return score
}

func briefWithScenarioRetryFeedback(brief briefdomain.ArticleBrief, feedback string) briefdomain.ArticleBrief {
	feedback = strings.TrimSpace(feedback)
	if feedback == "" {
		return brief
	}
	updated := brief
	updated.TargetLengthStructure = appendSentence(updated.TargetLengthStructure, feedback)
	updated.MustInclude = appendSentence(updated.MustInclude, feedback)
	return updated
}

func scenarioRetryFeedback(result draftapp.GenerateResult, runes, minRunes int, minStyleScore float64) string {
	var feedback []string
	if runes < minRunes {
		feedback = append(feedback, fmt.Sprintf("前回の下書きは%d字で、最低%d字に届かなかった。次は各見出しを具体例・判断理由・読者の次の行動で厚くし、必ず%d字以上にする", runes, minRunes, minRunes))
	}
	if result.Evaluation.Comparison.Score < minStyleScore {
		feedback = append(feedback, fmt.Sprintf("前回の文体スコアは%.1fで、最低%.1fに届かなかった。文体ガイドの一人称、頻出テーマ、段落リズムを優先して全文を書き直す", result.Evaluation.Comparison.Score, minStyleScore))
	}
	if result.Verification.Performed && !result.Verification.Passed {
		summary := strings.TrimSpace(result.Verification.Summary)
		if summary == "" {
			summary = "軽量検証が不合格だった"
		}
		feedback = append(feedback, "前回の最終検証は不合格だった: "+summary+"。事実関係、論理のつながり、媒体の目的を見直す")
	}
	if len(feedback) == 0 {
		return ""
	}
	return "再生成条件: " + strings.Join(feedback, "。") + "。"
}

func appendSentence(base, addition string) string {
	base = strings.TrimSpace(base)
	addition = strings.TrimSpace(addition)
	if base == "" {
		return addition
	}
	if addition == "" || strings.Contains(base, addition) {
		return base
	}
	return base + "\n" + addition
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
