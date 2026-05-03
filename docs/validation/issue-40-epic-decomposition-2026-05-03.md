# Issue 40 epic decomposition

Date: 2026-05-03

## Context

The full Tailnet Evo X2 media-matrix run completed against the primary OpenAI-compatible API path, but only `terisuke_note_essay` passed. The failures were useful, but they also showed that the scenario plan needed one more layer before implementation:

- The draft-only live matrix starts from completed `ArticleBrief` fixtures, so it cannot prove that the revised fixed questions are actually easier to answer.
- Early unusable drafts were discarded before enough diagnostic artifacts were written.
- Some failures were recoverable format errors: assistant preamble leakage and Zenn/Qiita notation leakage.
- The homepage section was judged with long-form article assumptions.

## Epic

[#40](https://github.com/terisuke/note_maker/issues/40) is the runtime stabilization epic. The staged validation criteria for the current publishing-target scope were met on 2026-05-03 by the final #74 full Tailnet Evo X2 matrix run.

## Sub-issues

| Order | Issue | Scope | Why it exists |
|---:|---|---|---|
| 1 | [#70](https://github.com/terisuke/note_maker/issues/70) | Interview-template scenario | Proves question-template usability and medium-specific brief output before Evo X2 draft generation. |
| 2 | [#71](https://github.com/terisuke/note_maker/issues/71) | Failed draft artifacts | Preserves raw output and runtime metrics when validation fails before a draft is accepted. |
| 3 | [#72](https://github.com/terisuke/note_maker/issues/72) | Bounded format repair | Gives recoverable format mistakes one strict retry without weakening validators. |
| 4 | [#73](https://github.com/terisuke/note_maker/issues/73) | Output-format-specific gates | Separates long-form article gates from homepage short HTML gates. |
| 5 | [#74](https://github.com/terisuke/note_maker/issues/74) | Staged Evo X2 rerun | Runs template/offline/live validation in the correct order and records results back to #40. |

## Implementation order

1. Implement #70.
2. Implement #71/#72/#73 in parallel only if write scopes stay disjoint.
3. Run one live Evo X2 case from a previously failing medium.
4. Run the full note/Qiita/Zenn/Cor blog matrix only after the scenario and diagnostic gaps are closed.

## Full matrix result

Final command scope:

```sh
LIVE_MEDIA_MATRIX_CASES=terisuke_note_essay,cor_blog_technical_report,cor_blog_vision_sharing,cloudia_zenn_tutorial,cloudia_qiita_how_to \
RUN_LIVE_MEDIA_MATRIX=1 \
SCENARIO_STREAM_DRAFT=1 \
LLM_BASE_URL=http://evo-x2.tailb30e58.ts.net/v1 \
DRAFT_LLM_MODEL=gemma4:31b \
VERIFY_LLM_MODEL=gemma4:latest \
go run ./cmd/scenario/live_media_matrix
```

Result:

| Case | Status | Attempt | Seconds | Score / min | Runes / min | Verification | Structural gate |
|---|---|---:|---:|---:|---:|---|---|
| `terisuke_note_essay` | passed | 1 | `107.86` | `90.7 / 82.0` | `2849 / 2800` | passed | passed |
| `cor_blog_technical_report` | passed | 1 | `152.34` | `81.4 / 80.0` | `3329 / 2200` | passed | passed |
| `cor_blog_vision_sharing` | passed | 1 | `113.98` | `89.5 / 80.0` | `3156 / 1600` | passed | passed |
| `cloudia_zenn_tutorial` | passed | 1 | `121.14` | `86.4 / 82.0` | `5641 / 1800` | passed | passed |
| `cloudia_qiita_how_to` | passed | 2 | `114.75` | `82.2 / 82.0` | `3737 / 1400` | passed | passed |

Aggregate:

- `5/5` publishing-target rows passed.
- Average seconds: `122.01`.
- Average score: `86.0`.
- Average runes: `3742`.
- Runtime: Evo X2 Ollama primary, `http://evo-x2.tailb30e58.ts.net/v1`, `gemma4:31b`.
- Artifacts: `tmp/media_matrix/live/aggregate.json`, `tmp/media_matrix/live/aggregate.md`.

Closure interpretation:

- #70-#74 are complete for the current note/Qiita/Zenn/Cor blog publishing-target scope.
- #40 can close after the PR lands and the final aggregate is linked from the issue.
- `cor_homepage_section` remains a separate short HTML format check and is intentionally outside this closure gate.

## Docs updated

- [ADR 0002](../adrs/0002-multi-persona-multi-format-extension.md)
- [Issue and ADR guardrails](../implementation-plans/issue-adr-guardrails.md)
- [Next implementation cut](../implementation-plans/next-implementation-cut.md)
