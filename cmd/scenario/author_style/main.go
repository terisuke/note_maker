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

	"github.com/teradakousuke/note_maker/internal/domain/author"
	notenote "github.com/teradakousuke/note_maker/internal/infrastructure/note"
)

const defaultOutputDir = "tmp/author_style"

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	username := envOrDefault("TERISUKE_NOTE_USERNAME", "cor_instrument")
	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	limit := 6

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	client := notenote.NewAPIClient(&http.Client{Timeout: 20 * time.Second})
	articles, err := client.FetchUserArticles(ctx, username, limit)
	if err != nil {
		fatalf("fetch note API articles: %v", err)
	}

	now := time.Now().UTC()
	source := author.AuthorSource{
		Username:  username,
		Articles:  author.SourceArticlesFromArticles(articles, now),
		FetchedAt: now,
	}
	profile, err := author.BuildAuthorStyleProfile(source, articles)
	if err != nil {
		fatalf("build profile: %v", err)
	}
	guide, err := author.BuildWritingStyleGuide(profile)
	if err != nil {
		fatalf("build guide: %v", err)
	}

	writeJSON(filepath.Join(outputDir, "profile.json"), profile)
	writeJSON(filepath.Join(outputDir, "guide.json"), guide)
	writeFile(filepath.Join(outputDir, "guide.md"), guide.Markdown+"\n")

	fmt.Printf("author style scenario completed\n")
	fmt.Printf("username=%s\n", username)
	fmt.Printf("articles=%d\n", len(articles))
	fmt.Printf("profile_id=%s\n", profile.ID)
	fmt.Printf("guide_id=%s\n", guide.ID)
	fmt.Printf("profile=%s\n", filepath.Join(outputDir, "profile.json"))
	fmt.Printf("guide=%s\n", filepath.Join(outputDir, "guide.md"))
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
