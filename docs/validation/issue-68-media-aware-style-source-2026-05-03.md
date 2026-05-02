# Issue 68 media-aware style source validation - 2026-05-03

## Problem

The interview question template changed with persona and output format, but the style analysis panel still looked and behaved like a note-only input. That made the company blog mode misleading: users could select `会社ブログ`, see company-blog questions, and still derive style from note unless they manually knew the hidden source selector syntax.

## Implementation

- Renamed the UI input from `Noteユーザー名` to `文体ソース`.
- The UI now defaults the source selector from the selected persona and output format:
  - `terisuke + note_article` -> `note:cor_instrument`
  - `terisuke + markdown_blog` -> `github:Cor-Incorporated/corsweb2024/src/content/blog/ja`
  - `terisuke + homepage_section` -> `github:Cor-Incorporated/corsweb2024/src/content/blog/ja`
  - `cloudia + zenn_article` -> `zenn:cloudia`
  - `cloudia + qiita_article` -> `qiita:Cloudia_Cor_Inc`
- `POST /api/author-style/analyze` now accepts `persona_id`, `output_format_id`, and `source_selector`.
- If the user leaves the source selector blank, the server picks the same persona/format-aware default source.
- Persona presets are now format-aware: the generated preset guide includes output-format notes and uses the appropriate source URL.
- The Cor company blog persona source now uses the canonical GitHub Markdown path as its `Ref`, so it can be passed directly to the source router.

## Verification

Commands:

```bash
go test ./internal/handlers ./internal/domain/persona ./internal/infrastructure/source
node --check static/js/script.js
git diff --check
```

Result:

- All checks passed.

## Remaining

Before #40 live Evo X2 scoring, do a browser sanity check that switching the output format changes both:

- the `文体ソース` default,
- the brief question template.
