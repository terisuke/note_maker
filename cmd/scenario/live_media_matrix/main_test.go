package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlannedRowReportsActiveGates(t *testing.T) {
	item := matrixCase{
		ID:             "cor_homepage_section",
		Medium:         "homepage",
		Style:          "concise product section",
		PersonaID:      "terisuke",
		OutputFormatID: "homepage_section",
		ActiveGates: scenarioGates{
			MinRunes:             350,
			MinStyleScore:        72,
			StructuralGateLabels: []string{"homepage_short_html", "section_element", "cta"},
			StructuralSignals:    []string{"<section", "<h2", "<p", "href="},
		},
	}

	row := plannedRow(item, "tmp/media_matrix/live/cor_homepage_section")
	if row.Status != "planned" {
		t.Fatalf("planned row status = %q", row.Status)
	}
	if row.MinRunes != 350 || row.ActiveGates.MinRunes != 350 {
		t.Fatalf("planned row did not report rune gate: %+v", row)
	}
	if row.MinStyleScore != 72 || row.ActiveGates.MinStyleScore != 72 {
		t.Fatalf("planned row did not report style gate: %+v", row)
	}
	if !contains(row.ActiveGates.StructuralGateLabels, "homepage_short_html") {
		t.Fatalf("planned row missing structural labels: %+v", row.ActiveGates.StructuralGateLabels)
	}
}

func TestDraftGenerationEnvPassesActiveGates(t *testing.T) {
	item := matrixCase{
		BriefPath:   "tmp/media_matrix/briefs/cloudia_zenn_tutorial.json",
		ProfilePath: "tmp/media_matrix/styles/cloudia_zenn_tutorial/profile.json",
		GuidePath:   "tmp/media_matrix/styles/cloudia_zenn_tutorial/guide.json",
		ActiveGates: scenarioGates{
			MinRunes:      1800,
			MinStyleScore: 82,
		},
	}

	env := draftGenerationEnv(item, "tmp/media_matrix/live/cloudia_zenn_tutorial")
	for _, expected := range []string{
		"RUN_LOCAL_LLM_SCENARIO=1",
		"ARTICLE_BRIEF_PATH=tmp/media_matrix/briefs/cloudia_zenn_tutorial.json",
		"AUTHOR_PROFILE_PATH=tmp/media_matrix/styles/cloudia_zenn_tutorial/profile.json",
		"WRITING_GUIDE_PATH=tmp/media_matrix/styles/cloudia_zenn_tutorial/guide.json",
		"SCENARIO_OUTPUT_DIR=tmp/media_matrix/live/cloudia_zenn_tutorial",
		"SCENARIO_MIN_STYLE_SCORE=82.0",
		"SCENARIO_MIN_DRAFT_RUNES=1800",
	} {
		if !contains(env, expected) {
			t.Fatalf("draft generation env missing %q: %v", expected, env)
		}
	}
}

func TestApplyRunMetricsCapturesAttemptCount(t *testing.T) {
	row := resultRow{}
	applyRunMetrics(&row, map[string]string{
		"attempt":             "2",
		"scenario_passed":     "true",
		"score":               "88.1",
		"min_style_score":     "82.0",
		"runes":               "5040",
		"min_draft_runes":     "1800",
		"verification_passed": "true",
	}, scenarioGates{MinStyleScore: 82, MinRunes: 1800})

	if row.Attempt != 2 {
		t.Fatalf("attempt = %d, want 2", row.Attempt)
	}
}

