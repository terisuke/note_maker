package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
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

	baseURL := envOrDefault("LLAMACPP_BASE_URL", "http://127.0.0.1:8081/v1")
	model := envOrDefault("LLAMACPP_MODEL", "gemma4:31b")
	client, err := llamacpp.NewClient(baseURL, model, &http.Client{Timeout: 12 * time.Minute})
	if err != nil {
		fatalf("create local llm client: %v", err)
	}
	service := draftapp.NewService(client)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()

	result, err := service.Generate(ctx, draftapp.GenerateRequest{
		StyleGuide:    guide,
		Brief:         brief,
		AuthorProfile: profile,
	})
	if err != nil {
		fatalf("generate draft: %v", err)
	}

	writeFile(filepath.Join(outputDir, "draft.md"), result.Draft.Markdown()+"\n")
	writeJSON(filepath.Join(outputDir, "evaluation.json"), result.Evaluation)

	fmt.Printf("draft generation scenario completed\n")
	fmt.Printf("passed=%v\n", result.Evaluation.Passed)
	fmt.Printf("score=%.1f\n", result.Evaluation.Comparison.Score)
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

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
