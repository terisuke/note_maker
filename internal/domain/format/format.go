package format

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	IDNoteArticle     = "note_article"
	IDMarkdownBlog    = "markdown_blog"
	IDZennArticle     = "zenn_article"
	IDQiitaArticle    = "qiita_article"
	IDHomepageSection = "homepage_section"
)

// Validator checks whether generated output fits a publishing target.
type Validator interface {
	Validate(markdown string) error
}

// OutputFormat describes one target writing surface.
type OutputFormat struct {
	ID             string    `json:"id"`
	DisplayName    string    `json:"display_name"`
	Description    string    `json:"description"`
	PromptFragment string    `json:"prompt_fragment"`
	Validator      Validator `json:"-"`
	AllowsCode     bool      `json:"allows_code"`
	RequiresMeta   bool      `json:"requires_meta"`
}

// Registry resolves output formats by id.
type Registry struct {
	formats map[string]OutputFormat
	order   []string
}

// DefaultRegistry returns the built-in format set.
func DefaultRegistry() Registry {
	formats := []OutputFormat{
		{
			ID:          IDNoteArticle,
			DisplayName: "note",
			Description: "note.com向け。貼り付け変換で崩れにくい最小Markdown。frontmatterなし。",
			PromptFragment: strings.TrimSpace(`note.comにそのまま貼れる日本語Markdown記事として書く。
1行目は必ず # タイトル。frontmatter、HTML、Zenn/Qiita独自記法は使わない。
noteエディタで変換されやすい記法だけを使う: ## 大見出し、### 小見出し、> 引用、- 箇条書き、1. 番号付きリスト、**太字**、~~取り消し線~~、--- 区切り線。
コード例が必要な場合だけ通常のコードブロックを使う。ファイル名付きコードフェンス、diff指定、メッセージブロック、脚注、数式、表は避ける。
面白い技術的な試み、考え方、体験からの気づきを、広い読者を引き寄せる読み物としてまとめる。
体験、違和感、解釈、読者への提案を自然につなぐ。`),
			Validator:  NoteValidator{},
			AllowsCode: true,
		},
		{
			ID:          IDMarkdownBlog,
			DisplayName: "会社ブログ",
			Description: "cor-jp.com / corsweb2024向け。Astro content blogのfrontmatter必須、日本語記事。",
			PromptFragment: strings.TrimSpace(`Cor.inc会社ブログ向けの日本語Markdown記事として書く。
先頭にcorsweb2024のAstro content用YAML frontmatterを必ず置く。必須項目は title, description, pubDate, author, category, tags, lang, featured。
frontmatter例:
---
title: "記事タイトル"
description: "50-100文字程度の説明"
pubDate: 2026-05-02
author: "Terisuke"
category: "engineering"
tags: ["AI", "開発", "知見"]
lang: "ja"
featured: false
isDraft: true
---
category は ai / engineering / founder / lab のいずれかにする。lang は必ず "ja"。
本文は # または ## の記事タイトルから始め、60-170行を目安に、断定口調「だ」「である」「する」を基本にする。
会社ブログでは、自社の技術的知見の報告、実装判断、検証結果、または社員へのビジョン共有を主目的にする。
コードフェンスには必ず言語名を付ける。背景、構成、課題、解決、実測データ、学び、次の行動を明確にする。`),
			Validator:    MarkdownBlogValidator{},
			AllowsCode:   true,
			RequiresMeta: true,
		},
		{
			ID:          IDZennArticle,
			DisplayName: "Zenn",
			Description: "Zenn向け。YAML frontmatter必須、tech/idea、topics、publishedを含める。",
			PromptFragment: strings.TrimSpace(`Zenn記事として書く。
先頭にYAML frontmatterを置き、title, emoji, type, topics, published を必ず含める。
type は tech または idea。topics は5個以内。
本文の見出しは ## から始める。コードブロックは「ts:src/main.ts」のように言語名と必要ならファイル名を使う。差分は「diff ts」のように書く。
補足はZenn独自の :::message、注意は :::message alert、折りたたみは :::details を使う。リンクカードや外部埋め込みは @[card](URL) などZenn形式を使う。
数式は $$ ブロックまたは $...$、脚注は [^1]、表はMarkdownテーブルを使える。Qiitaの :::note は使わない。
本文では技術的な前提、手順、コード例、つまずきやすい点を明確にする。`),
			Validator:    ZennValidator{},
			AllowsCode:   true,
			RequiresMeta: true,
		},
		{
			ID:          IDQiitaArticle,
			DisplayName: "Qiita",
			Description: "Qiita向け。title/tagsのfrontmatter、コード例、手順重視。",
			PromptFragment: strings.TrimSpace(`Qiita記事として書く。
先頭にYAML frontmatterを置き、title と tags を必ず含める。
本文は再現手順、環境、コード例、結果、参考リンクを重視する。読者がすぐ試せる粒度にする。
コードブロックは「rb:app.rb」のように言語名と必要ならファイル名を使う。差分はQiita形式の「diff_ruby」のように書く。
補足・警告はQiitaの :::note info / :::note warn / :::note alert を使う。折りたたみはHTMLの <details><summary>...</summary>...</details> を使い、Zennの :::details は使わない。
数式は math コードブロック、インライン数式はQiitaのバッククォート付きドル記法を優先する。表はMarkdownテーブルまたは必要時だけHTML tableを使う。`),
			Validator:    QiitaValidator{},
			AllowsCode:   true,
			RequiresMeta: true,
		},
		{
			ID:          IDHomepageSection,
			DisplayName: "ホームページHTML",
			Description: "Webページ埋め込み向け。HTML section、h2、p、CTAを出力。",
			PromptFragment: strings.TrimSpace(`Webサイトに埋め込むHTMLセクションとして書く。
MarkdownではなくHTMLだけを返す。少なくとも <section>, <h2>, <p> を含める。
必要に応じてCTAの <a> を入れ、会社サイトの落ち着いた説明文として使える密度にする。`),
			Validator: HomepageSectionValidator{},
		},
	}
	registry := Registry{formats: make(map[string]OutputFormat, len(formats)), order: make([]string, 0, len(formats))}
	for _, item := range formats {
		registry.formats[item.ID] = item
		registry.order = append(registry.order, item.ID)
	}
	return registry
}

