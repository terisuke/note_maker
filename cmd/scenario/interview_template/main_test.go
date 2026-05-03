package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestRunScenarioCoversAllPersonaFormatTemplates(t *testing.T) {
	outputDir := t.TempDir()
	report, err := runScenario(context.Background(), outputDir)
	if err != nil {
		t.Fatalf("run scenario: %v", err)
	}

	wantCases := len(personadomain.DefaultRegistry().List()) * len(outputformat.DefaultRegistry().List())
	if len(report.Cases) != wantCases {
		t.Fatalf("cases = %d, want %d", len(report.Cases), wantCases)
	}
	if !report.OfflineOnly {
		t.Fatal("scenario must remain offline-only")
	}
	for _, result := range report.Cases {
		if result.QuestionCount < len(briefdomain.FixedQuestions()) {
			t.Fatalf("%s question count = %d", result.ID, result.QuestionCount)
		}
		if !allChecksPassed(result.TemplateChecks) {
			t.Fatalf("%s template checks failed: %#v", result.ID, result.TemplateChecks)
		}
		if !allChecksPassed(result.BriefChecks) {
			t.Fatalf("%s brief checks failed: %#v", result.ID, result.BriefChecks)
		}
		if result.DeepDiveCount != briefdomain.MaxTotalFollowUps {
			t.Fatalf("%s deep dives = %d, want %d", result.ID, result.DeepDiveCount, briefdomain.MaxTotalFollowUps)
		}
		if _, err := os.Stat(result.BriefPath); err != nil {
			t.Fatalf("%s brief artifact: %v", result.ID, err)
		}
		if _, err := os.Stat(result.SessionPath); err != nil {
			t.Fatalf("%s session artifact: %v", result.ID, err)
		}
	}
}

func TestScenarioWritesSimulatedArticleBriefs(t *testing.T) {
	outputDir := t.TempDir()
	report, err := runScenario(context.Background(), outputDir)
	if err != nil {
		t.Fatalf("run scenario: %v", err)
	}

	target := findCase(t, report.Cases, personadomain.IDCloudia+"_"+outputformat.IDQiitaArticle)
	encoded, err := os.ReadFile(target.BriefPath)
	if err != nil {
		t.Fatalf("read brief: %v", err)
	}
	var brief briefdomain.ArticleBrief
	if err := json.Unmarshal(encoded, &brief); err != nil {
		t.Fatalf("decode brief: %v", err)
	}
	if brief.PersonaID != personadomain.IDCloudia {
		t.Fatalf("persona = %q", brief.PersonaID)
	}
	if brief.OutputFormatID != outputformat.IDQiitaArticle {
		t.Fatalf("format = %q", brief.OutputFormatID)
	}
	assertAnswerPresent(t, brief.CustomAnswers, briefdomain.QuestionIDTargetStack)
	assertAnswerPresent(t, brief.CustomAnswers, briefdomain.QuestionIDCodeExamples)
	assertAnswerPresent(t, brief.CustomAnswers, briefdomain.QuestionIDCloudiaViewpoint)
	if len(brief.DeepDives) != briefdomain.MaxTotalFollowUps {
		t.Fatalf("deep dives = %d, want %d", len(brief.DeepDives), briefdomain.MaxTotalFollowUps)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "report.json")); err != nil {
		t.Fatalf("report artifact: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "cases.md")); err != nil {
		t.Fatalf("cases markdown artifact: %v", err)
	}
}

func findCase(t *testing.T, cases []caseResult, id string) caseResult {
	t.Helper()
	for _, item := range cases {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("case %s not found", id)
	return caseResult{}
}

func assertAnswerPresent(t *testing.T, answers []briefdomain.BriefAnswer, questionID string) {
	t.Helper()
	for _, answer := range answers {
		if answer.QuestionID == questionID && answer.Content != "" {
			return
		}
	}
	t.Fatalf("answer %s not found in %#v", questionID, answers)
}
