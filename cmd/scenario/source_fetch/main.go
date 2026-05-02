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

	sourcefetch "github.com/teradakousuke/note_maker/internal/infrastructure/source"
)

const defaultOutputDir = "tmp/source_fetch"

func main() {
	if os.Getenv("RUN_SOURCE_FETCH_SCENARIO") != "1" {
		fatalf("set RUN_SOURCE_FETCH_SCENARIO=1 to run public source fetch scenario")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	outputDir := envOrDefault("SCENARIO_OUTPUT_DIR", defaultOutputDir)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		fatalf("create output dir: %v", err)
	}

	limit := 1
	fetcher := sourcefetch.NewAuthorStyleFetcherWithClient(&http.Client{Timeout: 20 * time.Second})
	selectors := map[string]string{
		"note":  envOrDefault("SOURCE_FETCH_NOTE", "note:cor_instrument"),
		"zenn":  envOrDefault("SOURCE_FETCH_ZENN", "zenn:zenn"),
		"qiita": envOrDefault("SOURCE_FETCH_QIITA", "qiita:Qiita"),
		"rss":   envOrDefault("SOURCE_FETCH_RSS", "rss:https://zenn.dev/zenn/feed"),
	}

	for name, selector := range selectors {
		started := time.Now()
		articles, err := fetcher.FetchUserLatestArticles(ctx, selector, limit)
		if err != nil {
			fatalf("fetch %s (%s): %v", name, selector, err)
		}
		path := filepath.Join(outputDir, name+".json")
		writeJSON(path, articles)
		fmt.Printf("%s selector=%s articles=%d elapsed_seconds=%.2f output=%s\n", name, selector, len(articles), time.Since(started).Seconds(), path)
	}
}

func writeJSON(path string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fatalf("encode %s: %v", path, err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
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
