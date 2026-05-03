package article

import (
	"fmt"
	"regexp"
	"strings"

	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
)

var (
	thinkingBlockPattern = regexp.MustCompile(`(?s)<\|channel\>thought.*?<channel\|>`)
	codeFencePattern     = regexp.MustCompile("(?s)^```(?:markdown|md)?\\s*(.*?)\\s*```$")
)

// Draft represents the final Markdown article body that can be pasted into Note.
type Draft struct {
	markdown string
}

// NewDraft normalizes and validates generated Markdown for note_article.
func NewDraft(raw string) (Draft, error) {
	return NewDraftForFormat(raw, outputformat.IDNoteArticle)
}

// NewDraftForFormat normalizes and validates generated output for a publishing target.
func NewDraftForFormat(raw, formatID string) (Draft, error) {
	markdown := normalizeDraft(raw)
	if markdown == "" {
		return Draft{}, fmt.Errorf("draft is empty")
	}
	format, ok := outputformat.DefaultRegistry().Get(formatID)
	if !ok {
		return Draft{}, fmt.Errorf("unknown output format %q", formatID)
	}
	if err := format.Validator.Validate(markdown); err != nil {
		return Draft{}, err
	}
	if !strings.HasPrefix(markdown, "---\n") && strings.Contains(markdown, "以下") && strings.Contains(markdown, "下書き") && strings.Index(markdown, "# ") > 20 {
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
	text = unwrapFencedFrontmatter(text)
	if strings.HasPrefix(text, "```") && !strings.HasPrefix(text, "```markdown") && !strings.HasPrefix(text, "```md") {
		return text
	}
	if match := codeFencePattern.FindStringSubmatch(text); len(match) == 2 {
		text = strings.TrimSpace(match[1])
	}
	droppedPreambleWithFence := false
	if idx := frontmatterStartIndex(text); idx > 0 {
		preamble := strings.TrimSpace(text[:idx])
		if looksLikePreamble(preamble) && canDropPreamble(preamble) {
			droppedPreambleWithFence = strings.Contains(preamble, "```")
			text = strings.TrimSpace(text[idx:])
		}
	}
	if idx := strings.Index(text, "# "); idx > 0 && !strings.HasPrefix(text, "---\n") {
		preamble := strings.TrimSpace(text[:idx])
		if looksLikePreamble(preamble) && canDropPreamble(preamble) {
			droppedPreambleWithFence = strings.Contains(preamble, "```")
			text = strings.TrimSpace(text[idx:])
		}
	}
	if droppedPreambleWithFence {
		text = strings.TrimSuffix(text, "```")
	}
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

func unwrapFencedFrontmatter(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) < 4 {
		return text
	}
	opener := strings.ToLower(strings.TrimSpace(lines[0]))
	if opener != "```yaml" && opener != "```yml" {
		return text
	}
	closing := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "```" {
			closing = i
			break
		}
	}
	if closing < 0 {
		return text
	}
	frontmatter := strings.TrimSpace(strings.Join(lines[1:closing], "\n"))
	if !strings.HasPrefix(frontmatter, "---\n") {
		return text
	}
	rest := strings.TrimSpace(strings.Join(lines[closing+1:], "\n"))
	if rest == "" {
		return frontmatter
	}
	return frontmatter + "\n\n" + rest
}

func frontmatterStartIndex(text string) int {
	if strings.HasPrefix(text, "---\n") {
		return 0
	}
	if idx := strings.Index(text, "\n---\n"); idx >= 0 {
		return idx + 1
	}
	return -1
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
