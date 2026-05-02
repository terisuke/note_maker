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

Open and active:

- Memory/history: [#26](https://github.com/terisuke/note_maker/issues/26), [#14](https://github.com/terisuke/note_maker/issues/14).
- History UI and readable artifacts: [#27](https://github.com/terisuke/note_maker/issues/27), [#28](https://github.com/terisuke/note_maker/issues/28).
- Quality and coverage: [#29](https://github.com/terisuke/note_maker/issues/29), [#13](https://github.com/terisuke/note_maker/issues/13).
- Runtime evaluation: [#40](https://github.com/terisuke/note_maker/issues/40).
- Live media-matrix runner: [#57](https://github.com/terisuke/note_maker/issues/57), child of [#40](https://github.com/terisuke/note_maker/issues/40).
- Fallback and packaging follow-up: [#36](https://github.com/terisuke/note_maker/issues/36), [#45](https://github.com/terisuke/note_maker/issues/45), [#15](https://github.com/terisuke/note_maker/issues/15).

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

There are three prerequisites before running the full multi-medium Evo X2 evaluation:

1. **Persistence first**: #26 must land so media-matrix drafts, evaluations, regenerated sections, answer forks, and style guides can be saved and reopened. Repeated Evo X2 runs are too expensive to leave only as loose files.
2. **Handler coverage gate**: #29 should run in parallel with #26 and must close before more endpoint-heavy UI work. #17-#25 added real handler surface; the next phase should harden it instead of adding more unguarded routes.
3. **Scenario ownership**: #40 owns the live Evo X2 media-matrix quality target. [#57](https://github.com/terisuke/note_maker/issues/57) owns the reusable runner/aggregate evaluator that executes the matrix and writes comparable reports.

## Parallel implementation plan

Use subagents with disjoint write scopes:

| Lane | Issue | Subagent role | Write scope | Done when |
|---|---|---|---|---|
| A | [#26](https://github.com/terisuke/note_maker/issues/26) / [#14](https://github.com/terisuke/note_maker/issues/14) | SQLite worker | `internal/infrastructure/repository/sqlite`, repository interfaces, boot wiring, migrations | JSON store imports, sessions/guides/briefs/drafts persist, cross-persona tests pass |
| B | [#29](https://github.com/terisuke/note_maker/issues/29) | Handler coverage worker | `internal/handlers/*_test.go`, coverage script/docs | `workflow.go` reaches the agreed coverage gate without real LLM/network |
| C | [#57](https://github.com/terisuke/note_maker/issues/57), feeding [#40](https://github.com/terisuke/note_maker/issues/40) | Scenario metrics worker | `cmd/scenario/*`, `docs/validation/*`, Make targets | media-matrix live runner records endpoint/model/elapsed/score/runes/verification in aggregate JSON/Markdown |

Lane A and Lane B can run immediately in parallel. Lane C can start by implementing offline/resumable runner mechanics now, but the full multi-case Evo X2 run should wait until Lane A provides persistence or until the user explicitly wants a one-off artifact-file run.

## Recommended order

1. Merge this docs alignment PR.
2. Start #26 and #29 in parallel.
3. Merge #29 as soon as handler coverage is sufficient.
4. Merge #26 once JSON import, SQLite schema, and restart recovery are proven.
5. Use #57/#40 to run one media-matrix case per implementation phase, then run the full note/Qiita/Zenn/company-blog pass after the persistence layer is stable.
6. Start #27 and #28 after #26; both depend on persistent projects/sessions/guides.
7. Start #13 after #27/#28 have enough browser surface to justify E2E tests.
8. Keep #36/#45 as fallback/runtime P2 work and #15 as packaging after persistence/history are usable.

## Why not run the full Evo X2 matrix now?

The source and prompt matrix is ready, but full Evo X2 draft generation is expensive and can take 20+ minutes per run. Running all media cases before persistence would produce useful files but not durable product memory. The better sequence is to make the system capable of storing those expensive results, then use #40 to evaluate one varied slice per phase and finally run the full comparison table.