// List returns registered formats in stable UI order.
func (r Registry) List() []OutputFormat {
	result := make([]OutputFormat, 0, len(r.order))
	for _, id := range r.order {
		if format, ok := r.formats[id]; ok {
			result = append(result, format)
		}
	}
	return result
}

// Get returns a format, defaulting empty ids to note_article.
func (r Registry) Get(id string) (OutputFormat, bool) {
	id = NormalizeID(id)
	format, ok := r.formats[id]
	return format, ok
}

// MustGet returns a format or panics. Use only for package defaults/tests.
func (r Registry) MustGet(id string) OutputFormat {
	format, ok := r.Get(id)
	if !ok {
		panic("unknown output format: " + id)
	}
	return format
}

// NormalizeID returns the default format when id is empty.
func NormalizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return IDNoteArticle
	}
	return id
}

type NoteValidator struct{}

func (NoteValidator) Validate(markdown string) error {
	markdown = strings.TrimSpace(markdown)
	if !strings.HasPrefix(markdown, "# ") {
		return fmt.Errorf("note article must start with a level-1 Markdown title")
	}
	if hasFrontMatter(markdown) {
		return fmt.Errorf("note article must not contain frontmatter")
	}
	if containsAny(markdown, []string{":::message", ":::note", ":::details", "@[card]", "@[gist]", "$$", "<details", "<table"}) {
		return fmt.Errorf("note article must not contain platform-specific extended Markdown")
	}
	if regexp.MustCompile("(?m)^```\\S+[:_]").MatchString(markdown) {
		return fmt.Errorf("note article must not contain filename or diff-specific code fences")
	}
	return nil
}

type MarkdownBlogValidator struct{}

