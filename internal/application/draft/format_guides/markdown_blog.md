# Cor.inc company blog Markdown guide

Use this guide only for `markdown_blog` output.

## Target file

The generated article should be copy-pasteable into:

`corsweb2024/src/content/blog/ja/{slug}.md`

## Required frontmatter

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

## Category

Use exactly one of:

- `ai`
- `engineering`
- `founder`
- `lab`

## Body

- Body starts with `# タイトル` or `## タイトル`.
- Use direct company-blog prose: mainly `だ`, `である`, `する`.
- Prefer 60-170 lines when the brief allows it.
- Code fences must specify a language.
- Include concrete implementation decisions, verification results, or operational lessons.

## Editorial purpose

Use the company blog for:

- company technical knowledge reports,
- implementation decision records,
- internal or employee-facing vision sharing,
- practical lessons that show Cor.inc's capability.

Do not write it like a broad-reader note essay unless the brief explicitly asks for that.