func TestAttachQualityGatesReportsScenarioGateDetails(t *testing.T) {
	rows := attachQualityGates([]resultRow{
		{
			Status:                "failed",
			ScenarioPassed:        false,
			FailureGroup:          "draft_length",
			Error:                 "draft length 2554 below scenario minimum 2800",
			Score:                 90,
			MinStyleScore:         82,
			Runes:                 2554,
			MinRunes:              2800,
			VerificationPerformed: true,
			VerificationPassed:    true,
			DraftPath:             "tmp/media_matrix/live/terisuke_note_essay/draft.md",
			ActiveGates: scenarioGates{
				StructuralGateLabels: []string{"note_long_form", "reader_takeaway"},
				StructuralSignals:    []string{"# ", "## "},
			},
		},
	})

	gate := rows[0].QualityGate
	if gate.Passed {
		t.Fatalf("quality gate passed unexpectedly: %+v", gate)
	}
	if gate.FailureGroup != "draft_length" || gate.Outcome != "failed" {
		t.Fatalf("quality gate failure classification = %+v", gate)
	}
	if !gate.StylePassed || gate.LengthPassed || !gate.VerificationPassed || !gate.StructuralGatePassed {
		t.Fatalf("quality gate booleans = %+v", gate)
	}
	if gate.Reason == "" || !contains(gate.StructuralGateLabels, "reader_takeaway") {
		t.Fatalf("quality gate detail missing: %+v", gate)
	}
}

func TestApplyStructuralGatesFailsMissingSignals(t *testing.T) {
	outputDir := t.TempDir()
	draftPath := filepath.Join(outputDir, "draft.md")
	if err := os.WriteFile(draftPath, []byte("---\ntitle: ok\n---\n\n## 手順\n\n本文です。\n"), 0o644); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	row := resultRow{
		Status:         "passed",
		ScenarioPassed: true,
		DraftPath:      draftPath,
		ActiveGates: scenarioGates{
			StructuralSignals: []string{"---", ":::message", "```"},
		},
	}
	applyStructuralGates(&row)

	if row.ScenarioPassed {
		t.Fatalf("expected structural gate failure: %+v", row)
	}
	if row.Status != "failed" {
		t.Fatalf("status = %q, want failed", row.Status)
	}
	if row.FailureGroup != "structural_gate" {
		t.Fatalf("failure group = %q", row.FailureGroup)
	}
	if !strings.Contains(row.Error, ":::message") || !strings.Contains(row.Error, "```") {
		t.Fatalf("missing structural signal detail: %q", row.Error)
	}
}

func TestRunCaseClearsStaleArtifactsWithoutResume(t *testing.T) {
	outputDir := t.TempDir()
	stalePath := filepath.Join(outputDir, "stderr.txt")
	if err := os.WriteFile(stalePath, []byte("old failure"), 0o644); err != nil {
		t.Fatalf("write stale artifact: %v", err)
	}

	item := matrixCase{BriefPath: "missing.json"}
	row := runCase(item, outputDir)

	if row.Status != "failed" {
		t.Fatalf("status = %q, want failed", row.Status)
	}
	if row.FailureGroup != "runtime_or_runner" {
		t.Fatalf("failure group = %q, want runtime_or_runner", row.FailureGroup)
	}
	content, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("expected fresh stderr artifact: %v", err)
	}
	if strings.Contains(string(content), "old failure") {
		t.Fatalf("stale stderr was not cleared:\n%s", string(content))
	}
}

