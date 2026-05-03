package main

import (
	"strings"
	"testing"

	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestActiveGatesForCaseSeparatesHomepageFromLongForm(t *testing.T) {
	var homepage matrixCase
	longForm := map[string]scenarioGates{}
	for _, item := range plannedCases() {
		gates := activeGatesForCase(item)
		if gates.MinRunes <= 0 {
			t.Fatalf("%s has no minimum rune gate", item.ID)
		}
		if gates.MinStyleScore <= 0 {
			t.Fatalf("%s has no minimum style score gate", item.ID)
		}
		if len(gates.StructuralGateLabels) == 0 {
			t.Fatalf("%s has no structural gate labels", item.ID)
		}
		if len(gates.StructuralSignals) == 0 {
			t.Fatalf("%s has no structural signals", item.ID)
		}

		if item.ID == "cor_homepage_section" {
			homepage = item
			continue
		}
		longForm[item.ID] = gates
	}
	if homepage.ID == "" {
		t.Fatal("homepage case was not found")
	}

	homepageGates := activeGatesForCase(homepage)
	if homepageGates.MinRunes >= 1000 {
		t.Fatalf("homepage minimum runes should stay short HTML focused, got %d", homepageGates.MinRunes)
	}
	if !contains(homepageGates.StructuralGateLabels, "homepage_short_html") {
		t.Fatalf("homepage gates missing homepage_short_html label: %v", homepageGates.StructuralGateLabels)
	}
	for _, signal := range []string{"<section", "<h2", "<p", "href="} {
		if !contains(homepageGates.StructuralSignals, signal) {
			t.Fatalf("homepage gates missing structural signal %q: %v", signal, homepageGates.StructuralSignals)
		}
	}

	for id, gates := range longForm {
		if gates.MinRunes < 1400 {
			t.Fatalf("%s long-form minimum runes should remain strict, got %d", id, gates.MinRunes)
		}
		if gates.MinStyleScore < 80 {
			t.Fatalf("%s long-form style gate should remain strict, got %.1f", id, gates.MinStyleScore)
		}
	}
}

func TestPlannedLLMCommandIncludesActiveGateEnv(t *testing.T) {
	gates := scenarioGates{MinRunes: 1800, MinStyleScore: 82}
	command := plannedLLMCommand(
		"tmp/media_matrix",
		"case_id",
		"tmp/media_matrix/briefs/case_id.json",
		"tmp/media_matrix/styles/case_id/profile.json",
		"tmp/media_matrix/styles/case_id/guide.json",
		gates,
	)
	for _, expected := range []string{
		"RUN_LOCAL_LLM_SCENARIO=1",
		"SCENARIO_MIN_STYLE_SCORE=82.0",
		"SCENARIO_MIN_DRAFT_RUNES=1800",
		"ARTICLE_BRIEF_PATH=tmp/media_matrix/briefs/case_id.json",
		"AUTHOR_PROFILE_PATH=tmp/media_matrix/styles/case_id/profile.json",
		"WRITING_GUIDE_PATH=tmp/media_matrix/styles/case_id/guide.json",
		"SCENARIO_OUTPUT_DIR=tmp/media_matrix/live/case_id",
	} {
		if !strings.Contains(command, expected) {
			t.Fatalf("planned command missing %q: %s", expected, command)
		}
	}
}

func TestScenarioStyleAssetsMatchBriefProfile(t *testing.T) {
	for _, item := range plannedCases() {
		persona, _ := personadomain.DefaultRegistry().Get(item.PersonaID)
		format, _ := outputformat.DefaultRegistry().Get(item.OutputFormatID)
		profile, guide := scenarioStyleAssets(item, persona, format)
		brief := buildBriefFromSession(item, profile.ID)

		if brief.StyleProfileID != profile.ID {
			t.Fatalf("%s brief profile = %q, want %q", item.ID, brief.StyleProfileID, profile.ID)
		}
		if guide.ProfileID != profile.ID {
			t.Fatalf("%s guide profile = %q, want %q", item.ID, guide.ProfileID, profile.ID)
		}
		if item.PersonaID == "cloudia" && profile.PreferredFirstPerson != "クラウディア" {
			t.Fatalf("%s preferred first person = %q", item.ID, profile.PreferredFirstPerson)
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
