# Issue 23/24 format and persona seed validation

Date: 2026-05-02

## Scope

This validation covers:

- [#23](https://github.com/terisuke/note_maker/issues/23) format-specific prompt templates and validators.
- [#24](https://github.com/terisuke/note_maker/issues/24) seeded persona library for `terisuke` and `cloudia`.

It deliberately does not cover source-fetcher generalisation ([#22](https://github.com/terisuke/note_maker/issues/22)), SQLite persistence ([#26](https://github.com/terisuke/note_maker/issues/26)), or handler coverage expansion ([#29](https://github.com/terisuke/note_maker/issues/29)).

## What changed

- Strengthened `internal/domain/format` validators:
  - note rejects HTML blocks in addition to platform-specific notation and filename/diff fences.
  - Zenn requires boolean `published`, inline `topics` with at most five items, and rejects HTML details.
  - Qiita rejects empty `tags`.
  - homepage sections reject frontmatter, code fences, and Markdown headings.
- Added unit coverage for the stricter format cases.
- Added unit coverage proving seeded personas reference registered default formats and expected source kinds.
- Added deterministic scenario command `cmd/scenario/format_persona_seed`.

## Scenario

Command:

```sh
go run ./cmd/scenario/format_persona_seed
```

Result:

```text
format/persona seed scenario completed
personas=2
formats=5
samples=5
summary=tmp/format_persona_seed/summary.json
```

The scenario validates all registered format samples through `article.NewDraftForFormat`, checks embedded format-guide injection through `draft.BuildPromptForMode`, writes prompt hints for both seeded personas, and exercises these persona/format samples:

- `terisuke_note`
- `terisuke_blog`
- `cloudia_zenn`
- `cloudia_qiita`
- `terisuke_homepage`

## Acceptance status

- Each validator has positive and negative unit coverage: done.
- Each registered format has an embedded guide injected into the draft prompt: done.
- Same writing brief can be represented under visibly different target surfaces: covered by deterministic scenario samples.
- Seed library includes `terisuke` and `cloudia`, with distinct default formats, source bundles, first-person hints, title patterns, and anti-patterns: done.

Live source-derived guide rebuilding for Zenn/Qiita/RSS remains dependent on [#22](https://github.com/terisuke/note_maker/issues/22), so it is not part of this validation.
