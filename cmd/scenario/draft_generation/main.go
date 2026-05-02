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

	baseURL := envFirst("http://127.0.0.1:8081/v1", "LLM_BASE_URL", "LLAMACPP_BASE_URL")
	model := envFirst("gemma4:31b", "DRAFT_LLM_MODEL", "LLM_MODEL", "LLAMACPP_MODEL")
	minStyleScore := envFloat("SCENARIO_MIN_STYLE_SCORE", 80)
	minDraftRunes := envInt("SCENARIO_MIN_DRAFT_RUNES", 2400)
	maxAttempts := envInt("DRAFT_MAX_ATTEMPTS", 2)
	streamDraft := os.Getenv("SCENARIO_STREAM_DRAFT") == "1"
	client, err := llamacpp.NewClientFromEnvForPurpose("DRAFT")
	if err != nil {
		fatalf("create local llm client: %v", err)
	}
	service := draftapp.NewService(client)

	var result draftapp.GenerateResult
	var finalElapsed time.Duration
	var finalFirstChunk time.Duration
	var finalChunks int
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
		started := time.Now()
		request := draftapp.GenerateRequest{
			StyleGuide:    guide,
			Brief:         brief,
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
		if err != nil {
			fatalf("generate draft attempt %d: %v", attempt, err)
		}
		finalElapsed = elapsed
		finalFirstChunk = firstChunk
		finalChunks = chunkCount
		writeFile(filepath.Join(outputDir, fmt.Sprintf("draft_attempt_%d.md", attempt)), result.Draft.Markdown()+"\n")
		writeJSON(filepath.Join(outputDir, fmt.Sprintf("evaluation_attempt_%d.json", attempt)), result.Evaluation)
		if result.Evaluation.Comparison.Score >= minStyleScore && len([]rune(result.Draft.Markdown())) >= minDraftRunes {
			break
		}
	}

	writeFile(filepath.Join(outputDir, "draft.md"), result.Draft.Markdown()+"\n")
	writeJSON(filepath.Join(outputDir, "evaluation.json"), result.Evaluation)
	if result.Evaluation.Comparison.Score < minStyleScore {
		fatalf("style score %.1f below scenario minimum %.1f", result.Evaluation.Comparison.Score, minStyleScore)
	}
	if runes := len([]rune(result.Draft.Markdown())); runes < minDraftRunes {
		fatalf("draft length %d below scenario minimum %d", runes, minDraftRunes)
	}

	fmt.Printf("draft generation scenario completed\n")
	fmt.Printf("passed=%v\n", result.Evaluation.Passed)
	fmt.Printf("score=%.1f\n", result.Evaluation.Comparison.Score)
	fmt.Printf("runes=%d\n", len([]rune(result.Draft.Markdown())))
	fmt.Printf("elapsed_seconds=%.2f\n", finalElapsed.Seconds())
	fmt.Printf("streaming=%v\n", streamDraft)
	if streamDraft {
		fmt.Printf("first_chunk_ms=%d\n", finalFirstChunk.Milliseconds())
		fmt.Printf("chunks=%d\n", finalChunks)
	}
	fmt.Printf("llm_base_url=%s\n", baseURL)
	fmt.Printf("llm_model=%s\n", model)
	fmt.Printf("draft=%s\n", filepath.Join(outputDir, "draft.md"))
	fmt.Printf("evaluation=%s\n", filepath.Join(outputDir, "evaluation.json"))
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

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
