package article

import "testing"

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

func TestNewDraftRejectsNonArticleOutput(t *testing.T) {
	if _, err := NewDraft("承知しました。記事を書きます。"); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := NewDraft("```text\n# title\n```"); err == nil {
		t.Fatal("expected code fence validation error")
	}
}
