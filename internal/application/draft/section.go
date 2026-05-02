package draft

import (
	"context"
	"fmt"
	"strings"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

// MarkdownSection is one editable top-level article section headed by "## ".
type MarkdownSection struct {
	Anchor  string `json:"anchor"`
	Heading string `json:"heading"`
	Content string `json:"content"`
	Start   int    `json:"start"`
	End     int    `json:"end"`
}

// RegenerateSectionRequest asks the model to replace one section only.
type RegenerateSectionRequest struct {
	GenerateRequest
	DraftMarkdown string
	SectionAnchor string
}

// RegenerateSectionResult carries the replacement and the full updated draft.
type RegenerateSectionResult struct {
	Section              MarkdownSection `json:"section"`
	ReplacementMarkdown  string          `json:"replacement_markdown"`
	UpdatedDraftMarkdown string          `json:"updated_draft_markdown"`
}

// FindMarkdownSection finds a "## " section by exact heading or normalized anchor.
func FindMarkdownSection(markdown, anchor string) (MarkdownSection, error) {
	anchor = normalizeSectionAnchor(anchor)
	if anchor == "" {
		return MarkdownSection{}, fmt.Errorf("section anchor is required")
	}
	sections := MarkdownSections(markdown)
	for _, section := range sections {
		if normalizeSectionAnchor(section.Anchor) == anchor || normalizeSectionAnchor(section.Heading) == anchor {
			return section, nil
		}
	}
	return MarkdownSection{}, fmt.Errorf("section %q was not found", anchor)
}

// MarkdownSections returns all "## " sections, ending each before the next "## ".
func MarkdownSections(markdown string) []MarkdownSection {
	type heading struct {
		offset int
		line   string
	}
	var headings []heading
	offset := 0
	for _, line := range strings.SplitAfter(markdown, "\n") {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\n"))
		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			headings = append(headings, heading{offset: offset, line: trimmed})
		}
		offset += len(line)
	}
	sections := make([]MarkdownSection, 0, len(headings))
	for i, item := range headings {
		end := len(markdown)
		if i+1 < len(headings) {
			end = headings[i+1].offset
		}
		headingText := strings.TrimSpace(strings.TrimPrefix(item.line, "## "))
		sections = append(sections, MarkdownSection{
			Anchor:  sectionAnchor(headingText),
			Heading: headingText,
			Content: markdown[item.offset:end],
			Start:   item.offset,
			End:     end,
		})
	}
	return sections
}

// ReplaceMarkdownSection returns a full draft with only section's byte range replaced.
func ReplaceMarkdownSection(markdown string, section MarkdownSection, replacement string) (string, error) {
	if section.Start < 0 || section.End < section.Start || section.End > len(markdown) {
		return "", fmt.Errorf("section range is invalid")
	}
	replacement = strings.TrimSpace(replacement)
	if replacement == "" {
		return "", fmt.Errorf("replacement section is empty")
	}
	if !strings.HasPrefix(strings.TrimSpace(replacement), "## ") {
		replacement = "## " + section.Heading + "\n\n" + replacement
	}
	if !strings.HasSuffix(replacement, "\n") {
		replacement += "\n"
	}
	replacementSections := MarkdownSections(replacement)
	if len(replacementSections) != 1 {
		return "", fmt.Errorf("replacement must contain exactly one h2 section")
	}
	if normalizeSectionAnchor(replacementSections[0].Heading) != normalizeSectionAnchor(section.Heading) {
		return "", fmt.Errorf("replacement heading %q does not match target heading %q", replacementSections[0].Heading, section.Heading)
	}
	return markdown[:section.Start] + replacement + markdown[section.End:], nil
}

// RegenerateSection rewrites one "## " section while preserving the rest byte-for-byte.
func (s *Service) RegenerateSection(ctx context.Context, req RegenerateSectionRequest) (RegenerateSectionResult, error) {
	if s.generator == nil {
		return RegenerateSectionResult{}, fmt.Errorf("text generator is required")
	}
	if err := validateRequest(req.GenerateRequest); err != nil {
		return RegenerateSectionResult{}, err
	}
	section, err := FindMarkdownSection(req.DraftMarkdown, req.SectionAnchor)
	if err != nil {
		return RegenerateSectionResult{}, err
	}
	persona := req.Persona
	if persona.ID == "" {
		persona, _ = personadomain.DefaultRegistry().Get(req.Brief.PersonaID)
	}
	format := req.OutputFormat
	if format.ID == "" {
		var ok bool
		format, ok = outputformat.DefaultRegistry().Get(req.Brief.OutputFormatID)
		if !ok {
			format, _ = outputformat.DefaultRegistry().Get(outputformat.IDNoteArticle)
		}
	}
	prompt := BuildSectionRegenerationPrompt(req.StyleGuide, req.Brief, req.AuthorProfile, persona, format, req.DraftMarkdown, section)
	replacement, err := s.generator.Generate(ctx, prompt)
	if err != nil {
		return RegenerateSectionResult{}, fmt.Errorf("regenerate section with local llm: %w", err)
	}
	updated, err := ReplaceMarkdownSection(req.DraftMarkdown, section, replacement)
	if err != nil {
		return RegenerateSectionResult{}, err
	}
	if _, err := articledomain.NewDraftForFormat(updated, format.ID); err != nil {
		return RegenerateSectionResult{}, fmt.Errorf("regenerated draft is invalid: %w", err)
	}
	return RegenerateSectionResult{
		Section:              section,
		ReplacementMarkdown:  strings.TrimSpace(replacement),
		UpdatedDraftMarkdown: updated,
	}, nil
}

func sectionAnchor(heading string) string {
	return normalizeSectionAnchor(heading)
}

func normalizeSectionAnchor(value string) string {
	value = strings.TrimSpace(strings.TrimPrefix(value, "## "))
	value = strings.ToLower(value)
	replacer := strings.NewReplacer(" ", "-", "　", "-", "_", "-", "/", "-", ":", "", "：", "", "?", "", "？", "", "!", "", "！", "", "`", "", "\"", "", "'", "")
	value = replacer.Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	return strings.Trim(value, "-")
}