func (MarkdownBlogValidator) Validate(markdown string) error {
	markdown = strings.TrimSpace(markdown)
	if !hasFrontMatter(markdown) {
		return fmt.Errorf("company blog article requires YAML frontmatter")
	}
	frontmatter := frontMatter(markdown)
	requiredKeys := []string{"title:", "description:", "pubDate:", "author:", "category:", "tags:", "lang:", "featured:"}
	for _, key := range requiredKeys {
		if !strings.Contains(frontmatter, key) {
			return fmt.Errorf("company blog frontmatter missing %s", strings.TrimSuffix(key, ":"))
		}
	}
	if !regexp.MustCompile(`(?m)^category:\s*"?\b(ai|engineering|founder|lab)\b"?`).MatchString(frontmatter) {
		return fmt.Errorf("company blog category must be ai, engineering, founder, or lab")
	}
	if !regexp.MustCompile(`(?m)^lang:\s*"?ja"?`).MatchString(frontmatter) {
		return fmt.Errorf("company blog lang must be ja")
	}
	if !regexp.MustCompile(`(?m)^tags:\s*\[.+\]`).MatchString(frontmatter) {
		return fmt.Errorf("company blog tags must be an inline YAML array")
	}
	body := bodyAfterFrontMatter(markdown)
	if !strings.HasPrefix(body, "# ") && !strings.HasPrefix(body, "## ") {
		return fmt.Errorf("company blog body must start with # title or ## title")
	}
	if hasCodeFenceWithoutLanguage(body) {
		return fmt.Errorf("company blog code fences must specify a language")
	}
	return nil
}

type ZennValidator struct{}

func (ZennValidator) Validate(markdown string) error {
	if !hasFrontMatter(markdown) {
		return fmt.Errorf("zenn article requires YAML frontmatter")
	}
	frontmatter := frontMatter(markdown)
	for _, key := range []string{"title:", "emoji:", "type:", "topics:", "published:"} {
		if !strings.Contains(frontmatter, key) {
			return fmt.Errorf("zenn frontmatter missing %s", strings.TrimSuffix(key, ":"))
		}
	}
	if !regexp.MustCompile(`(?m)^type:\s*"?\b(tech|idea)\b"?`).MatchString(frontmatter) {
		return fmt.Errorf("zenn type must be tech or idea")
	}
	if strings.Contains(markdown, ":::note") {
		return fmt.Errorf("zenn article must use :::message, not Qiita :::note")
	}
	if strings.Contains(markdown, "```diff_") {
		return fmt.Errorf("zenn diff code fences use `diff language`, not diff_language")
	}
	return nil
}

type QiitaValidator struct{}

func (QiitaValidator) Validate(markdown string) error {
	if !hasFrontMatter(markdown) {
		return fmt.Errorf("qiita article requires YAML frontmatter")
	}
	frontmatter := frontMatter(markdown)
	for _, key := range []string{"title:", "tags:"} {
		if !strings.Contains(frontmatter, key) {
			return fmt.Errorf("qiita frontmatter missing %s", strings.TrimSuffix(key, ":"))
		}
	}
	if strings.Contains(markdown, ":::message") || strings.Contains(markdown, ":::details") || strings.Contains(markdown, "@[card]") {
		return fmt.Errorf("qiita article must not contain Zenn-specific notation")
	}
	if regexp.MustCompile("(?m)^```diff\\s+\\w+").MatchString(markdown) {
		return fmt.Errorf("qiita diff code fences use diff_language, not `diff language`")
	}
	return nil
}

type HomepageSectionValidator struct{}

func (HomepageSectionValidator) Validate(markdown string) error {
	html := strings.TrimSpace(strings.ToLower(markdown))
	if strings.HasPrefix(html, "#") {
		return fmt.Errorf("homepage section must be HTML, not Markdown")
	}
	for _, tag := range []string{"<section", "<h2", "<p"} {
		if !strings.Contains(html, tag) {
			return fmt.Errorf("homepage section must contain %s", tag)
		}
	}
	return nil
}

func hasFrontMatter(markdown string) bool {
	return strings.HasPrefix(strings.TrimSpace(markdown), "---\n")
}

func frontMatter(markdown string) string {
	markdown = strings.TrimSpace(markdown)
	if !strings.HasPrefix(markdown, "---\n") {
		return ""
	}
	rest := strings.TrimPrefix(markdown, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	return rest[:end]
}

func bodyAfterFrontMatter(markdown string) string {
	markdown = strings.TrimSpace(markdown)
	if !strings.HasPrefix(markdown, "---\n") {
		return markdown
	}
	rest := strings.TrimPrefix(markdown, "---\n")
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[end+len("\n---"):])
}

func hasCodeFenceWithoutLanguage(markdown string) bool {
	inFence := false
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "```") {
			continue
		}
		if !inFence {
			if strings.TrimSpace(strings.TrimPrefix(trimmed, "```")) == "" {
				return true
			}
			inFence = true
			continue
		}
		inFence = false
	}
	return false
}

func containsAny(value string, needles []string) bool {
	lower := strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}
