# Issue 19 section regeneration validation

Date: 2026-05-02

## Scope

Validate [#19](https://github.com/terisuke/note_maker/issues/19): editable draft Markdown plus one-section regeneration.

The runtime scenario used the prior Phase A4 company-blog draft so this run differs from the earlier note-style runs while still measuring the same Evo X2 Tailnet primary path.

## Runtime

- Base URL: `http://evo-x2.tailb30e58.ts.net/v1`
- Draft model: `gemma4:31b`
- Verification model: `gemma4:latest`
- Persona: `terisuke`
- Output format: `markdown_blog`
- Source draft: `tmp/draft_generation_issue20/draft.md`
- Target section: `## 実装と検証：再現性の数値化`
- Output directory: `tmp/section_regeneration_issue19`

## Results

| Metric | Value |
|---|---:|
| Section regeneration elapsed | 167.33s |
| Lightweight verification elapsed | 23.14s |
| Updated draft length | 4011 runes |
| Replacement length | 1336 runes |
| Style score | 72.1 |
| Lightweight verification | PASS |
| Non-target prefix unchanged | true |
| Non-target suffix unchanged | true |

Comparison to Issue 20 company-blog draft run:

| Run | Elapsed | Runes | Score | Verification |
|---|---:|---:|---:|---|
| Issue 20 full draft | 439.86s | 3675 | 72.3 | PASS |
| Issue 19 section regenerate | 167.33s + 23.14s verify | 4011 | 72.1 | PASS |

## Observations

- The section replacement preserved every byte before and after the target `## ` subtree.
- The score stayed in the same band as the previous company-blog run (`72.1` vs `72.3`), which supports treating this as medium/style variance rather than a #19 regression.
- The strict Terisuke note-style threshold still fails for company-blog drafts because paragraph and sentence lengths are longer than the note baseline. This remains a cross-medium baseline issue, not a section-regeneration bug.
- The first scenario attempt used a shortened anchor (`実装と検証`) and correctly failed because anchors require the exact heading/normalized anchor.
- A second attempt used the correct heading but hit the client default `LLM_TIMEOUT_SECONDS=180` before Evo X2 returned response headers. The successful run set both the scenario timeout and LLM client timeout to 600 seconds.
- The lightweight verifier summary contains a typo (`Tailsale互換API`). The verifier still returned PASS, so this is recorded as a quality signal for future verifier-prompt tightening rather than a blocker for #19.

## Commands

```bash
RUN_LOCAL_LLM_SCENARIO=1 \
LLM_BASE_URL=http://evo-x2.tailb30e58.ts.net/v1 \
LLM_TIMEOUT_SECONDS=600 \
DRAFT_LLM_MODEL=gemma4:31b \
VERIFY_LLM_MODEL=gemma4:latest \
SCENARIO_OUTPUT_DIR=tmp/section_regeneration_issue19 \
SCENARIO_SECTION_ANCHOR=実装と検証：再現性の数値化 \
SCENARIO_SECTION_TIMEOUT_SECONDS=600 \
go run ./cmd/scenario/section_regeneration
```
