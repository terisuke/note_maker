# note Markdown guide

Use this guide only for `note_article` output.

## Goal

Generate paste-ready Japanese Markdown that converts cleanly in the note editor.

## Allowed notation

- `# タイトル` on the first line for the article title.
- `## 大見出し`.
- `### 小見出し`.
- `> 引用`.
- `- 箇条書き`.
- `1. 番号付きリスト`.
- `**太字**`.
- `~~取り消し線~~`.
- `---` for a horizontal rule.
- Plain fenced code blocks only when code is truly needed.

## Avoid

- YAML frontmatter.
- HTML blocks.
- Markdown tables.
- Footnotes.
- Math blocks or inline math.
- Zenn notation: `:::message`, `:::details`, `@[card](...)`.
- Qiita notation: `:::note info`, `:::note warn`, `:::note alert`.
- Filename code fences such as `ts:src/main.ts`.
- Diff-specific code fences.

## Writing bias

Use note for broad-reader essays: technical experiments, personal observations, and ideas that should attract readers beyond a narrow implementation context.
