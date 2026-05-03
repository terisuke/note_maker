package article

import (
	"strings"
	"testing"
)

func TestNewDraftNormalizesPasteReadyMarkdown(t *testing.T) {
	raw := "承知しました。以下です。\n\n```markdown\n# タイトル\n\n本文です。\n\n\n## 見出し\n内容です。\n```"
	draft, err := NewDraft(raw)
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}
	want := "# タイトル\n\n本文です。\n\n## 見出し\n内容です。"
	if draft.Markdown() != want {
		t.Fatalf("unexpected markdown:\n%s", draft.Markdown())
	}
}

func TestNewDraftForFormatAllowsTechnicalFormats(t *testing.T) {
	zenn := "---\ntitle: \"Goで試す\"\nemoji: \"🧪\"\ntype: \"tech\"\ntopics: [\"go\", \"test\"]\npublished: false\n---\n\n## 実装\n\n```go\nfmt.Println(\"ok\")\n```"
	draft, err := NewDraftForFormat(zenn, "zenn_article")
	if err != nil {
		t.Fatalf("zenn draft: %v", err)
	}
	if draft.Markdown() != zenn {
		t.Fatalf("unexpected draft:\n%s", draft.Markdown())
	}

	homepage := "<section><h2>AI活用</h2><p>小さく試せる導入文です。</p></section>"
	if _, err := NewDraftForFormat(homepage, "homepage_section"); err != nil {
		t.Fatalf("homepage draft: %v", err)
	}
}

func TestNewDraftForFormatDoesNotTreatFrontmatterBodyAsPreamble(t *testing.T) {
	zenn := "---\n" +
		"title: \"Goで試す\"\n" +
		"emoji: \"🧪\"\n" +
		"type: \"tech\"\n" +
		"topics: [\"go\", \"test\"]\n" +
		"published: false\n" +
		"---\n\n" +
		"本文では以下の下書きを検証する。\n\n" +
		"## 実装\n\n" +
		":::message\n補足\n:::"
	if _, err := NewDraftForFormat(zenn, "zenn_article"); err != nil {
		t.Fatalf("frontmatter format should not be rejected as assistant preamble: %v", err)
	}
}

func TestNewDraftForFormatDropsAssistantPreambleBeforeFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		formatID string
		raw      string
		want     string
	}{
		{
			name:     "markdown blog",
			formatID: "markdown_blog",
			want: "---\n" +
				"title: \"AI開発の知見\"\n" +
				"description: \"検証結果を共有する\"\n" +
				"pubDate: 2026-05-03\n" +
				"author: \"Terisuke\"\n" +
				"category: \"engineering\"\n" +
				"tags: [\"AI\", \"検証\"]\n" +
				"lang: \"ja\"\n" +
				"featured: false\n" +
				"isDraft: true\n" +
				"---\n\n" +
				"# AI開発の知見\n\n" +
				"## 検証\n\n本文です。",
		},
		{
			name:     "zenn",
			formatID: "zenn_article",
			want: "---\n" +
				"title: \"Goで試す\"\n" +
				"emoji: \"🧪\"\n" +
				"type: \"tech\"\n" +
				"topics: [\"go\", \"test\"]\n" +
				"published: false\n" +
				"---\n\n" +
				"## 実装\n\n" +
				":::message\n補足\n:::\n",
		},
		{
			name:     "qiita",
			formatID: "qiita_article",
			want: "---\n" +
				"title: \"Qiitaで試す\"\n" +
				"tags:\n" +
				"  - Go\n" +
				"  - AI\n" +
				"---\n\n" +
				"## 手順\n\n" +
				":::note info\n補足\n:::\n\n" +
				"```diff_go\n+fmt.Println(1)\n```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := "承知しました。以下、下書きです。\n\n" + tt.want
			draft, err := NewDraftForFormat(raw, tt.formatID)
			if err != nil {
				t.Fatalf("new draft: %v", err)
			}
			if draft.Markdown() != strings.TrimSpace(tt.want) {
				t.Fatalf("unexpected markdown:\n%s", draft.Markdown())
			}
		})
	}
}

func TestNewDraftForFormatUnwrapsYAMLFrontmatterFence(t *testing.T) {
	raw := "```yaml\n" +
		"---\n" +
		"title: \"Qiitaで試す\"\n" +
		"tags:\n" +
		"  - Go\n" +
		"  - AI\n" +
		"---\n" +
		"```\n" +
		"# Qiitaで試す\n\n" +
		"## 手順\n\n" +
		":::note info\n補足\n:::\n\n" +
		"```diff_go\n+fmt.Println(1)\n```"

	draft, err := NewDraftForFormat(raw, "qiita_article")
	if err != nil {
		t.Fatalf("new draft: %v", err)
	}

	if strings.HasPrefix(draft.Markdown(), "```yaml") {
		t.Fatalf("frontmatter fence was not unwrapped:\n%s", draft.Markdown())
	}
	if !strings.HasPrefix(draft.Markdown(), "---\n") {
		t.Fatalf("frontmatter was not preserved at document start:\n%s", draft.Markdown())
	}
}

func TestNewDraftRejectsNonArticleOutput(t *testing.T) {
	if _, err := NewDraft("承知しました。記事を書きます。"); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := NewDraft("```text\n# title\n```"); err == nil {
		t.Fatal("expected code fence validation error")
	}
}
