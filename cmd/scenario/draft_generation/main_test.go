package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
)

func TestWriteFailureAttemptPreservesRawOutputsAndRuntimeMetrics(t *testing.T) {
	outputDir := t.TempDir()
	generateErr := &draftapp.UnusableDraftError{
		FormatID: "zenn_article",
		Err:      errors.New("zenn article must use :::message, not Qiita :::note"),
		Attempts: []draftapp.GenerationAttempt{
			{
				Index:           1,
				Kind:            "initial",
				RawOutput:       "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n:::note info\nwrong\n:::",
				ValidationError: "zenn article must use :::message, not Qiita :::note",
			},
			{
				Index:           2,
				Kind:            "format_repair",
				RawOutput:       "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n:::note warn\nstill wrong\n:::",
				ValidationError: "zenn article must use :::message, not Qiita :::note",
			},
		},
	}
	metrics := attemptRuntimeMetrics{
		ElapsedSeconds: 1.25,
		TimeoutSeconds: 30,
		Streaming:      true,
		FirstChunkMs:   120,
		Chunks:         3,
	}
	context := failureAttemptContext{
		LLMBaseURL:     "http://evo-x2.tailb30e58.ts.net/v1",
		LLMModel:       "gemma4:31b",
		VerifyModel:    "gemma4:latest",
		PersonaID:      "cloudia",
		OutputFormatID: "zenn_article",
	}

	artifacts := writeRawAttemptArtifacts(outputDir, 2, generationAttemptsFromError(generateErr))
	failurePath := writeFailureAttempt(outputDir, 2, generateErr, metrics, context, artifacts)

	if len(artifacts) != 2 {
		t.Fatalf("artifacts = %#v, want 2", artifacts)
	}
	for _, artifact := range artifacts {
		content, err := os.ReadFile(artifact.Path)
		if err != nil {
			t.Fatalf("read raw artifact %s: %v", artifact.Path, err)
		}
		if len(content) == 0 {
			t.Fatalf("raw artifact %s was empty", artifact.Path)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "raw_attempt_2_generation_1_initial.txt")); err != nil {
		t.Fatalf("missing initial raw artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "raw_attempt_2_generation_2_format_repair.txt")); err != nil {
		t.Fatalf("missing repair raw artifact: %v", err)
	}

	encoded, err := os.ReadFile(failurePath)
	if err != nil {
		t.Fatalf("read failure artifact: %v", err)
	}
	var report failureAttemptReport
	if err := json.Unmarshal(encoded, &report); err != nil {
		t.Fatalf("decode failure artifact: %v", err)
	}
	if report.Attempt != 2 {
		t.Fatalf("attempt = %d, want 2", report.Attempt)
	}
	if report.ValidationError != "zenn article must use :::message, not Qiita :::note" {
		t.Fatalf("validation error = %q", report.ValidationError)
	}
	if report.RuntimeMetrics.ElapsedSeconds != 1.25 || report.RuntimeMetrics.FirstChunkMs != 120 || report.RuntimeMetrics.Chunks != 3 {
		t.Fatalf("runtime metrics not preserved: %#v", report.RuntimeMetrics)
	}
	if report.Context.LLMBaseURL != context.LLMBaseURL || report.Context.LLMModel != context.LLMModel || report.Context.OutputFormatID != context.OutputFormatID {
		t.Fatalf("runtime context not preserved: %#v", report.Context)
	}
	if len(report.RawOutputs) != 2 || report.RawOutputs[0].ValidationError == "" {
		t.Fatalf("raw output metadata not preserved: %#v", report.RawOutputs)
	}
}

func TestValidateScenarioInputsRejectsMismatchedStyleArtifacts(t *testing.T) {
	profile := authordomain.AuthorStyleProfile{ID: "profile_case"}
	guide := authordomain.WritingStyleGuide{ProfileID: "other_profile"}
	brief := briefdomain.ArticleBrief{StyleProfileID: "profile_case"}

	err := validateScenarioInputs(profile, guide, brief)
	if err == nil {
		t.Fatal("expected guide/profile mismatch")
	}

	guide.ProfileID = "profile_case"
	brief.StyleProfileID = "other_profile"
	err = validateScenarioInputs(profile, guide, brief)
	if err == nil {
		t.Fatal("expected brief/profile mismatch")
	}
}

func TestVerificationGateFailsPerformedFailedVerification(t *testing.T) {
	if !verificationGatePassed(draftapp.FinalVerification{}) {
		t.Fatal("not-performed verification should not block scenario")
	}
	if verificationGatePassed(draftapp.FinalVerification{Performed: true, Passed: false}) {
		t.Fatal("performed failed verification should block scenario")
	}
}

func TestScenarioRetryFeedbackCapturesLengthStyleAndVerificationFailures(t *testing.T) {
	result := draftapp.GenerateResult{
		Evaluation: draftapp.StyleEvaluation{
			Comparison: articledomain.StyleComparison{Score: 73.5},
		},
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    false,
			Summary:   "根拠が不足している",
		},
	}

	result.Evaluation.Comparison.MetricScores = map[string]int{"keyword_overlap": 64}
	feedback := scenarioRetryFeedback(result, 2554, 2800, 82, 70, 8000, 9*time.Second)

	for _, want := range []string{"最低2800字", "文体スコアは73.5", "keyword_overlapは64", "first chunkは9000ms", "根拠が不足している"} {
		if !strings.Contains(feedback, want) {
			t.Fatalf("feedback missing %q: %s", want, feedback)
		}
	}

	brief := briefWithScenarioRetryFeedback(briefdomain.ArticleBrief{
		MustInclude:           "体験を含める",
		TargetLengthStructure: "3000字前後",
	}, feedback)
	if !strings.Contains(brief.MustInclude, "再生成条件") || !strings.Contains(brief.TargetLengthStructure, "最低2800字") {
		t.Fatalf("brief retry feedback not applied: %+v", brief)
	}
}

