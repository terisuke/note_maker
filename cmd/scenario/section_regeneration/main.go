package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	"github.com/teradakousuke/note_maker/internal/infrastructure/llamacpp"
)

const defaultOutputDir = "tmp/section_regeneration"

func main() {
	if os.Getenv("RUN_LOCAL_LLM_SCENARIO") != "1" {
		fatalf("set RUN_LOCAL_LLM_SCENARIO=1 to run local LLM section-regeneration scenario")
	}

	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	profilePath := envOrDefault("AUTHOR_PROFILE_PATH", "tmp/author_style/profile.json")
	guidePath := envOrDefault("WRITING_GUIDE_PATH", "tmp/author_style/guide.json")
	briefPath := envOrDefault("ARTICLE_BRIEF_PATH", "tmp/brief_interview/brief.json")
	draftPath := envOrDefault("DRAFT_MARKDOWN_PATH", "tmp/draft_generation_issue20/draft.md")
	sectionAnchor := envOrDefault("SCENARIO_SECTION_ANCHOR", "実装と検証")
	baseURL := envFirst("http://127.0.0.1:8081/v1", "LLM_BASE_URL", "LLAMACPP_BASE_URL")
	model := envFirst("gemma4:31b", "DRAFT_LLM_MODEL", "LLM_MODEL", "LLAMACPP_MODEL")
	verifyModel := envFirst("gemma4:latest", "VERIFY_LLM_MODEL", "LLM_MODEL", "LLAMACPP_MODEL")

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	var profile authordomain.AuthorStyleProfile
	var guide authordomain.WritingStyleGuide
	var brief briefdomain.ArticleBrief
	readJSON(profilePath, &profile)
	readJSON(guidePath, &guide)
	readJSON(briefPath, &brief)
	draftMarkdown := readFile(draftPath)

	persona, ok := personadomain.DefaultRegistry().Get(brief.PersonaID)
	if !ok {
		fatalf("persona %q not found", brief.PersonaID)
	}
	format, ok := outputformat.DefaultRegistry().Get(brief.OutputFormatID)
	if !ok {
		fatalf("output format %q not found", brief.OutputFormatID)
	}
	target, err := draftapp.FindMarkdownSection(draftMarkdown, sectionAnchor)
	if err != nil {
		fatalf("find target section: %v", err)
	}

	generator, err := llamacpp.NewClientFromEnvForPurpose("DRAFT")
	if err != nil {
		fatalf("create draft llm client: %v", err)
	}
	verifyClient, err := llamacpp.NewClientFromEnvForPurpose("VERIFY")
	if err != nil {
		fatalf("create verification llm client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), scenarioTimeout())
	defer cancel()
	started := time.Now()
	result, err := draftapp.NewService(generator).RegenerateSection(ctx, draftapp.RegenerateSectionRequest{
		GenerateRequest: draftapp.GenerateRequest{
			StyleGuide:    guide,
			Brief:         brief,
			AuthorProfile: profile,
			Persona:       persona,
			OutputFormat:  format,
		},
		DraftMarkdown: draftMarkdown,
		SectionAnchor: sectionAnchor,
	})
	elapsed := time.Since(started)
	if err != nil {
		fatalf("regenerate section: %v", err)
	}

	updatedDraft, err := articledomain.NewDraftForFormat(result.UpdatedDraftMarkdown, format.ID)
	if err != nil {
		fatalf("validate updated draft: %v", err)
	}
	evaluation := draftapp.EvaluateStyle(profile, brief, updatedDraft)
	verifyStarted := time.Now()
	verification, err := draftapp.NewLightweightVerifier(verifyClient).VerifyDraft(ctx, draftapp.VerificationRequest{
		StyleGuide:    guide,
		Brief:         brief,
		AuthorProfile: profile,
		Persona:       persona,
		OutputFormat:  format,
		DraftMarkdown: updatedDraft.Markdown(),
		Evaluation:    evaluation,
	})
	verificationElapsed := time.Since(verifyStarted)
	if err != nil {
		fatalf("verify updated draft: %v", err)
	}
	verification.Performed = true

	unchangedPrefix := strings.HasPrefix(result.UpdatedDraftMarkdown, draftMarkdown[:target.Start])
	unchangedSuffix := strings.HasSuffix(result.UpdatedDraftMarkdown, draftMarkdown[target.End:])
	writeFile(filepath.Join(outputDir, "replacement.md"), result.ReplacementMarkdown+"\n")
	writeFile(filepath.Join(outputDir, "updated_draft.md"), result.UpdatedDraftMarkdown)
	writeJSON(filepath.Join(outputDir, "section.json"), result.Section)
	writeJSON(filepath.Join(outputDir, "evaluation.json"), evaluation)
	writeJSON(filepath.Join(outputDir, "verification.json"), verification)

	fmt.Printf("section regeneration scenario completed\n")
	fmt.Printf("section_anchor=%s\n", sectionAnchor)
	fmt.Printf("target_heading=%s\n", target.Heading)
	fmt.Printf("persona_id=%s\n", persona.ID)
	fmt.Printf("output_format_id=%s\n", format.ID)
	fmt.Printf("elapsed_seconds=%.2f\n", elapsed.Seconds())
	fmt.Printf("verification_elapsed_seconds=%.2f\n", verificationElapsed.Seconds())
	fmt.Printf("updated_runes=%d\n", len([]rune(result.UpdatedDraftMarkdown)))
	fmt.Printf("replacement_runes=%d\n", len([]rune(result.ReplacementMarkdown)))
	fmt.Printf("score=%.1f\n", evaluation.Comparison.Score)
	fmt.Printf("evaluation_passed=%v\n", evaluation.Passed)
	fmt.Printf("verification_passed=%v\n", verification.Passed)
	fmt.Printf("verification_summary=%s\n", verification.Summary)
	fmt.Printf("unchanged_prefix=%v\n", unchangedPrefix)
	fmt.Printf("unchanged_suffix=%v\n", unchangedSuffix)
	fmt.Printf("llm_base_url=%s\n", baseURL)
	fmt.Printf("llm_model=%s\n", model)
	fmt.Printf("verify_model=%s\n", verifyModel)
	fmt.Printf("replacement=%s\n", filepath.Join(outputDir, "replacement.md"))
	fmt.Printf("updated_draft=%s\n", filepath.Join(outputDir, "updated_draft.md"))
	if !unchangedPrefix || !unchangedSuffix {
		fatalf("non-target draft bytes were changed")
	}
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

func readFile(path string) string {
	encoded, err := os.ReadFile(path)
	if err != nil {
		fatalf("read %s: %v", path, err)
	}
	return string(encoded)
}

func writeJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", err)
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

func scenarioTimeout() time.Duration {
	seconds := envInt("SCENARIO_SECTION_TIMEOUT_SECONDS", 420)
	return time.Duration(seconds) * time.Second
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
