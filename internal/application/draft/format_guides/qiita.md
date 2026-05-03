# Qiita Markdown guide

Use this guide only for `qiita_article` output.

## Required frontmatter

The final article must start with `---` as the first characters. Do not wrap the frontmatter or the full article in ```yaml, ```markdown, or any other code fence.

```yaml
---
title: "記事タイトル"
tags:
  - Go
  - AI
---
```

`tags` should use discoverable technology names.

## Structure

- Emphasize reproduction: environment, steps, code, result, troubleshooting, references.
- Keep explanations concrete enough that another engineer can try them immediately.

## Qiita-specific notation

- Filename code fences: ````rb:app.rb````.
- Diff highlighting: ````diff_ruby````.
- Tips and warnings:
  - `:::note info`
  - `:::note warn`
  - `:::note alert`
- Collapsible details: `<details><summary>...</summary>...</details>`.
- Math block: ````math````.
- Inline math: prefer Qiita's backtick dollar notation, e.g. `$`x^2 + y^2 = 1`$`.
- Markdown tables are allowed. HTML tables are acceptable only when Markdown cannot express the table.

## Avoid

- Zenn `:::message`.
- Zenn `:::message alert`.
- Zenn `:::details`.
- Zenn `@[card](...)`.
- Zenn diff fences such as `diff ts`.
