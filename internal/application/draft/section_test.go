package draft

import (
	"context"
	"strings"
	"testing"

	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
)

func TestFindMarkdownSectionByHeadingAndAnchor(t *testing.T) {
	markdown := "# Title\n\nLead\n\n## 実装\n\nA\n\n### Detail\n\nB\n\n## 検証結果\n\nC\n"
	section, err := FindMarkdownSection(markdown, "実装")
	if err != nil {
		t.Fatalf("find section: %v", err)
	}
	if section.Heading != "実装" {
		t.Fatalf("heading = %q", section.Heading)
	}
	if !strings.Contains(section.Content, "### Detail") {
		t.Fatalf("section should include nested headings: %q", section.Content)
	}
	if strings.Contains(section.Content, "## 検証結果") {
		t.Fatalf("section should stop before next h2: %q", section.Content)
	}
	if _, err := FindMarkdownSection(markdown, "検証結果"); err != nil {
		t.Fatalf("find by normalized anchor: %v", err)
	}
}

func TestReplaceMarkdownSectionPreservesOtherSections(t *testing.T) {
	markdown := "# Title\n\nIntro\n\n## 実装\n\nold\n\n## 検証\n\nkeep\n"
	section, err := FindMarkdownSection(markdown, "実装")
	if err != nil {
		t.Fatalf("find section: %v", err)
	}
	updated, err := ReplaceMarkdownSection(markdown, section, "## 実装\n\nnew\n")
	if err != nil {
		t.Fatalf("replace section: %v", err)
	}
	want := "# Title\n\nIntro\n\n## 実装\n\nnew\n## 検証\n\nkeep\n"
	if updated != want {
		t.Fatalf("updated markdown:\n%q\nwant:\n%q", updated, want)
	}
}

func TestReplaceMarkdownSectionRejectsMultipleH2Sections(t *testing.T) {
	markdown := "# Title\n\n## 実装\n\nold\n\n## 検証\n\nkeep\n"
	section, err := FindMarkdownSection(markdown, "実装")
	if err != nil {
		t.Fatalf("find section: %v", err)
	}
	_, err = ReplaceMarkdownSection(markdown, section, "## 実装\n\nnew\n\n## 検証\n\nrewritten")
	if err == nil {
		t.Fatal("expected multiple h2 replacement to fail")
	}
}

func TestRegenerateSectionReplacesOnlyTargetSection(t *testing.T) {
	markdown := "# Title\n\nIntro\n\n## 実装\n\nold\n\n## 検証\n\nkeep\n"
	service := NewService(staticSectionGenerator{content: "## 実装\n\nnew content\n"})
	result, err := service.RegenerateSection(context.Background(), RegenerateSectionRequest{
		GenerateRequest: GenerateRequest{
			StyleGuide: authordomain.WritingStyleGuide{
				ID:                   "guide-1",
				ProfileID:            "profile-1",
				PreferredFirstPerson: "僕",
				Markdown:             "- practical",
				RecurringThemes:      []string{"AI"},
				ParagraphRhythm:      "短め",
				SentenceRhythm:       "明快",
				HeadingGuidance:      "具体的",
				QuoteGuidance:        "必要な引用だけ使う",
				OpeningPatterns:      []string{"体験から始める"},
				ConclusionPatterns:   []string{"次の行動で締める"},
			},
			Brief: briefdomain.ArticleBrief{
				StyleProfileID:        "profile-1",
				PersonaID:             personadomain.IDTerisuke,
				OutputFormatID:        outputformat.IDNoteArticle,
				Theme:                 "section regenerate",
				Reader:                "writers",
				MustInclude:           "implementation",
				TargetLengthStructure: "1200字",
			},
			AuthorProfile: authordomain.AuthorStyleProfile{
				ID:                   "profile-1",
				Version:              "author-style/v1",
				Source:               authordomain.AuthorSource{Articles: []authordomain.SourceArticle{{ID: "article-1"}}},
				ArticleCount:         1,
				PreferredFirstPerson: "僕",
				Metrics: articledomain.StyleProfile{
					CharCount:        1000,
					ParagraphCount:   10,
					SentenceCount:    20,
					FirstPersonCount: 3,
				},
			},
			Persona:      mustPersona(t, personadomain.IDTerisuke),
			OutputFormat: outputformat.DefaultRegistry().MustGet(outputformat.IDNoteArticle),
		},
		DraftMarkdown: markdown,
		SectionAnchor: "実装",
	})
	if err != nil {
		t.Fatalf("regenerate section: %v", err)
	}
	if !strings.Contains(result.UpdatedDraftMarkdown, "## 実装\n\nnew content\n") {
		t.Fatalf("replacement missing: %q", result.UpdatedDraftMarkdown)
	}
	if !strings.Contains(result.UpdatedDraftMarkdown, "## 検証\n\nkeep\n") {
		t.Fatalf("non-target section changed: %q", result.UpdatedDraftMarkdown)
	}
}

type staticSectionGenerator struct {
	content string
}

func (g staticSectionGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	return g.content, nil
}

func mustPersona(t *testing.T, id string) personadomain.Persona {
	t.Helper()
	persona, ok := personadomain.DefaultRegistry().Get(id)
	if !ok {
		t.Fatalf("persona %q was not found", id)
	}
	return persona
}