func TestApplyFailureArtifactsRestoresRuntimeDiagnostics(t *testing.T) {
	outputDir := t.TempDir()
	failure := failureAttemptReport{
		Attempt: 3,
		RuntimeMetrics: attemptRuntimeMetrics{
			ElapsedSeconds: 12.5,
			FirstChunkMs:   250,
			Chunks:         4,
		},
		Context: failureContext{
			LLMBaseURL:  "http://evo-x2.tailb30e58.ts.net/v1",
			LLMModel:    "gemma4:31b",
			VerifyModel: "gemma4:latest",
		},
		RawOutputs: []rawAttemptArtifact{
			{Path: filepath.Join(outputDir, "raw_attempt_1_generation_1_initial.txt")},
		},
	}
	encoded, err := json.Marshal(failure)
	if err != nil {
		t.Fatalf("encode failure: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "failure_attempt_1.json"), encoded, 0o644); err != nil {
		t.Fatalf("write failure: %v", err)
	}

	row := resultRow{}
	applyFailureArtifacts(&row, outputDir)

	if row.FailurePath == "" {
		t.Fatalf("failure path was not restored: %+v", row)
	}
	if row.Attempt != 3 {
		t.Fatalf("attempt was not restored from failure artifact: %+v", row)
	}
	if row.ElapsedSeconds != 12.5 || row.FirstChunkMS != 250 || row.Chunks != 4 {
		t.Fatalf("runtime metrics were not restored: %+v", row)
	}
	if row.LLMBaseURL != failure.Context.LLMBaseURL || row.LLMModel != failure.Context.LLMModel || row.VerifyModel != failure.Context.VerifyModel {
		t.Fatalf("runtime context was not restored: %+v", row)
	}
	if len(row.RawOutputPaths) != 1 || !strings.HasSuffix(row.RawOutputPaths[0], "initial.txt") {
		t.Fatalf("raw output paths were not restored: %+v", row.RawOutputPaths)
	}
}

func TestFailureGroupPrefersStyleScoreOverFinalVerification(t *testing.T) {
	row := resultRow{
		VerificationPerformed: true,
		VerificationPassed:    false,
		Score:                 57.6,
		MinStyleScore:         82,
		Error:                 "style score 57.6 below scenario minimum 82.0",
	}

	if got := failureGroup(row); got != "style_score" {
		t.Fatalf("failure group = %q, want style_score", got)
	}

	row.Score = 0
	row.MinStyleScore = 0
	if got := failureGroup(row); got != "style_score" {
		t.Fatalf("failure group from concrete error = %q, want style_score", got)
	}
}

func TestMarkdownReportShowsGatesBesideActuals(t *testing.T) {
	report := aggregateReport{
		SelectedCaseIDs: []string{"cloudia_qiita_how_to"},
		RunOrdinals:     []int{1},
		Rows: []resultRow{
			{
				RunOrdinal:     1,
				ComparisonKey:  "run_01/cloudia_qiita_how_to",
				CaseID:         "cloudia_qiita_how_to",
				Medium:         "Qiita",
				Style:          "practical how-to",
				Status:         "passed",
				ScenarioPassed: true,
				Score:          83.2,
				MinStyleScore:  82,
				Runes:          1510,
				MinRunes:       1400,
				ActiveGates: scenarioGates{
					MinRunes:             1400,
					MinStyleScore:        82,
					StructuralGateLabels: []string{"qiita_long_form", "note_block", "diff_code"},
				},
				OutputDir: "tmp/media_matrix/live/cloudia_qiita_how_to",
			},
		},
	}
	report.Summary = summarizeRows(report.Rows, len(report.SelectedCaseIDs))

	markdown := markdownReport(report)
	for _, expected := range []string{
		"Gates",
		"Selected cases: `cloudia_qiita_how_to`",
		"Run ordinals: `1`",
		"83.2 / 82.0",
		"1510 / 1400",
		"qiita_long_form",
	} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("markdown report missing %q:\n%s", expected, markdown)
		}
	}
}

