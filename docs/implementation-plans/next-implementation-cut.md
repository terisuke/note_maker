# Next implementation cut: Phase C persona/history/card polish

Date: 2026-05-03
Branch: `codex/phase-c-persona-history-polish`
Route: C, docs/coordination

This document records the current Phase C persona/history/card polish cut. The runtime and browser-E2E gates now have validation records; this cut adds custom persona create/list and editable brief/style card persistence on top of the existing history surface.

The remaining limitations are narrower: custom persona update/delete, richer persona source management, and full queryable/versioned product memory remain outside this cut.

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

Implemented in the current #27/#28 history/artifact cut:

- [#27](https://github.com/terisuke/note_maker/issues/27) — first saved-history UI cut: `履歴から再開`, persona-scoped saved style-guide and brief-session selectors, and open/clear/refresh controls.
- [#28](https://github.com/terisuke/note_maker/issues/28) — first readable artifact cut: style-guide cards and article-brief cards replace raw-only `pre` output while keeping Markdown/JSON details available.
- History read API surface — `GET /api/history`, `GET /api/workflow/artifacts`, `GET /api/author-style`, `GET /api/brief-sessions`, `GET /api/briefs`, and `GET /api/briefs/{id}`.
- Store list support — memory and SQLite now expose `ListAuthorStyles`, `ListSessions`, and `ListBriefs`; SQLite also has `ListProjects` and `ListArticlesByProject` for the richer #26 schema.
- Focused tests — `internal/handlers/workflow_history_test.go` covers saved artifact responses and brief detail errors; `static/history_ui_test.go` locks the frontend contract.
- Validation — [Issue 27/28 history and artifact UI/API validation](../validation/issue-27-28-history-artifacts-2026-05-03.md).

Implemented in the #13 follow-up contract cut:

- Static browser contract coverage checks the HTML entrypoint, production script, model selectors, question controls, history controls, artifact-card containers, persistence hooks, and route-table wiring.
- This is useful pre-Playwright coverage, but it is not equivalent to browser E2E. It does not exercise real DOM events, fetch stubbing, localStorage reload behavior, SSE/cancel, or the section-regeneration workflow in a browser.
- Validation — [Issue #13 Browser Contract Coverage](../validation/issue-13-browser-contract-coverage-2026-05-03.md).

Implemented in the `codex/issue13-browser-e2e` cut:

- Python `pytest` plus Playwright starts the real Go server on a free localhost port and stubs application APIs through browser route handlers.
- Browser tests now cover model selector persistence, legacy localStorage migration, persona/format switching, custom question CRUD/reset, interview start payloads, answer SSE submission/cancel recovery, saved history opening/readable cards, edit/fork, draft streaming/cancel recovery, and section regeneration reject/accept.
- Validation — [Issue #13 Browser E2E Validation](../validation/issue-13-browser-e2e-2026-05-03.md).

Open and active:

- Memory/history umbrella: [#14](https://github.com/terisuke/note_maker/issues/14), now backed by the #26 schema work and the project/article/draft read surface.
- Runtime evaluation: [#40](https://github.com/terisuke/note_maker/issues/40), now satisfied for the current note/Qiita/Zenn/Cor blog publishing-target acceptance scope by the 2026-05-03 full Tailnet Evo X2 matrix.
- Runtime evaluation sub-issue [#74](https://github.com/terisuke/note_maker/issues/74), satisfied by the staged reruns and the final `5/5` full matrix pass.
- Fallback and packaging follow-up: [#36](https://github.com/terisuke/note_maker/issues/36), [#45](https://github.com/terisuke/note_maker/issues/45), [#15](https://github.com/terisuke/note_maker/issues/15).
- Runtime defect fixed by this cut: [#63](https://github.com/terisuke/note_maker/issues/63) makes the plain web-app default match the intended Evo X2 Tailnet primary path and records the 2026-05-03 draft-generation 500 root cause.
- Documentation and DDD audit: [#64](https://github.com/terisuke/note_maker/issues/64), with details in [Runtime and DDD alignment audit](../validation/runtime-ui-ddd-audit-2026-05-03.md).
- Interview usability fixed before measurement: [#66](https://github.com/terisuke/note_maker/issues/66), with details in [Issue 66 plain brief questions validation](../validation/issue-66-plain-brief-questions-2026-05-03.md).
- Style-source switching fixed before measurement: [#68](https://github.com/terisuke/note_maker/issues/68), with details in [Issue 68 media-aware style source validation](../validation/issue-68-media-aware-style-source-2026-05-03.md).

Resolved validation baseline:

- Browser E2E coverage: [#13](https://github.com/terisuke/note_maker/issues/13) is closed by the Playwright validation cut. Track remaining Phase C product work under #14/#27/#28.

Implemented in this Phase C polish cut:

- `GET /api/personas` returns built-in and user-authored personas; `POST /api/personas` validates and persists custom personas.
- Memory and SQLite stores persist custom personas; SQLite restore after reopen is covered.
- The web UI exposes an add-persona form, selects the new persona after save, keeps the history persona selector aligned, and reloads the saved persona through the API.
- Style-guide cards can be edited and saved through `PATCH /api/author-style/{id}` / `POST /api/author-style/{id}/versions`; the server stores the edit as a new style-guide version.
- Brief cards can be edited and saved through `PATCH /api/briefs/{id}`; the saved artifact updates while existing session answers remain auditable.
- Phase C E2E covers custom persona add/reload plus brief/style card save, cancel, and error behavior.

Remaining Phase C limitations:

- Custom persona update/delete is not implemented.
- Persona source management is minimal: create accepts initial source metadata, while richer source editing remains future work.
- Brief-card edits update the saved brief artifact; they do not rewrite the original interview answers or create a separate brief-version table.
- #14 remains open for full queryable product memory and broader version/history semantics beyond this cut.

## Current Review Findings

Review date: 2026-05-03 on `codex/phase-c-history-e2e`.

Validation that passed:

```sh
go test ./...
go test ./internal/handlers ./static
go test ./cmd/server ./internal/handlers ./static
node --check static/js/script.js
git diff --check
```

No blocking code-risk finding remains in the targeted suite after the parallel fixes. The #13 browser E2E closure risk is resolved by the `tests/e2e` suite. The Phase C polish validation now covers custom persona create/list and editable brief/style card persistence; #14 remains the broader product-memory umbrella.

## Issue Close/Open Proposal

| Issue | Proposal | Rationale |
|---|---|---|
| [#13](https://github.com/terisuke/note_maker/issues/13) | Closed | Browser E2E validation covers model config, questions, persona/format switching, history/cards, streaming/cancel, edit/fork, and regenerate-section. |
| [#14](https://github.com/terisuke/note_maker/issues/14) | Keep open | #26 gives the SQLite schema, the current branch exposes project/article/draft read cards, and this cut adds custom persona persistence plus editable brief/style cards. Broader queryable product memory, persona update/delete, and complete artifact version/history semantics remain. |
| [#27](https://github.com/terisuke/note_maker/issues/27) | Close with this PR if the issue owner accepts create/list as the custom-persona scope | Saved style-guide/session/project/article/draft selectors are implemented, custom personas can be created/listed/persisted, and E2E covers selecting the saved persona and loading its history after reload. Custom persona update/delete should be tracked separately if needed. |
| [#28](https://github.com/terisuke/note_maker/issues/28) | Close with this PR if the issue owner accepts brief/style edit persistence as the card scope | Readable style-guide and brief cards are editable with save/cancel/error behavior. Style edits create a new saved guide version; brief edits persist the saved artifact while leaving original session answers auditable. |

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

The bounded Cloudia technical proofs then passed:

| Case | Seconds | First chunk | Chunks | Score / min | Runes / min | Verification |
|---|---:|---:|---:|---:|---:|---|
| `cloudia_zenn_tutorial` | `741.93` | `86456ms` | `2205` | `88.1 / 82.0` | `5040 / 1800` | passed |
| `cloudia_qiita_how_to` | `598.62` | `111645ms` | `1336` | `83.7 / 82.0` | `3318 / 1400` | passed |

The full publishing-target matrix now also passes:

| Case | Attempt | Seconds | First chunk | Chunks | Score / min | Runes / min | Verification |
|---|---:|---:|---:|---:|---:|---:|---|
| `terisuke_note_essay` | 1 | `107.86` | `67818ms` | `1610` | `90.7 / 82.0` | `2849 / 2800` | passed |
| `cor_blog_technical_report` | 1 | `152.34` | `67984ms` | `1696` | `81.4 / 80.0` | `3329 / 2200` | passed |
| `cor_blog_vision_sharing` | 1 | `113.98` | `68986ms` | `1657` | `89.5 / 80.0` | `3156 / 1600` | passed |
| `cloudia_zenn_tutorial` | 1 | `121.14` | `65568ms` | `2686` | `86.4 / 82.0` | `5641 / 1800` | passed |
| `cloudia_qiita_how_to` | 2 | `114.75` | `68857ms` | `1898` | `82.2 / 82.0` | `3737 / 1400` | passed |

Aggregate: `5/5` passed, `0` failed, average `122.01s`, average style score `86.0`, average `3742` runes. Artifacts are `tmp/media_matrix/live/aggregate.json` and `tmp/media_matrix/live/aggregate.md`.

## Parallel implementation plan

Use subagents with disjoint write scopes when implementation resumes:

| Lane | Issue | Subagent role | Write scope | Done when |
|---|---|---|---|---|
| A | [#14](https://github.com/terisuke/note_maker/issues/14) | Persistence worker | future SQLite/history API and validation docs | custom personas, edited artifacts, projects, articles, drafts, source snapshots, and versions are queryable as one coherent product history |
| B | Persona follow-up | Product worker | future persona edit/delete UI and API files | custom personas can be updated or removed without corrupting existing history references |
| C | Artifact follow-up | Product worker | future artifact version UI and API files | brief edits have explicit version/history semantics comparable to style-guide versions |

Keep these lanes disjoint when implementation resumes; persona management and artifact history are adjacent but separable.

## Recommended order

1. Use the #13 Playwright validation as the browser baseline; do not track Phase C product gaps as browser-E2E debt.
2. Merge this Phase C polish cut if the validation document stays green, then close #27/#28 according to the issue-owner scope decision.
3. Keep #14 open for complete queryable product memory and decide whether custom persona update/delete or brief-version tables need separate follow-up issues.
4. Keep #36/#45 as fallback/runtime P2 work and #15 as packaging after persistence/history are usable. Homepage remains a separate short-format check, not part of the #40 closure gate.
