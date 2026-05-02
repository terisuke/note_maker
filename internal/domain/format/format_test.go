package format

import "testing"

func TestDefaultRegistryContainsFormats(t *testing.T) {
	registry := DefaultRegistry()
	for _, id := range []string{IDNoteArticle, IDMarkdownBlog, IDZennArticle, IDQiitaArticle, IDHomepageSection} {
		if _, ok := registry.Get(id); !ok {
			t.Fatalf("missing format %s", id)
		}
	}
}

func TestValidators(t *testing.T) {
	tests := []struct {
		name      string
		validator Validator
		valid     string
		invalid   string
	}{
		{
			name:      "note",
			validator: NoteValidator{},
			valid:     "# タイトル\n\n## 見出し\n\n本文\n\n```go\nfmt.Println(1)\n```",
			invalid:   "# タイトル\n\n:::message\nZennの補足\n:::",
		},
		{
			name:      "blog",
			validator: MarkdownBlogValidator{},
			valid: "---\n" +
				"title: \"AI開発の知見\"\n" +
				"description: \"AI駆動開発で得た実装判断と検証結果を共有する\"\n" +
				"pubDate: 2026-05-02\n" +
				"author: \"Terisuke\"\n" +
				"category: \"engineering\"\n" +
				"tags: [\"AI\", \"開発\", \"検証\"]\n" +
				"lang: \"ja\"\n" +
				"featured: false\n" +
				"isDraft: true\n" +
				"---\n\n" +
				"# AI開発の知見\n\n" +
				"```go\nfmt.Println(1)\n```",
			invalid: "## 実装\n\n```go\nfmt.Println(1)\n```",
		},
		{
			name:      "zenn",
			validator: ZennValidator{},
			valid:     "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n## 本文\n\n:::message\n補足\n:::\n\n```diff go\n+fmt.Println(1)\n```",
			invalid:   "---\ntitle: T\n---\n本文",
		},
		{
			name:      "qiita",
			validator: QiitaValidator{},
			valid:     "---\ntitle: \"T\"\ntags: [{name: Go, versions: [\"1.24\"]}]\n---\n\n## 本文\n\n:::note warn\n注意\n:::\n\n```diff_go\n+fmt.Println(1)\n```",
			invalid:   "---\ntitle: T\n---\n本文",
		},
		{
			name:      "homepage",
			validator: HomepageSectionValidator{},
			valid:     "<section><h2>見出し</h2><p>本文</p></section>",
			invalid:   "# 見出し\n\n本文",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.validator.Validate(tt.valid); err != nil {
				t.Fatalf("valid rejected: %v", err)
			}
			if err := tt.validator.Validate(tt.invalid); err == nil {
				t.Fatal("invalid accepted")
			}
		})
	}
}

func TestPlatformSpecificNotationIsNotMixed(t *testing.T) {
	zennWithQiitaNote := "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n:::note info\nQiita記法\n:::"
	if err := (ZennValidator{}).Validate(zennWithQiitaNote); err == nil {
		t.Fatal("expected zenn validator to reject Qiita note notation")
	}

	qiitaWithZennMessage := "---\ntitle: \"T\"\ntags: [{name: Go}]\n---\n\n:::message\nZenn記法\n:::"
	if err := (QiitaValidator{}).Validate(qiitaWithZennMessage); err == nil {
		t.Fatal("expected qiita validator to reject Zenn message notation")
	}

	noteWithFilenameFence := "# タイトル\n\n```go:main.go\nfmt.Println(1)\n```"
	if err := (NoteValidator{}).Validate(noteWithFilenameFence); err == nil {
		t.Fatal("expected note validator to reject filename code fence")
	}

	noteWithHTML := "# タイトル\n\n<section><h2>見出し</h2><p>本文</p></section>"
	if err := (NoteValidator{}).Validate(noteWithHTML); err == nil {
		t.Fatal("expected note validator to reject HTML blocks")
	}
}

func TestMarkdownBlogRejectsUnsupportedCorBlogMetadata(t *testing.T) {
	invalid := `---
title: "Vision"
description: "社員向けのビジョン共有"
pubDate: 2026-05-02
author: "Terisuke"
category: "culture"
tags: ["Vision"]
lang: "en"
featured: false
---

# Vision`

	if err := (MarkdownBlogValidator{}).Validate(invalid); err == nil {
		t.Fatal("expected invalid company blog metadata to be rejected")
	}
}

func TestZennValidatorEnforcesMetadataShape(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "published is boolean",
			input: "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: \"no\"\n---\n\n## 本文",
		},
		{
			name:  "topics are inline list",
			input: "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics:\n  - go\npublished: false\n---\n\n## 本文",
		},
		{
			name:  "topics capped at five",
			input: "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\", \"ai\", \"llm\", \"zenn\", \"test\", \"cli\"]\npublished: false\n---\n\n## 本文",
		},
		{
			name:  "html details is qiita style",
			input: "---\ntitle: \"T\"\nemoji: \"📝\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n<details><summary>詳細</summary></details>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := (ZennValidator{}).Validate(tt.input); err == nil {
				t.Fatal("expected invalid zenn article to be rejected")
			}
		})
	}
}

func TestQiitaValidatorRejectsEmptyTags(t *testing.T) {
	invalid := "---\ntitle: \"T\"\ntags: []\n---\n\n## 本文"
	if err := (QiitaValidator{}).Validate(invalid); err == nil {
		t.Fatal("expected empty Qiita tags to be rejected")
	}
}

func TestHomepageSectionRejectsMarkdownScaffolding(t *testing.T) {
	tests := []string{
		"---\ntitle: \"T\"\n---\n<section><h2>見出し</h2><p>本文</p></section>",
		"<section><h2>見出し</h2><p>本文</p></section>\n\n```html\n<p>余分</p>\n```",
		"<section><h2>見出し</h2><p>本文</p>\n## Markdown見出し\n</section>",
	}
	for _, input := range tests {
		if err := (HomepageSectionValidator{}).Validate(input); err == nil {
			t.Fatalf("expected homepage section to reject:\n%s", input)
		}
	}
}
