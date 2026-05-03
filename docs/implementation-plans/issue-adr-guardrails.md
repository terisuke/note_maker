# Issue and ADR guardrails

Date: 2026-05-01 (last updated 2026-05-03)

This document maps GitHub issues to [ADR 0001](../adrs/0001-three-phase-local-article-generation.md) and [ADR 0002](../adrs/0002-multi-persona-multi-format-extension.md) and defines implementation guardrails.

## Issue Map

| Issue | Scope | ADR Section | Guardrail |
| --- | --- | --- | --- |
| [#7](https://github.com/terisuke/note_maker/issues/7) | Author style profile workflow | Author Style Analysis | Domain must not import Note or LLM infrastructure. Real Note access only happens in infrastructure/scenario paths. |
| [#8](https://github.com/terisuke/note_maker/issues/8) | Fixed-question and deep-dive brief interview | Article Brief Interview | Deep dives must be anchored to a fixed `target_question_id`; no free-topic drift. |
| [#9](https://github.com/terisuke/note_maker/issues/9) | Draft generation from style guide and brief | Draft Generation | Draft generation must not fetch Note articles; it only consumes `WritingStyleGuide + ArticleBrief`. |
| [#10](https://github.com/terisuke/note_maker/issues/10) | API, UI, and scenario integration | API Direction / Testing Strategy | `go test ./...` stays offline; network/LLM scenarios require explicit env vars. |

Active issues that ADR 0002 reframes (see [ADR 0002 — Tracked issues](../adrs/0002-multi-persona-multi-format-extension.md#tracked-issues) for the new umbrella):

| Issue | Scope | ADR Section | Guardrail |
| --- | --- | --- | --- |
| [#14](https://github.com/terisuke/note_maker/issues/14) | Persistent queryable database | ADR 0002 §Persistence direction | SQLite migration is the acceptance for #14; multi-persona schema is mandatory. |
| [#15](https://github.com/terisuke/note_maker/issues/15) | Desktop launcher packaging | Out of ADR 0002 scope | Tracked separately; depends on Phase C completion before packaging makes sense. |
| [#36](https://github.com/terisuke/note_maker/issues/36) | local llama.cpp fallback quality | ADR 0001/0002 runtime validation | Non-blocking for Phase A. Do not promote fallback as production-quality until it passes strict draft thresholds. |
| [#45](https://github.com/terisuke/note_maker/issues/45) | Evo X2 llama.cpp swap orchestration | ADR 0001/0002 runtime validation | Non-blocking P2. Keep Ollama primary; llama.cpp swap/start commands must be dry-run by default, require explicit restart gates, target `/llama/v1` directly for validation, and remain open until live brief/draft metrics pass without disrupting Ollama. |
| [#40](https://github.com/terisuke/note_maker/issues/40) | Tailnet Evo X2 primary quality and runtime metrics epic | ADR 0001/0002 runtime validation | Primary runtime must record endpoint/model/elapsed/score/runes and distinguish generation variance from transport failures. The current note/Qiita/Zenn/Cor blog publishing-target scope passed on 2026-05-03 with a `5/5` full Tailnet Evo X2 matrix run. |
| [#57](https://github.com/terisuke/note_maker/issues/57) | Live media-matrix runner and aggregate evaluator | ADR 0001/0002 runtime validation | Child of #40. Offline mode remains default; live mode must require explicit env vars and must refuse accidental workstation-local fallback for primary Evo X2 validation. |
| [#70](https://github.com/terisuke/note_maker/issues/70) | Interview-template scenario before Evo X2 media runs | ADR 0002 §Testing Strategy | The question-template change must be tested before draft-only live runs. Scenario output must prove small plain-Japanese questions and medium-specific `ArticleBrief` artifacts. |
| [#71](https://github.com/terisuke/note_maker/issues/71) | Failed draft artifacts and runtime metrics | ADR 0001/0002 runtime validation | Early validation failures must preserve raw output, elapsed time, endpoint, model, and failure JSON. Do not discard unusable drafts before diagnosis. |
| [#72](https://github.com/terisuke/note_maker/issues/72) | Bounded format-repair retry | ADR 0002 §Format-specific output | Validators remain strict. One repair retry may be attempted for recoverable preamble or cross-format notation failures, with original and repaired attempts preserved. |
| [#73](https://github.com/terisuke/note_maker/issues/73) | Output-format-specific scenario gates | ADR 0002 §Testing Strategy | Long-form note/Zenn/Qiita/Cor blog gates stay strict, while homepage HTML uses short-form structure and CTA gates instead of long-article length assumptions. |
| [#74](https://github.com/terisuke/note_maker/issues/74) | Staged Tailnet Evo X2 validation rerun | ADR 0001/0002 runtime validation | Re-run order is template scenario → offline media matrix → one previously failing live case → full note/Qiita/Zenn/Cor blog live comparison. The final full comparison passed `5/5`; closure requires linking `tmp/media_matrix/live/aggregate.{json,md}` and the validation doc. |

Current cut status:

- [#26](https://github.com/terisuke/note_maker/issues/26) is implemented as `internal/infrastructure/repository/sqlite` plus `WORKFLOW_STORE_DRIVER=sqlite` web-app opt-in. [#14](https://github.com/terisuke/note_maker/issues/14) remains the broader queryable-history umbrella for complete product memory beyond the current custom-persona and brief/style edit surface.
- The [#13](https://github.com/terisuke/note_maker/issues/13) browser-E2E gate is closed by the Playwright validation record. Do not track remaining Phase C product polish as #13 scope.
- The `codex/phase-c-persona-history-polish` cut implements custom persona create/list plus editable brief/style card persistence. Keep custom persona update/delete and broader version/history semantics separate from the #13 browser-coverage gate.
- [#29](https://github.com/terisuke/note_maker/issues/29) reaches the handler coverage gate: `go test ./internal/handlers -cover` reports 80.0%.
- [#57](https://github.com/terisuke/note_maker/issues/57) is implemented as `cmd/scenario/live_media_matrix`; it defaults to offline planned aggregate output and requires `RUN_LIVE_MEDIA_MATRIX=1` or `make scenario-media-matrix-live` for Evo X2 calls.
- [#40](https://github.com/terisuke/note_maker/issues/40) is now an epic with sub-issues [#70](https://github.com/terisuke/note_maker/issues/70)-[#74](https://github.com/terisuke/note_maker/issues/74). The staged validation criteria are met for the current publishing-target scope: the final full matrix passed `5/5` with endpoint, phase models, elapsed time, score, runes, final verification, structural gates, quality gates, and artifacts recorded.

Closed historical issues:

| Issue | Completed Scope | Relation to ADR 0001 |
| --- | --- | --- |
| [#3](https://github.com/terisuke/note_maker/issues/3) | Gemini removal and local LLM runtime | Provides local `gemma4:31b` foundation. |
| [#4](https://github.com/terisuke/note_maker/issues/4) | Resilient Note acquisition | Provides article acquisition adapter for author style analysis. |
| [#5](https://github.com/terisuke/note_maker/issues/5) | DDD boundary split | Provides package boundary precedent. |
| [#6](https://github.com/terisuke/note_maker/issues/6) | API contract alignment | Existing compatibility endpoint remains while new workflow is added. |
| [#11](https://github.com/terisuke/note_maker/issues/11) | Strict style threshold tuning | Threshold logic is in place; future persona-specific revisions must be tracked separately. |
| [#13](https://github.com/terisuke/note_maker/issues/13) | Browser E2E for model config and question CRUD | Closed. Playwright coverage exists for model config persistence, custom questions, persona/format switching, history/cards, streaming/cancel, edit/fork, and regenerate-section. |
| [#21](https://github.com/terisuke/note_maker/issues/21) | Persona and OutputFormat domain concepts | B1 landed early; remaining B work must not expand persistence assumptions until Phase C. |
| [#22](https://github.com/terisuke/note_maker/issues/22) | Historical source acquisition | Zenn/Qiita/Cor blog sources are available; Cor blog style analysis should prefer GitHub Markdown over RSS summaries. |
| [#23](https://github.com/terisuke/note_maker/issues/23) | Format prompt templates and validators | Format guides and validators exist; new formats must add validator + guide + scenario sample. |
| [#24](https://github.com/terisuke/note_maker/issues/24) | Seed `terisuke` and `cloudia` personas | Persona seeds are available; third-persona work must wait for SQLite persistence. |
| [#25](https://github.com/terisuke/note_maker/issues/25) | Persona/format question templates | Server templates exist; frontend must not duplicate template questions when sending custom questions. |

## ADR 0002 Phase Map

The phases in [ADR 0002](../adrs/0002-multi-persona-multi-format-extension.md) (A, B, C, D) are tracked as separate issues in the new tranche. Their guardrails extend the rules below:

- Phase A (Conversation UX): keep domain changes narrow to auditable conversation state transitions such as fork-on-edit. Must keep all existing `go test ./...` green without weakening expectations.
- Phase A execution started with [#18](https://github.com/terisuke/note_maker/issues/18) because Tailnet Evo X2 runs are long enough that spinner-only UX is no longer acceptable. [#17](https://github.com/terisuke/note_maker/issues/17) follows and reuses the streaming primitives.
- Phase B (Persona / OutputFormat): implemented for built-in personas, five formats, source acquisition, and question templates. Further persona/library expansion should wait for Phase C persistence.
- Phase C (SQLite store): repository interfaces stay; only implementations change. JSON-file store remains the compatibility path, but storage selection must be visible in the web settings UI rather than hidden behind make/env setup. Phase C product completion requires persisted custom personas and editable human-readable artifacts, not only built-in persona selection and read-only cards; the current polish cut covers create/list and brief/style edits, while update/delete and broader artifact versioning remain follow-up scope.
- Phase D (Quality): handler tests are mandatory before any further endpoint-heavy UI work lands. Coverage gate: `internal/handlers/workflow.go` ≥ 80 %.

## Architectural Guardrails

1. Domain packages contain business rules only.
   - Allowed: value objects, entities, validation, scoring, deterministic selection.
   - Not allowed: HTTP clients, file I/O, environment variables, LLM calls.

2. Application packages orchestrate ports.
   - They may depend on domain packages and interfaces.
   - They must not hard-code concrete infrastructure clients.

3. Infrastructure packages adapt external systems.
   - Note.com access belongs in `internal/infrastructure/note`.
   - OpenAI-compatible local LLM access belongs in `internal/infrastructure/llamacpp`.
   - In-memory/file repositories belong in `internal/infrastructure/repository`.
   - Evo X2 Ollama is the primary heavy-inference runtime and must be reached through the Tailnet OpenAI-compatible API (`http://evo-x2.tailb30e58.ts.net/v1` by default) in `make dev`, `make evo-x2`, the plain web server, and scenario targets.
   - The fallback order is Evo X2 Ollama → Evo X2 llama.cpp (`http://evo-x2.tailb30e58.ts.net/llama/v1`) → workstation-local llama.cpp.
   - SSH tunnels are allowed only as explicit developer diagnostics, not as the product default, because they depend on per-device SSH setup.
   - Local llama.cpp (`http://127.0.0.1:8081/v1`) is fallback only. Do not set `LLM_BASE_URL` to local Ollama or local llama.cpp for Evo X2 validation unless the test is explicitly measuring fallback behavior.
   - Runtime validation must report base URL, model, elapsed time, score, and draft length.
   - Evo X2 llama.cpp swap/start orchestration for Issue [#45](https://github.com/terisuke/note_maker/issues/45) must be dry-run by default. Any remote start requires `EVO_X2_LLAMA_CPP_APPLY=1`; any profile swap/restart also requires `EVO_X2_LLAMA_CPP_ALLOW_RESTART=1`.
   - #45 validation must set `LLM_BASE_URL` directly to Evo X2 `/llama/v1` and clear fallback URLs so the run cannot silently pass through Ollama.
   - Each implementation PR that touches interview, prompt, draft, or runtime behavior should add one scenario datapoint with a deliberately varied medium/persona/format. If the PR touches question templates, the datapoint must come from the interview-template scenario rather than only draft generation. Do not force every PR to rerun every live scenario; build averages by collecting one different slice per phase. Use `cmd/scenario/media_matrix` as the canonical matrix for final Note/Qiita/Zenn/Cor blog comparison.
   - Draft generation must run the lightweight final verification step before returning the final result; if verification reports NEEDS_REVIEW, surface the report instead of hiding it.
   - If fallback validation fails the strict draft thresholds, keep Evo X2 primary enabled and track fallback hardening separately (Issue [#36](https://github.com/terisuke/note_maker/issues/36)).
   - If Tailnet Evo X2 reaches the API but misses quality gates, track it under Issue [#40](https://github.com/terisuke/note_maker/issues/40), not as a transport regression.

4. Handlers are JSON boundaries.
   - They create/use application services.
   - They do not build prompts directly.
   - They do not contain interview phase logic.

5. UI follows the workflow.
   - Style analysis, interview, and draft generation are visible phases.
   - The user can inspect style guide and brief before draft generation.

## Interview Guardrails

The interview is a structured取材 session, not a generic chat.

- Fixed questions run first in deterministic order.
- Fixed questions should be small and plain enough to answer in one or two short sentences. If one question asks for multiple kinds of thinking, split it into smaller template questions.
- Optional questions must be clearly optional in the UI and may advance as `未定`; do not force the user to invent detail just to continue.
- Deep-dive questions run after fixed questions.
- A deep-dive question must store:
  - `target_question_id`
  - `follow_up_index`
  - `flow_type=deep_dive_follow_up`
- Follow-ups must ask exactly one question.
- Follow-ups must not be yes/no questions.
- Follow-ups must not be binary choice questions.
- Follow-ups must ask for one concrete scene, step, number, reason, turning point, emotion, or reader lesson.
- If LLM-generated follow-up text fails validation, use a rule-based fallback.

## Draft Guardrails

Draft generation receives:

- `WritingStyleGuide`
- completed `ArticleBrief`
- `AuthorStyleProfile` for evaluation

It must not receive raw full article bodies by default.

The response should include:

- validated Markdown draft,
- style comparison,
- pass/fail evaluation,
- risks when thresholds are not met.

Minimum scenario thresholds:

- total score `>= 82`
- paragraph length `>= 75`
- sentence length `>= 75`
- keyword overlap `>= 70`
- quote density `>= 55`
- first-person score `>= 60`

## Test Guardrails

Normal tests:

```bash
go test ./...
```

must not require:

- note.com network access,
- local LLM server,
- `gemma4:31b`,
- external secrets.

Scenario tests and commands may require:

- `RUN_NOTE_SCENARIO=1`
- `RUN_LOCAL_LLM_SCENARIO=1`
- `LLAMACPP_BASE_URL`
- `LLAMACPP_MODEL=gemma4:31b`

Current live-media evaluation flow:

1. `go run ./cmd/scenario/media_matrix` creates the deterministic cross-media brief/prompt matrix.
2. `RUN_SOURCE_FETCH_SCENARIO=1 ... go run ./cmd/scenario/source_fetch` validates current live sources.
3. #57's runner emits planned aggregate output by default and live output only with explicit `RUN_LIVE_MEDIA_MATRIX=1`.
4. #74's staged sequence is complete for note/Qiita/Zenn/Cor blog: bounded Zenn and Qiita proofs passed, then the full publishing-target matrix passed `5/5`.
5. Future live-media runs must preserve primary/fallback endpoint, per-phase models, elapsed seconds, score/runes/minimums, final verification, structural gate result, `quality_gate`, and draft/evaluation/verification/failure/raw artifact paths.

## Completion Criteria

An issue can be closed only when:

1. code exists for the issue scope,
2. unit tests cover the behavior without external dependencies,
3. relevant scenario command exists or the integration issue explicitly owns it,
4. docs are updated if API/UI behavior changed,
5. `go test ./...` passes.