func TestRepeatRunSummaryTracksSelectionOrdinalsAndGroups(t *testing.T) {
	rows := []resultRow{
		{
			RunOrdinal:     1,
			ComparisonKey:  "run_01/cloudia_qiita_how_to",
			CaseID:         "cloudia_qiita_how_to",
			Status:         "passed",
			ScenarioPassed: true,
			ElapsedSeconds: 12.25,
			Score:          84,
			Runes:          1500,
			LLMBaseURL:     "http://evo-x2.tailb30e58.ts.net/v1",
			LLMModel:       "gemma4:31b",
			VerifyModel:    "gemma4:latest",
		},
		{
			RunOrdinal:     2,
			ComparisonKey:  "run_02/cloudia_qiita_how_to",
			CaseID:         "cloudia_qiita_how_to",
			Status:         "passed",
			ElapsedSeconds: 20,
			Score:          79,
			Runes:          1320,
			LLMBaseURL:     "http://evo-x2.tailb30e58.ts.net/v1",
			LLMModel:       "gemma4:31b",
			VerifyModel:    "gemma4:latest",
		},
		{
			RunOrdinal:    2,
			ComparisonKey: "run_02/cloudia_zenn_tutorial",
			CaseID:        "cloudia_zenn_tutorial",
			Status:        "failed",
			Error:         "style score below gate",
			LLMBaseURL:    "http://evo-x2.tailb30e58.ts.net/v1",
			LLMModel:      "gemma4:31b",
		},
	}

	summary := summarizeRows(rows, 2)

	if summary.TotalRows != 3 || summary.SelectedCases != 2 {
		t.Fatalf("summary counts = %+v", summary)
	}
	if summary.PassedRows != 1 || summary.FailedRows != 2 {
		t.Fatalf("pass/fail counts = %+v", summary)
	}
	if len(summary.ConciseRows) != 2 {
		t.Fatalf("concise row count = %d, want 2", len(summary.ConciseRows))
	}
	if summary.ConciseRows[0].RunOrdinal != 1 || summary.ConciseRows[0].PassedRows != 1 {
		t.Fatalf("run 1 summary = %+v", summary.ConciseRows[0])
	}
	if summary.ConciseRows[1].RunOrdinal != 2 || summary.ConciseRows[1].FailedRows != 2 {
		t.Fatalf("run 2 summary = %+v", summary.ConciseRows[1])
	}
	failedGroup := summary.PassFailGroups[0]
	if failedGroup.Outcome != "failed" || failedGroup.Count != 2 {
		t.Fatalf("failed group = %+v", failedGroup)
	}

	runtime := runtimeFromRows(rows)
	if !runtime.TailnetEvoX2 || len(runtime.LLMBaseURLs) != 1 || runtime.LLMBaseURLs[0] != "http://evo-x2.tailb30e58.ts.net/v1" {
		t.Fatalf("runtime metadata = %+v", runtime)
	}
}

func TestRepeatRunOutputDirsKeepSingleRunCompatible(t *testing.T) {
	if got := outputDirForRun("tmp/media_matrix/live", "case_a", 1, 1); got != "tmp/media_matrix/live/case_a" {
		t.Fatalf("single run output dir = %q", got)
	}
	if got := outputDirForRun("tmp/media_matrix/live", "case_a", 2, 1); got != "tmp/media_matrix/live/run_02/case_a" {
		t.Fatalf("ordinal output dir = %q", got)
	}
	if got := outputDirForRun("tmp/media_matrix/live", "case_a", 1, 3); got != "tmp/media_matrix/live/run_01/case_a" {
		t.Fatalf("repeat output dir = %q", got)
	}
}

func TestSelectedCasesPreserveMatrixOrder(t *testing.T) {
	cases := []matrixCase{{ID: "zenn"}, {ID: "qiita"}, {ID: "note"}}
	selected := selectedCases(cases, selectedCaseIDs("note,zenn"))

	if got := strings.Join(caseIDs(selected), ","); got != "zenn,note" {
		t.Fatalf("selected case order = %q", got)
	}
}

func TestGeneratedAtIsStableOfflineAndOverridable(t *testing.T) {
	if got := generatedAt(false); got != offlineGeneratedAt {
		t.Fatalf("offline generated_at = %q, want %q", got, offlineGeneratedAt)
	}

	t.Setenv("LIVE_MEDIA_MATRIX_GENERATED_AT", "2026-05-03T00:00:00Z")
	if got := generatedAt(true); got != "2026-05-03T00:00:00Z" {
		t.Fatalf("generated_at override = %q", got)
	}
}

func TestApplyRunMetricsKeepsRuntimeEnvFallbacks(t *testing.T) {
	row := resultRow{
		LLMBaseURL:  "http://evo-x2.tailb30e58.ts.net/v1",
		LLMModel:    "gemma4:31b",
		VerifyModel: "gemma4:latest",
	}

	applyRunMetrics(&row, map[string]string{"score": "82.5"}, scenarioGates{})

	if row.LLMBaseURL != "http://evo-x2.tailb30e58.ts.net/v1" || row.LLMModel != "gemma4:31b" || row.VerifyModel != "gemma4:latest" {
		t.Fatalf("runtime fallbacks were overwritten: %+v", row)
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
