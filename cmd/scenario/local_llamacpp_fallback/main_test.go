package main

import (
	"strings"
	"testing"
)

func TestRequireLoopbackBaseURLRejectsEvoX2AndRemoteHosts(t *testing.T) {
	for _, raw := range []string{
		"http://evo-x2.tailb30e58.ts.net/v1",
		"http://192.168.1.10:8081/v1",
		"http://example.test/v1",
	} {
		if err := requireLoopbackBaseURL(raw); err == nil {
			t.Fatalf("expected %s to be rejected", raw)
		}
	}
}

func TestRequireLoopbackBaseURLAcceptsLocalhostAndLoopback(t *testing.T) {
	for _, raw := range []string{
		"http://localhost:8081/v1",
		"http://127.0.0.1:8081/v1",
		"http://[::1]:8081/v1",
	} {
		if err := requireLoopbackBaseURL(raw); err != nil {
			t.Fatalf("expected %s to be accepted: %v", raw, err)
		}
	}
}

func TestResultFromDraftOutputParsesThresholdMetrics(t *testing.T) {
	output := strings.Join([]string{
		"draft generation scenario completed",
		"scenario_passed=true",
		"attempt=2",
		"selected_attempt=2",
		"passed=true",
		"score=84.5",
		"keyword_overlap=72",
		"runes=3012",
		"verification_performed=true",
		"verification_passed=true",
		"elapsed_seconds=123.45",
		"streaming=true",
		"first_chunk_ms=1200",
		"chunks=44",
	}, "\n")

	result := resultFromDraftOutput(output)

	if !result.ScenarioPassed || result.Attempt != 2 || result.Score != 84.5 || result.KeywordOverlap != 72 || result.Runes != 3012 {
		t.Fatalf("unexpected parsed result: %#v", result)
	}
	if !result.VerificationPerformed || !result.VerificationPassed || result.FirstChunkMs != 1200 || result.Chunks != 44 {
		t.Fatalf("stream/verification fields not parsed: %#v", result)
	}
}

func TestCommandEnvClearsFallbackChainAndPinsLocalRuntime(t *testing.T) {
	t.Setenv("LLM_FALLBACK_BASE_URLS", "http://evo-x2.tailb30e58.ts.net/llama/v1")
	t.Setenv("DRAFT_FALLBACK_LLM_BASE_URLS", "http://evo-x2.tailb30e58.ts.net/llama/v1")
	config := scenarioConfig{
		BaseURL:           "http://127.0.0.1:8081/v1",
		Model:             "local-qwen",
		VerifyModel:       "local-verify",
		MinStyleScore:     82,
		MinKeywordOverlap: 70,
		MinDraftRunes:     2800,
		MaxAttempts:       2,
		TimeoutSeconds:    900,
		StreamDraft:       true,
	}

	env := keyValueEnv(commandEnv(config, nil))

	if env["LLM_BASE_URL"] != config.BaseURL || env["DRAFT_LLM_MODEL"] != config.Model || env["VERIFY_LLM_MODEL"] != config.VerifyModel {
		t.Fatalf("local runtime env not pinned: %#v", env)
	}
	if env["LLM_FALLBACK_BASE_URLS"] != "" || env["DRAFT_LLM_FALLBACK_BASE_URLS"] != "" || env["DRAFT_FALLBACK_LLM_BASE_URLS"] != "" {
		t.Fatalf("fallback chain was not cleared: %#v", env)
	}
}

func TestLocalFallbackPassedRequiresIssue36KeywordGate(t *testing.T) {
	report := scenarioReport{
		Thresholds: thresholdReport{
			MinStyleScore:     82,
			MinKeywordOverlap: 70,
			MinDraftRunes:     2800,
		},
		Result: resultReport{
			ScenarioPassed:     true,
			Score:              84.5,
			KeywordOverlap:     69,
			Runes:              3200,
			VerificationPassed: true,
		},
	}

	if localFallbackPassed(report) {
		t.Fatal("keyword_overlap below #36 threshold must fail even when the upstream draft scenario passed")
	}
	report.Result.KeywordOverlap = 70
	if !localFallbackPassed(report) {
		t.Fatalf("expected #36 gate to pass at exact thresholds: %#v", report)
	}
}

func keyValueEnv(values []string) map[string]string {
	out := map[string]string{}
	for _, entry := range values {
		key, value, _ := strings.Cut(entry, "=")
		out[key] = value
	}
	return out
}
