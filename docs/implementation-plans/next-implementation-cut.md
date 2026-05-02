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
- Fallback and packaging follow-up: [#36](https://github.com/terisuke/note_maker/issues/36), [#45](https://github.com/terisuke/note_maker/issues/45), [#15](https://github.com/terisuke/note_maker/issues/15).
- Runtime defect fixed by this cut: [#63](https://github.com/terisuke/note_maker/issues/63) makes the plain web-app default match the intended Evo X2 Tailnet primary path and records the 2026-05-03 draft-generation 500 root cause.
- Documentation and DDD audit: [#64](https://github.com/terisuke/note_maker/issues/64), with details in [Runtime and DDD alignment audit](../validation/runtime-ui-ddd-audit-2026-05-03.md).
- Interview usability fixed before measurement: [#66](https://github.com/terisuke/note_maker/issues/66), with details in [Issue 66 plain brief questions validation](../validation/issue-66-plain-brief-questions-2026-05-03.md).

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

The three prerequisites before running the full multi-medium Evo X2 evaluation are now mostly in place:

1. **Persistence first**: #26 adds SQLite storage for sessions, briefs, source snapshots, drafts, verification, and section-regeneration versions. #61/#62 makes the storage driver visible and switchable from the settings UI, so users do not have to choose it only through make/env setup.
2. **Handler coverage gate**: #29 raises `internal/handlers` coverage to 80.0%, including SSE, edit/fork, template, regenerate-section, and SQLite driver selection paths.
3. **Scenario ownership**: #57 adds the reusable live runner/aggregate evaluator. #40 remains the owner for actual Evo X2 Tailnet quality results.

## Parallel implementation plan

Use subagents with disjoint write scopes:

| Lane | Issue | Subagent role | Write scope | Done when |
|---|---|---|---|---|
| A | [#27](https://github.com/terisuke/note_maker/issues/27) / [#28](https://github.com/terisuke/note_maker/issues/28) | History/artifact UI worker | `static/*`, read APIs for projects/sessions/drafts once exposed | persona/session picker and human-readable brief/style cards use persisted state |
| B | [#13](https://github.com/terisuke/note_maker/issues/13) | Browser E2E worker | browser tests and fixtures | persona/format switching, edit/fork, streaming, regenerate-section, and legacy localStorage migration are covered |
| C | [#40](https://github.com/terisuke/note_maker/issues/40) | Scenario metrics worker | `docs/validation/*`, live run artifacts | media-matrix live runner records endpoint/model/elapsed/score/runes/verification in aggregate JSON/Markdown for actual Evo X2 runs |

Lane A and Lane B can run immediately in parallel. Lane C can start by implementing offline/resumable runner mechanics now, but the full multi-case Evo X2 run should wait until Lane A provides persistence or until the user explicitly wants a one-off artifact-file run.

## Recommended order

1. Browser-check the #66 smaller question flow with at least note and one technical format. Do this before spending Evo X2 runtime.
2. Run one bounded Evo X2 live case through #57 and attach it to #40 to verify the runner with real latency/score data.
3. Start #27 and #28 in parallel so persisted sessions, guides, and draft artifacts become visible in the web app.
4. Start #13 once the history/artifact UI has enough stable browser surface.
5. Run the full note/Qiita/Zenn/company-blog media matrix under #40.
6. Keep #36/#45 as fallback/runtime P2 work and #15 as packaging after persistence/history are usable.

## Why not run the full Evo X2 matrix now?

The source and prompt matrix is ready, but full Evo X2 draft generation is expensive and can take 20+ minutes per run. Running all media cases before persistence would produce useful files but not durable product memory. The better sequence is to make the system capable of storing those expensive results, then use #40 to evaluate one varied slice per phase and finally run the full comparison table.
