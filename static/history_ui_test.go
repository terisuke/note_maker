package static_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestHistoryUIContract(t *testing.T) {
	index, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	script, err := os.ReadFile("js/script.js")
	if err != nil {
		t.Fatalf("read script: %v", err)
	}
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(index))
	if err != nil {
		t.Fatalf("parse index: %v", err)
	}

	for _, selector := range []string{
		"#history-persona-select",
		"#history-style-select",
		"#history-session-select",
		"#refresh-history-btn",
		"#open-history-btn",
		"#clear-history-selection-btn",
		"#history-status",
		"#style-guide-card",
		"#brief-card",
	} {
		if document.Find(selector).Length() != 1 {
			t.Fatalf("selector %s count = %d, want 1", selector, document.Find(selector).Length())
		}
	}

	source := string(script)
	for _, want := range []string{
		"const historyEndpoint = '/api/workflow/artifacts'",
		"loadWorkflowHistory",
		"normalizeWorkflowHistory",
		"renderStyleGuideCard",
		"renderBriefCard",
		"requestJSON(`${historyEndpoint}?",
		"/api/author-style/",
		"/api/brief-sessions/",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("script missing %q", want)
		}
	}
}
