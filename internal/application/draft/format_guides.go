package draft

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed format_guides/*.md
var embeddedFormatGuides embed.FS

const maxFormatGuideRunes = 5000

var formatGuideFiles = map[string]string{
	"note_article":     "format_guides/note.md",
	"markdown_blog":    "format_guides/markdown_blog.md",
	"zenn_article":     "format_guides/zenn.md",
	"qiita_article":    "format_guides/qiita.md",
	"homepage_section": "format_guides/homepage_section.md",
}

func formatGuideMarkdown(formatID string) string {
	path, ok := formatGuideFiles[strings.TrimSpace(formatID)]
	if !ok {
		return ""
	}
	content, err := fs.ReadFile(embeddedFormatGuides, path)
	if err != nil {
		return ""
	}
	return truncateRunes(strings.TrimSpace(string(content)), maxFormatGuideRunes)
}
