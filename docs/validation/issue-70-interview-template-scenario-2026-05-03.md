# Issue 70 interview-template scenario validation - 2026-05-03

## Goal

Add an offline preflight before Evo X2 live media runs. The preflight validates that every built-in persona/output-format pair composes the expected interview template and can produce a simulated `ArticleBrief` without network access, LLM calls, or draft generation.

## Implementation

- Added `cmd/scenario/interview_template`.
- The scenario covers both seeded personas across all five output formats:
  - `terisuke`
  - `cloudia`
  - `note_article`
  - `markdown_blog`
  - `zenn_article`
  - `qiita_article`
  - `homepage_section`
- Each case starts the real `brief.InterviewService` with a nil follow-up generator, answers the composed fixed questions with deterministic scripted inputs, and lets the domain fallback generate deep-dive prompts.
- Each case writes:
  - the completed interview session JSON,
  - the simulated `ArticleBrief` JSON,
  - template and brief checks in `report.json`.
- The scenario intentionally does not call draft generation, live media matrix code, runtime LLM clients, or network-backed source fetchers.

## Verification

Commands:

```bash
go test ./cmd/scenario/interview_template
go run ./cmd/scenario/interview_template
```

Result:

- Targeted scenario tests passed.
- Deterministic scenario run passed with:
  - `offline_only=true`
  - `cases=10`
  - `question_templates=10`
  - `briefs=10`
- Generated artifacts:
  - `tmp/interview_template/report.json`
  - `tmp/interview_template/cases.md`
  - `tmp/interview_template/briefs/*.json`
  - `tmp/interview_template/sessions/*.json`

## Pre-live gate

Before running `cmd/scenario/live_media_matrix` in live mode, run:

```bash
go run ./cmd/scenario/interview_template
```

Proceed only when every case in `tmp/interview_template/cases.md` reports `passed`.
