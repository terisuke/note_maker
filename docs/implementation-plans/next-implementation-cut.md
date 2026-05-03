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
- [#70](https://github.com/terisuke/note_maker/issues/70)-[#73](https://github.com/terisuke/note_maker/issues/73) prerequisite slice for runtime evaluation — interview-template coverage, failed draft artifact preservation, bounded format repair, and output-format-specific scenario gates are in place for staged #74 reruns.

Open and active:

- Memory/history umbrella: [#14](https://github.com/terisuke/note_maker/issues/14), now backed by the #26 schema work.
- History UI and readable artifacts: [#27](https://github.com/terisuke/note_maker/issues/27), [#28](https://github.com/terisuke/note_maker/issues/28).
- Browser E2E coverage: [#13](https://github.com/terisuke/note_maker/issues/13).
- Runtime evaluation: [#40](https://github.com/terisuke/note_maker/issues/40).
- Runtime evaluation sub-issue still blocking #40: [#74](https://github.com/terisuke/note_maker/issues/74), now narrowed to Cloudia/Zenn style calibration plus staged live reruns.
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

## Current #74 status

The #70-#73 prerequisites are in place. The first staged #74 Tailnet rerun intentionally targeted one previously failing medium, `cloudia_zenn_tutorial`, and isolated the remaining failure:

| Layer | Result |
|---|---|
| Tailnet runtime | passed: Evo X2 OpenAI-compatible endpoint responded and streamed metrics |
| Format validation | passed: generated Markdown was accepted as Zenn output |
| Length gate | passed: `3905` runes against `1800` minimum |
| Final verification | passed with `gemma4:latest` |
| Strict style gate | failed: `73.6 / 82.0` |

The current cut fixed the evaluation reliability gap before trusting that score:

- `cmd/scenario/media_matrix` now emits case-specific style `profile.json` and `guide.json`.
- `cmd/scenario/live_media_matrix` passes those artifacts into `cmd/scenario/draft_generation`.
- Draft generation rejects profile/guide/brief style-profile mismatches.
- Final verification failure now blocks `scenario_passed`.
- Structural signals are enforced, not only reported.
- The web response includes `quality_gate` details so failed scores keep the draft visible.

The bounded Cloudia technical proofs now pass:

| Case | Seconds | First chunk | Chunks | Score / min | Runes / min | Verification |
|---|---:|---:|---:|---:|---:|---|
| `cloudia_zenn_tutorial` | `741.93` | `86456ms` | `2205` | `88.1 / 82.0` | `5040 / 1800` | passed |
| `cloudia_qiita_how_to` | `598.62` | `111645ms` | `1336` | `83.7 / 82.0` | `3318 / 1400` | passed |

The remaining #74 work is the full publishing-target matrix.

## Parallel implementation plan

Use subagents with disjoint write scopes when implementation resumes:

| Lane | Issue | Subagent role | Write scope | Done when |
|---|---|---|---|---|
| A | [#74](https://github.com/terisuke/note_maker/issues/74) | Full matrix worker | live aggregate and validation docs | note, Qiita, Zenn, and Cor blog rows all pass and record artifacts |
| D | [#27](https://github.com/terisuke/note_maker/issues/27) / [#28](https://github.com/terisuke/note_maker/issues/28) | History/artifact UI worker | `static/*`, read APIs for projects/sessions/drafts once exposed | persona/session picker and human-readable brief/style cards use persisted state |
| E | [#13](https://github.com/terisuke/note_maker/issues/13) | Browser E2E worker | browser tests and fixtures | persona/format switching, edit/fork, streaming, regenerate-section, and legacy localStorage migration are covered |

Lane A is the next expensive Evo X2 spend. Lane D/E can continue in parallel when they do not need the same frontend files.

## Recommended order

1. Run the full note/Qiita/Zenn/company-blog matrix under #74 and update #40 with the aggregate JSON/Markdown plus artifact paths.
2. Close #40 only when every publishing target records endpoint, phase models, elapsed time, runes, style score, structural signals, final verification, `quality_gate`, and artifacts with no runtime, format-validation, final-verification, structural-signal, or strict style-gate failures. Homepage can remain a separate format check and is not part of the #40 closure gate.
3. Continue #27/#28 and #13 in parallel as product-readiness work. Keep #36/#45 as fallback/runtime P2 work and #15 as packaging after persistence/history are usable.

## Why not run the full Evo X2 matrix now?

The source, prompt, artifact, repair, and gate layers are ready, but full Evo X2 draft generation is expensive and can take 20+ minutes per run. The Zenn and Qiita bounded proofs now pass, so the next useful spend is the full #40 publishing-target run.
