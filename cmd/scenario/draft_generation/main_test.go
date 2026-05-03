package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
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
