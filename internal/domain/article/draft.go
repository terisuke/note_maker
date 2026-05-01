package article

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	thinkingBlockPattern = regexp.MustCompile(`(?s)<\|channel\>thought.*?<channel\|>`)
	codeFencePattern     = regexp.MustCompile("(?s)^```(?:markdown|md)?\\s*(.*?)\\s*```$")
)

// Draft represents the final Markdown article body that can be pasted into Note.
type Draft struct {
	markdown string
}

// NewDraft normalizes and validates generated Markdown.
func NewDraft(raw string) (Draft, error) {
	markdown := normalizeDraft(raw)
	if markdown == "" {
		return Draft{}, fmt.Errorf("draft is empty")
	}
	if !strings.HasPrefix(markdown, "# ") {
		return Draft{}, fmt.Errorf("draft must start with a level-1 Markdown title")
	}
	if strings.Contains(markdown, "```") {
		return Draft{}, fmt.Errorf("draft must not wrap the article in code fences")
	}
	if strings.Contains(markdown, "以下") && strings.Contains(markdown, "下書き") && strings.Index(markdown, "# ") > 20 {
		return Draft{}, fmt.Errorf("draft appears to contain preamble before the article")
	}
	return Draft{markdown: markdown}, nil
}

// Markdown returns the normalized Markdown body.
func (d Draft) Markdown() string {
	return d.markdown
}

func normalizeDraft(raw string) string {
	text := strings.TrimSpace(raw)
	text = thinkingBlockPattern.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") && !strings.HasPrefix(text, "```markdown") && !strings.HasPrefix(text, "```md") {
		return text
	}
	if match := codeFencePattern.FindStringSubmatch(text); len(match) == 2 {
		text = strings.TrimSpace(match[1])
	}
	if idx := strings.Index(text, "# "); idx > 0 {
		preamble := strings.TrimSpace(text[:idx])
		if looksLikePreamble(preamble) && canDropPreamble(preamble) {
			text = strings.TrimSpace(text[idx:])
		}
	}
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)
	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
			cleaned = append(cleaned, "")
			continue
		}
		blank = false
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func canDropPreamble(text string) bool {
	if !strings.Contains(text, "```") {
		return true
	}
	return strings.Contains(text, "```markdown") || strings.Contains(text, "```md")
}

func looksLikePreamble(text string) bool {
	if text == "" {
		return true
	}
	preambles := []string{
		"以下",
		"承知しました",
		"もちろんです",
		"作成しました",
		"こちら",
	}
	for _, preamble := range preambles {
		if strings.Contains(text, preamble) {
			return true
		}
	}
	return len([]rune(text)) < 40
}
