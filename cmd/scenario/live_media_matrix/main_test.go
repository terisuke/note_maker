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
		BriefPath: "tmp/media_matrix/briefs/cloudia_zenn_tutorial.json",
		ActiveGates: scenarioGates{
			MinRunes:      1800,
			MinStyleScore: 82,
		},
	}

	env := draftGenerationEnv(item, "tmp/media_matrix/live/cloudia_zenn_tutorial")
	for _, expected := range []string{
		"RUN_LOCAL_LLM_SCENARIO=1",
		"ARTICLE_BRIEF_PATH=tmp/media_matrix/briefs/cloudia_zenn_tutorial.json",
		"SCENARIO_OUTPUT_DIR=tmp/media_matrix/live/cloudia_zenn_tutorial",
		"SCENARIO_MIN_STYLE_SCORE=82.0",
		"SCENARIO_MIN_DRAFT_RUNES=1800",
	} {
		if !contains(env, expected) {
			t.Fatalf("draft generation env missing %q: %v", expected, env)
		}
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

func TestMarkdownReportShowsGatesBesideActuals(t *testing.T) {
	report := aggregateReport{
		Rows: []resultRow{
			{
				CaseID:        "cloudia_qiita_how_to",
				Medium:        "Qiita",
				Style:         "practical how-to",
				Status:        "passed",
				Score:         83.2,
				MinStyleScore: 82,
				Runes:         1510,
				MinRunes:      1400,
				ActiveGates: scenarioGates{
					MinRunes:             1400,
					MinStyleScore:        82,
					StructuralGateLabels: []string{"qiita_long_form", "note_block", "diff_code"},
				},
				OutputDir: "tmp/media_matrix/live/cloudia_qiita_how_to",
			},
		},
	}

	markdown := markdownReport(report)
	for _, expected := range []string{
		"Gates",
		"83.2 / 82.0",
		"1510 / 1400",
		"qiita_long_form",
	} {
		if !strings.Contains(markdown, expected) {
			t.Fatalf("markdown report missing %q:\n%s", expected, markdown)
		}
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