func TestBetterScenarioAttemptKeepsBestWhenRetryRegresses(t *testing.T) {
	const (
		minRunes      = 2400
		minStyleScore = 80
	)
	selected := scenarioAttemptResult{
		Attempt: 1,
		Result: scenarioSelectionResult(87, draftapp.FinalVerification{
			Performed: true,
			Passed:    false,
			Summary:   "根拠が不足している",
		}),
		Runes: 3100,
	}
	regressedRetry := scenarioAttemptResult{
		Attempt: 2,
		Result: scenarioSelectionResult(58, draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
		}),
		Runes: 2600,
	}

	if betterScenarioAttempt(regressedRetry, selected, minRunes, minStyleScore) {
		t.Fatalf("regressed retry replaced selected attempt; gate scores retry=%d selected=%d", scenarioAttemptGateScore(regressedRetry.Result, regressedRetry.Runes, minRunes, minStyleScore), scenarioAttemptGateScore(selected.Result, selected.Runes, minRunes, minStyleScore))
	}
}

func TestBetterScenarioAttemptPrefersFullPassOverHigherFailingScore(t *testing.T) {
	const (
		minRunes      = 2400
		minStyleScore = 80
	)
	selected := scenarioAttemptResult{
		Attempt: 1,
		Result: scenarioSelectionResult(96, draftapp.FinalVerification{
			Performed: true,
			Passed:    false,
		}),
		Runes: 3600,
	}
	fullPass := scenarioAttemptResult{
		Attempt: 2,
		Result: scenarioSelectionResult(82, draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
		}),
		Runes: 2400,
	}

	if !betterScenarioAttempt(fullPass, selected, minRunes, minStyleScore) {
		t.Fatal("full pass should replace higher-scoring failed verification attempt")
	}
}

func TestBetterScenarioAttemptUsesLaterAttemptOnlyAsTieBreaker(t *testing.T) {
	const (
		minRunes      = 2400
		minStyleScore = 80
	)
	selected := scenarioAttemptResult{
		Attempt: 1,
		Result: scenarioSelectionResult(79, draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
		}),
		Runes: 2400,
	}
	equalQualityRetry := scenarioAttemptResult{
		Attempt: 2,
		Result: scenarioSelectionResult(79, draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
		}),
		Runes: 2400,
	}

	if !betterScenarioAttempt(equalQualityRetry, selected, minRunes, minStyleScore) {
		t.Fatal("later attempt should win only when pass state, gate score, style score, and length all tie")
	}
}

func TestScenarioAttemptPassedHonorsKeywordAndFirstChunkGates(t *testing.T) {
	result := scenarioSelectionResult(88, draftapp.FinalVerification{Performed: true, Passed: true})
	result.Evaluation.Comparison.MetricScores = map[string]int{"keyword_overlap": 72}

	if !scenarioAttemptPassed(result, 3000, 2800, 82, 70, 8000, 1200*time.Millisecond) {
		t.Fatal("expected all gates to pass")
	}
	if scenarioAttemptPassed(result, 3000, 2800, 82, 75, 8000, 1200*time.Millisecond) {
		t.Fatal("keyword overlap below threshold must fail")
	}
	if scenarioAttemptPassed(result, 3000, 2800, 82, 70, 8000, 9*time.Second) {
		t.Fatal("first chunk over threshold must fail")
	}
}

func scenarioSelectionResult(score float64, verification draftapp.FinalVerification) draftapp.GenerateResult {
	return draftapp.GenerateResult{
		Evaluation: draftapp.StyleEvaluation{
			Comparison: articledomain.StyleComparison{Score: score},
		},
		Verification: verification,
	}
}
