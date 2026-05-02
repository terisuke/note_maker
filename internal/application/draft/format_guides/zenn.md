# Zenn Markdown guide

Use this guide only for `zenn_article` output.

## Required frontmatter

```yaml
---
title: "記事タイトル"
emoji: "📝"
type: "tech"
topics: ["go", "ai"]
published: false
---
```

- `type` must be `tech` or `idea`.
- `topics` should contain up to 5 short technical tags.

## Structure

- Start body headings at `##`.
- Prefer reproducible technical explanations: premise, environment, steps, code, result, pitfalls, references.

## Zenn-specific notation

- Filename code fences: ````ts:src/main.ts````.
- Diff highlighting: ````diff ts````.
- Tips: `:::message`.
- Warnings: `:::message alert`.
- Collapsible details: `:::details`.
- Link cards: `@[card](https://example.com)`.
- Gist embeds: `@[gist](https://gist.github.com/...)`.
- Math: `$$` blocks or inline `$...$`.
- Footnotes: `[^1]`.
- Markdown tables are allowed.

## Avoid

- Qiita `:::note info`, `:::note warn`, `:::note alert`.
- Qiita `diff_language` fences such as `diff_ruby`.
- HTML `<details>` for collapsible blocks.
