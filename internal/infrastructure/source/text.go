package source

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func normalizeWhitespace(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func normalizeParagraphs(value string) string {
	lines := strings.Split(strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n"), "\n")
	normalized := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		line = normalizeWhitespace(line)
		if line == "" {
			if !blank {
				normalized = append(normalized, "")
			}
			blank = true
			continue
		}
		blank = false
		normalized = append(normalized, line)
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func htmlToParagraphText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(value))
	if err != nil {
		return normalizeParagraphs(value)
	}
	parts := make([]string, 0)
	doc.Find("h1,h2,h3,h4,p,li,blockquote,pre,code").Each(func(_ int, s *goquery.Selection) {
		text := normalizeWhitespace(s.Text())
		if text != "" {
			parts = append(parts, text)
		}
	})
	if len(parts) == 0 {
		return normalizeParagraphs(doc.Text())
	}
	return normalizeParagraphs(strings.Join(parts, "\n\n"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
