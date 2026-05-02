# Next implementation cut

Date: 2026-05-03

This document translates the current open issue set into the next executable implementation sequence. The end state is unchanged: run Evo X2 Tailnet scenarios for note, Qiita, Zenn, and Cor.inc company blog with different themes, tones, and target lengths, then compare runtime, score, verification, and final output quality.

## Current state

Implemented and merged:

- [#11](https://github.com/terisuke/note_maker/issues/11) — strict Terisuke style tuning.
- [#17](https://github.com/terisuke/note_maker/issues/17) — chat transcript and editable answers.
- [#18](https://github.com/terisuke/note_maker/issues/18) — SSE streaming, heartbeat, and cancellation.
- [#19](https://github.com/terisuke/note_maker/issues/19) — editable draft Markdown and per-section regeneration.
- [#20](https://github.com/terisuke/note_maker/issues/20) — deep-dive rationale in transcript and prompt.
- [#21](https://github.com/terisuke/note_maker/issues/21) — Persona and OutputFormat domain concepts.
- [#22](https://github.com/terisuke/note_maker/issues/22) — historical source acquisition for note, Zenn, Qiita, RSS, HTML, and GitHub Markdown.
- [#23](https://github.com/terisuke/note_maker/issues/23) — format-specific prompt templates and validators.
- [#24](https://github.com/terisuke/note_maker/issues/24) — built-in `terisuke` and `cloudia` persona seeds.
- [#25](https://github.com/terisuke/note_maker/issues/25) — persona- and format-aware question templates plus the media matrix scenario.
- [#38](https://github.com/terisuke/note_maker/issues/38) — Evo X2 Tailnet OpenAI-compatible API as the primary runtime.
- [#26](https://github.com/terisuke/note_maker/issues/26) — SQLite-backed workflow store with project/article/session/draft/source snapshot schema and explicit opt-in web-app wiring via `WORKFLOW_STORE_DRIVER=sqlite`.
- [#29](https://github.com/terisuke/note_maker/issues/29) — focused handler tests for the expanded `workflow.go` surface; `go test ./internal/handlers -cover` now reaches 80.0%.
- [#57](https://github.com/terisuke/note_maker/issues/57) — live media-matrix runner and aggregate JSON/Markdown evaluator with offline planned mode by default.
- [#61](https://github.com/terisuke/note_maker/issues/61) / [PR #62](https://github.com/terisuke/note_maker/pull/62) — workflow storage mode can be inspected and switched from the settings UI; environment-locked deployments remain read-only.

Open and active:

- Memory/history umbrella: [#14](https://github.com/terisuke/note_maker/issues/14), now backed by the #26 schema work.
- History UI and readable artifacts: [#27](https://github.com/terisuke/note_maker/issues/27), [#28](https://github.com/terisuke/note_maker/issues/28).
- Browser E2E coverage: [#13](https://github.com/terisuke/note_maker/issues/13).
- Runtime evaluation: [#40](https://github.com/terisuke/note_maker/issues/40).
- Runtime evaluation sub-issues: [#70](https://github.com/terisuke/note_maker/issues/70), [#71](https://github.com/terisuke/note_maker/issues/71), [#72](https://github.com/terisuke/note_maker/issues/72), [#73](https://github.com/terisuke/note_maker/issues/73), [#74](https://github.com/terisuke/note_maker/issues/74).
- Fallback and packaging follow-up: [#36](https://github.com/terisuke/note_maker/issues/36), [#45](https://github.com/terisuke/note_maker/issues/45), [#15](https://github.com/terisuke/note_maker/issues/15).
- Runtime defect fixed by this cut: [#63](https://github.com/terisuke/note_maker/issues/63) makes the plain web-app default match the intended Evo X2 Tailnet primary path and records the 2026-05-03 draft-generation 500 root cause.
- Documentation and DDD audit: [#64](https://github.com/terisuke/note_maker/issues/64), with details in [Runtime and DDD alignment audit](../validation/runtime-ui-ddd-audit-2026-05-03.md).
- Interview usability fixed before measurement: [#66](https://github.com/terisuke/note_maker/issues/66), with details in [Issue 66 plain brief questions validation](../validation/issue-66-plain-brief-questions-2026-05-03.md).
- Style-source switching fixed before measurement: [#68](https://github.com/terisuke/note_maker/issues/68), with details in [Issue 68 media-aware style source validation](../validation/issue-68-media-aware-style-source-2026-05-03.md).

## Final evaluation target

The final integrated evaluation should use `cmd/scenario/media_matrix` as the input matrix, then run live Evo X2 Tailnet draft scenarios for:

| Case | Medium | Style | Primary source |
|---|---|---|---|
| `terisuke_note_essay` | note | reflective essay | `note:cor_instrument` |
| `cor_blog_technical_report` | Cor.inc blog | technical report | `github:Cor-Incorporated/corsweb2024/src/content/blog/ja` |
| `cor_blog_vision_sharing` | Cor.inc blog | vision sharing | `github:Cor-Incorporated/corsweb2024/src/content/blog/ja` |
| `cloudia_zenn_tutorial` | Zenn | tutorial | `zenn:cloudia` |
| `cloudia_qiita_how_to` | Qiita | practical how-to | `qiita:Cloudia_Cor_Inc` |

The homepage section case remains in the matrix as a useful format check, but the user-facing publishing targets for the full Evo X2 run are note, Qiita, Zenn, and the company blog.

Each live run must record:

- endpoint and whether it was primary or fallback,
- model per phase,
- elapsed seconds,
- generated runes,
- style score and failed metrics,
- final verification result,
- output path and scenario case id.

## Before the full Evo X2 media run

The previous prerequisites are in place, but the 2026-05-03 live result exposed a missing layer in the validation plan. A draft-only media matrix cannot prove that the revised question templates are usable, because it starts from completed `ArticleBrief` fixtures.

The runtime stabilization work is now split under epic #40:

| Order | Issue | Purpose | Done when |
|---:|---|---|---|
| 1 | [#70](https://github.com/terisuke/note_maker/issues/70) | Add an interview-template scenario | note/Cor blog/Zenn/Qiita/homepage questions and generated briefs differ by mode and remain small enough to answer |
| 2 | [#71](https://github.com/terisuke/note_maker/issues/71) | Preserve failed draft artifacts | unusable drafts still write raw output, failure JSON, elapsed time, endpoint, and model |
| 3 | [#72](https://github.com/terisuke/note_maker/issues/72) | Add bounded format repair | preamble leakage and Zenn/Qiita notation leakage get one strict repair retry without relaxing validators |
| 4 | [#73](https://github.com/terisuke/note_maker/issues/73) | Split scenario gates by output format | homepage uses short HTML gates while long-form media keep strict length/style gates |
| 5 | [#74](https://github.com/terisuke/note_maker/issues/74) | Re-run staged Evo X2 validation | one previously failing medium passes first, then the full note/Qiita/Zenn/Cor blog live matrix is rerun |

## Parallel implementation plan

Use subagents with disjoint write scopes:

| Lane | Issue | Subagent role | Write scope | Done when |
|---|---|---|---|---|
| A | [#70](https://github.com/terisuke/note_maker/issues/70) | Template scenario worker | `cmd/scenario/*`, `internal/domain/brief/*`, validation docs | question-template usability is measured before draft-only live runs |
| B | [#71](https://github.com/terisuke/note_maker/issues/71) / [#72](https://github.com/terisuke/note_maker/issues/72) | Draft recovery worker | `internal/application/draft/*`, `internal/domain/article/*`, scenario output paths | failed drafts are diagnosable and recoverable format errors get one repair attempt |
| C | [#73](https://github.com/terisuke/note_maker/issues/73) | Scenario gate worker | `cmd/scenario/*`, validation docs | long-form and homepage gates are explicit and recorded |
| D | [#27](https://github.com/terisuke/note_maker/issues/27) / [#28](https://github.com/terisuke/note_maker/issues/28) | History/artifact UI worker | `static/*`, read APIs for projects/sessions/drafts once exposed | persona/session picker and human-readable brief/style cards use persisted state |
| E | [#13](https://github.com/terisuke/note_maker/issues/13) | Browser E2E worker | browser tests and fixtures | persona/format switching, edit/fork, streaming, regenerate-section, and legacy localStorage migration are covered |

Lanes A, B, and C can run in parallel if their write scopes stay separate. Lane D/E can continue in parallel when they do not need the same frontend files.

## Recommended order

1. Implement #70 first. This proves the revised questions and generated briefs before any more expensive live draft runs.
2. Implement #71/#72/#73 in parallel where possible. These directly address the failures observed on 2026-05-03.
3. Run one bounded Evo X2 live case from a previously failing medium, not the already-passing note case.
4. Start or continue #27/#28 so expensive live outputs can be viewed and reused from the web app.
5. Run the full note/Qiita/Zenn/company-blog matrix under #74, then update #40 with the aggregate.
6. Keep #36/#45 as fallback/runtime P2 work and #15 as packaging after persistence/history are usable.

## Why not run the full Evo X2 matrix now?

The source and prompt matrix is ready, but full Evo X2 draft generation is expensive and can take 20+ minutes per run. The 2026-05-03 full run also showed that draft-only evaluation can miss whether interview templates are actually usable. The better sequence is to prove the question-to-brief layer first, preserve failed outputs, repair recoverable format mistakes, then use #40/#74 to evaluate one varied failing slice before the full comparison table.
