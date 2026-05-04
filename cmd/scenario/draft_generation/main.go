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
	minKeywordOverlap := envInt("SCENARIO_MIN_KEYWORD_OVERLAP", 0)
	minDraftRunes := envInt("SCENARIO_MIN_DRAFT_RUNES", 2400)
	maxFirstChunkMs := envInt("SCENARIO_MAX_FIRST_CHUNK_MS", 0)
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
	warmupEnabled := scenarioWarmupEnabled(streamDraft, maxFirstChunkMs)
	preflight := runScenarioPreflight(client, baseURL, model, warmupEnabled)
	writeJSON(filepath.Join(outputDir, "preflight.json"), preflight)
	if preflight.Enabled {
		fmt.Printf("preflight_enabled=true\n")
		fmt.Printf("preflight_passed=%v\n", preflight.Passed)
		fmt.Printf("preflight_elapsed_seconds=%.2f\n", preflight.ElapsedSeconds)
		fmt.Printf("preflight_models=%s\n", strings.Join(preflight.Models, ","))
		fmt.Printf("preflight_required_models=%s\n", strings.Join(preflight.RequiredModels, ","))
		if !preflight.Passed && preflight.Required {
			fatalf("draft preflight failed: %s", preflightFailure(preflight))
		}
	}
	warmup := runScenarioWarmup(client, warmupEnabled)
	writeJSON(filepath.Join(outputDir, "warmup.json"), warmup)
	if warmup.Enabled {
		fmt.Printf("warmup_enabled=true\n")
		fmt.Printf("warmup_passed=%v\n", warmup.Passed)
		fmt.Printf("warmup_first_chunk_ms=%d\n", warmup.FirstChunkMs)
		fmt.Printf("warmup_elapsed_seconds=%.2f\n", warmup.ElapsedSeconds)
		fmt.Printf("warmup_chunks=%d\n", warmup.Chunks)
		if !warmup.Passed && warmup.Required {
			fatalf("draft warmup failed: %s", warmup.Error)
		}
	}
	service := draftapp.NewService(client)
	if envBoolDefault("SCENARIO_VERIFY_DRAFT", true) {
		service = draftapp.NewServiceWithVerifier(client, draftapp.NewLightweightVerifier(verifyClient))
	}

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
		if betterScenarioAttempt(candidate, bestAttempt, minDraftRunes, minStyleScore, minKeywordOverlap) {
			bestAttempt = candidate
		}
		if scenarioAttemptPassed(result, runes, minDraftRunes, minStyleScore, minKeywordOverlap, maxFirstChunkMs, candidate.FirstChunk) {
			break
		}
		retryFeedback = scenarioRetryFeedback(result, runes, minDraftRunes, minStyleScore, minKeywordOverlap, maxFirstChunkMs, candidate.FirstChunk)
	}
	if bestAttempt.Attempt == 0 {
		fatalf("no draft generation attempts completed")
	}

	result = bestAttempt.Result
	writeFile(filepath.Join(outputDir, "draft.md"), result.Draft.Markdown()+"\n")
	writeJSON(filepath.Join(outputDir, "evaluation.json"), result.Evaluation)
	writeJSON(filepath.Join(outputDir, "verification.json"), result.Verification)

	runes := bestAttempt.Runes
	passesScenario := scenarioAttemptPassed(result, runes, minDraftRunes, minStyleScore, minKeywordOverlap, maxFirstChunkMs, bestAttempt.FirstChunk)
	fmt.Printf("draft generation scenario completed\n")
	fmt.Printf("scenario_passed=%v\n", passesScenario)
	fmt.Printf("attempt=%d\n", bestAttempt.Attempt)
	fmt.Printf("selected_attempt=%d\n", bestAttempt.Attempt)
	fmt.Printf("current_attempt=%d\n", currentAttempt)
	fmt.Printf("passed=%v\n", result.Evaluation.Passed)
	fmt.Printf("score=%.1f\n", result.Evaluation.Comparison.Score)
	fmt.Printf("min_style_score=%.1f\n", minStyleScore)
	fmt.Printf("keyword_overlap=%d\n", keywordOverlapScore(result))
	if minKeywordOverlap > 0 {
		fmt.Printf("min_keyword_overlap=%d\n", minKeywordOverlap)
	}
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
	if maxFirstChunkMs > 0 {
		fmt.Printf("max_first_chunk_ms=%d\n", maxFirstChunkMs)
	}
	fmt.Printf("llm_base_url=%s\n", baseURL)
	fmt.Printf("llm_model=%s\n", model)
	fmt.Printf("verify_model=%s\n", verifyModel)
	fmt.Printf("preflight=%s\n", filepath.Join(outputDir, "preflight.json"))
	fmt.Printf("warmup=%s\n", filepath.Join(outputDir, "warmup.json"))
	fmt.Printf("draft=%s\n", filepath.Join(outputDir, "draft.md"))
	fmt.Printf("evaluation=%s\n", filepath.Join(outputDir, "evaluation.json"))
	fmt.Printf("verification=%s\n", filepath.Join(outputDir, "verification.json"))
	if result.Evaluation.Comparison.Score < minStyleScore {
		fatalf("style score %.1f below scenario minimum %.1f", result.Evaluation.Comparison.Score, minStyleScore)
	}
	if minKeywordOverlap > 0 && keywordOverlapScore(result) < minKeywordOverlap {
		fatalf("keyword overlap %d below scenario minimum %d", keywordOverlapScore(result), minKeywordOverlap)
	}
	if runes < minDraftRunes {
		fatalf("draft length %d below scenario minimum %d", runes, minDraftRunes)
	}
	if maxFirstChunkMs > 0 && !firstChunkGatePassed(bestAttempt.FirstChunk, maxFirstChunkMs) {
		fatalf("first chunk %dms exceeded scenario maximum %dms", bestAttempt.FirstChunk.Milliseconds(), maxFirstChunkMs)
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

type scenarioPreflightReport struct {
	Enabled        bool     `json:"enabled"`
	Required       bool     `json:"required"`
	Passed         bool     `json:"passed"`
	Models         []string `json:"models,omitempty"`
	RequiredModels []string `json:"required_models,omitempty"`
	MissingModels  []string `json:"missing_models,omitempty"`
	ElapsedSeconds float64  `json:"elapsed_seconds,omitempty"`
	Error          string   `json:"error,omitempty"`
}

type scenarioWarmupReport struct {
	Enabled        bool    `json:"enabled"`
	Required       bool    `json:"required"`
	Passed         bool    `json:"passed"`
	FirstChunkMs   int64   `json:"first_chunk_ms,omitempty"`
	ElapsedSeconds float64 `json:"elapsed_seconds,omitempty"`
	Chunks         int     `json:"chunks,omitempty"`
	Error          string  `json:"error,omitempty"`
}

type scenarioAttemptResult struct {
	Attempt         int
	Result          draftapp.GenerateResult
	Runes           int
	Elapsed         time.Duration
	FirstChunk      time.Duration
	StreamingChunks int
}

func runScenarioPreflight(client *llamacpp.Client, baseURL, model string, warmupEnabled bool) scenarioPreflightReport {
	enabled := envBoolDefault("SCENARIO_DRAFT_PREFLIGHT", warmupEnabled)
	report := scenarioPreflightReport{
		Enabled:        enabled,
		Required:       envBoolDefault("SCENARIO_DRAFT_PREFLIGHT_REQUIRED", true),
		RequiredModels: scenarioPreflightRequiredModels(baseURL, model),
	}
	if !enabled {
		report.Passed = true
		return report
	}
	timeout := time.Duration(envInt("SCENARIO_DRAFT_PREFLIGHT_TIMEOUT_SECONDS", 30)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	started := time.Now()
	models, err := client.ListModels(ctx)
	report.ElapsedSeconds = time.Since(started).Seconds()
	if err != nil {
		report.Error = err.Error()
		return report
	}
	report.Models = models
	report.MissingModels = missingModels(models, report.RequiredModels)
	report.Passed = len(report.MissingModels) == 0
	if !report.Passed {
		report.Error = "missing models: " + strings.Join(report.MissingModels, ", ")
	}
	return report
}

func runScenarioWarmup(client *llamacpp.Client, enabled bool) scenarioWarmupReport {
	report := scenarioWarmupReport{
		Enabled:  enabled,
		Required: envBoolDefault("SCENARIO_DRAFT_WARMUP_REQUIRED", true),
	}
	if !enabled {
		report.Passed = true
		return report
	}
	timeout := time.Duration(envInt("SCENARIO_DRAFT_WARMUP_TIMEOUT_SECONDS", 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	started := time.Now()
	var firstChunk time.Duration
	chunkCount := 0
	_, err := client.Warmup(ctx, envOrDefault("SCENARIO_DRAFT_WARMUP_PROMPT", "短くokとだけ返してください。"), func(chunk string) error {
		chunkCount++
		if firstChunk == 0 {
			firstChunk = time.Since(started)
		}
		return nil
	})
	report.ElapsedSeconds = time.Since(started).Seconds()
	report.FirstChunkMs = finalFirstChunkMs(firstChunk)
	report.Chunks = chunkCount
	if err != nil {
		report.Error = err.Error()
		return report
	}
	report.Passed = firstChunk > 0 && chunkCount > 0
	if !report.Passed {
		report.Error = "warmup produced no stream chunks"
	}
	return report
}

func scenarioWarmupEnabled(streamDraft bool, maxFirstChunkMs int) bool {
	defaultEnabled := streamDraft && maxFirstChunkMs > 0
	return envBoolDefault("SCENARIO_DRAFT_WARMUP", defaultEnabled)
}

func scenarioPreflightRequiredModels(baseURL, model string) []string {
	if value := strings.TrimSpace(os.Getenv("SCENARIO_PREFLIGHT_REQUIRED_MODELS")); value != "" {
		return splitCSV(value)
	}
	models := []string{model}
	if strings.Contains(strings.ToLower(strings.TrimSpace(baseURL)), "/llama/v1") {
		models = append(models,
			os.Getenv("LLM_MODEL"),
			os.Getenv("STYLE_LLM_MODEL"),
			os.Getenv("BRIEF_LLM_MODEL"),
			os.Getenv("ARTICLE_LLM_MODEL"),
			os.Getenv("DRAFT_LLM_MODEL"),
			os.Getenv("VERIFY_LLM_MODEL"),
		)
	}
	return uniqueStrings(models)
}

func missingModels(models, required []string) []string {
	seen := map[string]bool{}
	for _, model := range models {
		seen[strings.TrimSpace(model)] = true
	}
	var missing []string
	for _, model := range required {
		if !seen[strings.TrimSpace(model)] {
			missing = append(missing, model)
		}
	}
	return missing
}

func preflightFailure(report scenarioPreflightReport) string {
	if strings.TrimSpace(report.Error) != "" {
		return report.Error
	}
	if len(report.MissingModels) > 0 {
		return "missing models: " + strings.Join(report.MissingModels, ", ")
	}
	return "unknown preflight failure"
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

func scenarioAttemptPassed(result draftapp.GenerateResult, runes, minRunes int, minStyleScore float64, minKeywordOverlap, maxFirstChunkMs int, firstChunk time.Duration) bool {
	return result.Evaluation.Comparison.Score >= minStyleScore &&
		keywordOverlapGatePassed(result, minKeywordOverlap) &&
		runes >= minRunes &&
		firstChunkGatePassed(firstChunk, maxFirstChunkMs) &&
		verificationGatePassed(result.Verification)
}

func betterScenarioAttempt(candidate, current scenarioAttemptResult, minRunes int, minStyleScore float64, minKeywordOverlap int) bool {
	if candidate.Attempt == 0 {
		return false
	}
	if current.Attempt == 0 {
		return true
	}

	candidatePassed := scenarioAttemptPassed(candidate.Result, candidate.Runes, minRunes, minStyleScore, minKeywordOverlap, 0, 0)
	currentPassed := scenarioAttemptPassed(current.Result, current.Runes, minRunes, minStyleScore, minKeywordOverlap, 0, 0)
	if candidatePassed != currentPassed {
		return candidatePassed
	}

	candidateGateScore := scenarioAttemptGateScore(candidate.Result, candidate.Runes, minRunes, minStyleScore, minKeywordOverlap)
	currentGateScore := scenarioAttemptGateScore(current.Result, current.Runes, minRunes, minStyleScore, minKeywordOverlap)
	if candidateGateScore != currentGateScore {
		return candidateGateScore > currentGateScore
	}

	candidateKeywordOverlap := keywordOverlapScore(candidate.Result)
	currentKeywordOverlap := keywordOverlapScore(current.Result)
	if candidateKeywordOverlap != currentKeywordOverlap {
		return candidateKeywordOverlap > currentKeywordOverlap
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

func scenarioAttemptGateScore(result draftapp.GenerateResult, runes, minRunes int, minStyleScore float64, minKeywordOverlap int) int {
	score := 0
	if result.Evaluation.Comparison.Score >= minStyleScore {
		score++
	}
	if keywordOverlapGatePassed(result, minKeywordOverlap) {
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

func keywordOverlapScore(result draftapp.GenerateResult) int {
	if result.Evaluation.Comparison.MetricScores == nil {
		return 0
	}
	return result.Evaluation.Comparison.MetricScores["keyword_overlap"]
}

func keywordOverlapGatePassed(result draftapp.GenerateResult, minKeywordOverlap int) bool {
	return minKeywordOverlap <= 0 || keywordOverlapScore(result) >= minKeywordOverlap
}

func firstChunkGatePassed(firstChunk time.Duration, maxFirstChunkMs int) bool {
	return maxFirstChunkMs <= 0 || (firstChunk > 0 && firstChunk.Milliseconds() <= int64(maxFirstChunkMs))
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

func scenarioRetryFeedback(result draftapp.GenerateResult, runes, minRunes int, minStyleScore float64, minKeywordOverlap, maxFirstChunkMs int, firstChunk time.Duration) string {
	var feedback []string
	if runes < minRunes {
		feedback = append(feedback, fmt.Sprintf("前回の下書きは%d字で、最低%d字に届かなかった。次は各見出しを具体例・判断理由・読者の次の行動で厚くし、必ず%d字以上にする", runes, minRunes, minRunes))
	}
	if result.Evaluation.Comparison.Score < minStyleScore {
		feedback = append(feedback, fmt.Sprintf("前回の文体スコアは%.1fで、最低%.1fに届かなかった。文体ガイドの一人称、頻出テーマ、段落リズムを優先して全文を書き直す", result.Evaluation.Comparison.Score, minStyleScore))
	}
	if minKeywordOverlap > 0 && keywordOverlapScore(result) < minKeywordOverlap {
		feedback = append(feedback, fmt.Sprintf("前回のkeyword_overlapは%dで、最低%dに届かなかった。文体ガイドの頻出語と主題語を本文の見出し・具体例・結論に自然に入れる", keywordOverlapScore(result), minKeywordOverlap))
	}
	if maxFirstChunkMs > 0 && !firstChunkGatePassed(firstChunk, maxFirstChunkMs) {
		feedback = append(feedback, fmt.Sprintf("前回のfirst chunkは%dmsで、最大%dmsを超えた。次は冒頭から即座に本文生成に入る", firstChunk.Milliseconds(), maxFirstChunkMs))
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

func envBoolDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if cleaned := strings.TrimSpace(part); cleaned != "" {
			values = append(values, cleaned)
		}
	}
	return values
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	unique := make([]string, 0, len(values))
	for _, value := range values {
		cleaned := strings.TrimSpace(value)
		if cleaned == "" || seen[cleaned] {
			continue
		}
		seen[cleaned] = true
		unique = append(unique, cleaned)
	}
	return unique
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
