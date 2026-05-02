# Publishing platform research

Date: 2026-05-02

This note captures the practical acquisition and generation assumptions behind the first Persona / OutputFormat implementation.

## Sources checked

- Zenn official RSS article: `https://zenn.dev/zenn/articles/zenn-feed-rss`
- Zenn official CLI guide: `https://zenn.dev/zenn/articles/zenn-cli-guide`
- Zenn official Markdown guide: `https://zenn.dev/zenn/articles/markdown-guide`
- Qiita API v2 docs: `https://qiita.com/api/v2/docs`
- Qiita official Markdown cheat sheet: `https://qiita.com/Qiita/items/c686397e4a0f4f11683d`
- note Markdown summary article: `https://note.com/akihamitsuki/n/nc7fdbff15bc8`
- note help center RSS article: `https://www.help-note.com/hc/ja/articles/4402395202841-iframe-RSSで-自分のサイトにnoteを表示する`
- note help center full RSS article: `https://www.help-note.com/hc/ja/articles/900001001246`
- Cor.inc blog index: `https://cor-jp.com/blog/`
- Cor.inc company blog repository: `https://github.com/Cor-Incorporated/corsweb2024`
- `corsweb2024` files checked through GitHub API:
  - `BLOG_FORMAT_GUIDE.md`
  - `src/content/config.ts`
  - `src/config/categories.ts`
  - `src/content/blog/ja/ai-driven-development-workflow.md`
  - `src/content/blog/ja/complete-multilingual-blog-expansion.md`
  - `src/content/blog/ja/mcp-server-vibe-coding.md`
  - `src/content/blog/ja/google-cloud-project-migration.md`
- Public examples:
  - `https://note.com/cor_instrument`
  - `https://zenn.dev/cloudia`
  - `https://qiita.com/Cloudia_Cor_Inc`
  - `https://cor-jp.com/blog/`

## Acquisition decisions

| Platform | Stable read path | Notes |
| --- | --- | --- |
| note | `https://note.com/{user}/rss` plus existing compatibility fetcher | Official help documents RSS. Standard RSS includes excerpts; full RSS is note pro / custom-domain limited. Existing unofficial JSON endpoints stay compatibility-only. |
| Zenn | `https://zenn.dev/{user}/feed?all=1` | Zenn documents user RSS and `all=1`. Unofficial JSON endpoints exist in the wild but are not the primary contract. |
| Qiita | Qiita API v2 `GET /api/v2/users/:user_id/items` | Official API returns public item metadata and Markdown body. Unauthenticated clients are rate-limited to 60 requests/hour/IP. |
| Cor.inc blog | `https://cor-jp.com/rss.xml`, `src/content/blog/ja/*.md` in `corsweb2024`, and semantic HTML fallback | The repo is the best source for exact frontmatter and Markdown body format. Article generation should target `src/content/blog/ja/{slug}.md` first; multilingual expansion can happen later through the existing site pipeline. |

## Initial persona and format split

| Persona | Default format | Practical voice hints |
| --- | --- | --- |
| `terisuke` | `note_article` | First person `僕` / `私`; note is for broad-reader technical experiments and reflections; Cor.inc blog is for company technical knowledge reports, implementation decisions, and employee-facing vision sharing. |
| `cloudia` | `zenn_article` | First person `クラウディア` / `うち`; light Hakata-ben flavour; technical tutorial voice; exclamation-rich titles; code and step-by-step guidance. |

| OutputFormat | Minimum validator |
| --- | --- |
| `note_article` | Starts with `# `, no YAML frontmatter, no platform-specific extended Markdown. Plain code fences are allowed only when needed. |
| `markdown_blog` | Cor.inc Astro blog frontmatter is required: `title`, `description`, `pubDate`, `author`, `category`, `tags`, `lang`, `featured`. Body starts with `# ` or `## `. |
| `zenn_article` | YAML frontmatter with `title`, `emoji`, `type`, `topics`, `published`; `type` is `tech` or `idea`. |
| `qiita_article` | YAML frontmatter with `title` and `tags`; code fences allowed. |
| `homepage_section` | HTML output containing `<section>`, `<h2>`, and `<p>`. |

## Editor notation split

The output mode should also choose notation, not just prose.

| Format | Practical notation policy |
| --- | --- |
| note | Keep to the subset that paste-converts reliably in the note editor: `##`, `###`, blockquotes, `-` lists, `1.` lists, `**bold**`, `~~strike~~`, horizontal rule, and plain code fences only when necessary. Avoid frontmatter, HTML, tables, footnotes, math, file-name code fences, and Zenn/Qiita blocks. |
| Zenn | Use Zenn frontmatter and Zenn extensions: code fences may use `language:filename`; diff highlighting uses `diff language`; tips use `:::message`; warnings use `:::message alert`; collapsible sections use `:::details`; link cards use `@[card](URL)`. |
| Qiita | Use Qiita frontmatter and Qiita extensions: code fences may use `language:filename`; diff highlighting uses `diff_language`; tips/warnings use `:::note info`, `:::note warn`, `:::note alert`; collapsible sections use HTML `<details><summary>...`; math should prefer `math` code fences or inline `$` backtick notation. |

Generation and validation should reject obvious cross-platform leakage:

- note must not output `:::message`, `:::note`, `:::details`, `@[card]`, HTML tables/details, or math blocks.
- Zenn must not output Qiita `:::note` or `diff_language` fences.
- Qiita must not output Zenn `:::message`, `:::details`, `@[card]`, or `diff language` fences.

Implementation note: these notation policies are now captured as embedded Markdown guides under `internal/application/draft/format_guides/`. The final draft prompt injects only the selected format's guide, so the model sees concrete editor rules without receiving unrelated platform examples.

## UX implication

The app should not ask users to remember platform rules. The first usable version therefore exposes:

- writer selector,
- output target selector,
- source/voice summary,
- automatic default format per persona,
- technical extra questions for Zenn / Qiita,
- homepage CTA questions for HTML sections,
- format-specific draft validation before showing the result.

## Cor.inc blog generation target

The company blog is an Astro content collection. The immediate practical output should be copy-pasteable Markdown for `corsweb2024/src/content/blog/ja/{slug}.md`, not an abstract blog draft.

Required frontmatter shape:

```yaml
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
```

Categories are defined in `src/config/categories.ts`: `ai`, `engineering`, `founder`, `lab`. The older guide text omits `engineering`, but the active Astro schema imports category ids from this config, so the app should follow the config.

Practical style constraints:

- Generate Japanese first: `lang: "ja"`.
- Use direct company-blog prose: mainly `だ` / `である` / `する`.
- Keep technical knowledge reports concrete: prerequisites, commands/code, verification result, learned constraints.
- Keep vision-sharing articles useful for employees: why the decision matters, what behavior should change, and how it connects to company direction.
- Include code fences only with a language marker.
- Prefer `isDraft: true` for generated output so the pasted file is safe by default.
