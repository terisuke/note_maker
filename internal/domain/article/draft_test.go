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

func TestNewDraftRejectsNonArticleOutput(t *testing.T) {
	if _, err := NewDraft("承知しました。記事を書きます。"); err == nil {
		t.Fatal("expected validation error")
	}
	if _, err := NewDraft("```text\n# title\n```"); err == nil {
		t.Fatal("expected code fence validation error")
	}
}
