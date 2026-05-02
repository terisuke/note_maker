# Issue and ADR guardrails

Date: 2026-05-01 (last updated 2026-05-02)

This document maps GitHub issues to [ADR 0001](../adrs/0001-three-phase-local-article-generation.md) and [ADR 0002](../adrs/0002-multi-persona-multi-format-extension.md) and defines implementation guardrails.

## Issue Map

| Issue | Scope | ADR Section | Guardrail |
| --- | --- | --- | --- |
| [#7](https://github.com/terisuke/note_maker/issues/7) | Author style profile workflow | Author Style Analysis | Domain must not import Note or LLM infrastructure. Real Note access only happens in infrastructure/scenario paths. |
| [#8](https://github.com/terisuke/note_maker/issues/8) | Fixed-question and deep-dive brief interview | Article Brief Interview | Deep dives must be anchored to a fixed `target_question_id`; no free-topic drift. |
| [#9](https://github.com/terisuke/note_maker/issues/9) | Draft generation from style guide and brief | Draft Generation | Draft generation must not fetch Note articles; it only consumes `WritingStyleGuide + ArticleBrief`. |
| [#10](https://github.com/terisuke/note_maker/issues/10) | API, UI, and scenario integration | API Direction / Testing Strategy | `go test ./...` stays offline; network/LLM scenarios require explicit env vars. |

Open issues that ADR 0002 reframes (see [ADR 0002 — Tracked issues](../adrs/0002-multi-persona-multi-format-extension.md#tracked-issues) for the new umbrella):

| Issue | Scope | ADR Section | Guardrail |
| --- | --- | --- | --- |
| [#11](https://github.com/terisuke/note_maker/issues/11) | Strict style threshold tuning | ADR 0001 Draft Generation | Thresholds remain code-level; tuning may be revised once `first_person` density is computed per persona (ADR 0002 §Persona). |
| [#13](https://github.com/terisuke/note_maker/issues/13) | Browser E2E for model config and question CRUD | ADR 0002 §Testing Strategy | Phase D folds persona switch, format switch, edit-and-fork, streaming, regenerate-section into the E2E surface. |
| [#14](https://github.com/terisuke/note_maker/issues/14) | Persistent queryable database | ADR 0002 §Persistence direction | SQLite migration is the acceptance for #14; multi-persona schema is mandatory. |
| [#15](https://github.com/terisuke/note_maker/issues/15) | Desktop launcher packaging | Out of ADR 0002 scope | Tracked separately; depends on Phase C completion before packaging makes sense. |

Closed historical issues:

| Issue | Completed Scope | Relation to ADR 0001 |
| --- | --- | --- |
| [#3](https://github.com/terisuke/note_maker/issues/3) | Gemini removal and local LLM runtime | Provides local `gemma4:31b` foundation. |
| [#4](https://github.com/terisuke/note_maker/issues/4) | Resilient Note acquisition | Provides article acquisition adapter for author style analysis. |
| [#5](https://github.com/terisuke/note_maker/issues/5) | DDD boundary split | Provides package boundary precedent. |
| [#6](https://github.com/terisuke/note_maker/issues/6) | API contract alignment | Existing compatibility endpoint remains while new workflow is added. |

## ADR 0002 Phase Map

The phases in [ADR 0002](../adrs/0002-multi-persona-multi-format-extension.md) (A, B, C, D) are tracked as separate issues in the new tranche. Their guardrails extend the rules below:

- Phase A (Conversation UX): no domain changes, only handler streaming + frontend rewrite. Must keep all existing `go test ./...` green without modification.
- Phase B (Persona / OutputFormat): introduces `internal/domain/persona` and `internal/domain/format`. The note.com host check moves out of application services into `internal/infrastructure/source/note` only.
- Phase C (SQLite store): repository interfaces stay; only implementations change. JSON-file store becomes import/export utility.
- Phase D (Quality): handler tests are mandatory before any further endpoint additions land. Coverage gate: `internal/handlers/workflow.go` ≥ 80 %.

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
   - Evo X2 is the primary heavy-inference runtime and must be reached through the Tailscale SSH tunnel endpoint (`http://127.0.0.1:21434/v1`) in `make evo-x2` and scenario targets.
   - Local llama.cpp (`http://127.0.0.1:8081/v1`) is fallback only. Do not set `LLM_BASE_URL` to local Ollama or local llama.cpp for Evo X2 validation unless the test is explicitly measuring fallback behavior.
   - Runtime validation must report base URL, model, elapsed time, score, and draft length.
   - If fallback validation fails the strict draft thresholds, keep Evo X2 primary enabled and track fallback hardening separately (Issue [#36](https://github.com/terisuke/note_maker/issues/36)).

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
- Deep-dive questions run after fixed questions.
- A deep-dive question must store:
  - `target_question_id`
  - `follow_up_index`
  - `flow_type=deep_dive_follow_up`
- Follow-ups must ask exactly one question.
- Follow-ups must not be yes/no questions.
- Follow-ups must not be binary choice questions.
- Follow-ups must ask for concrete scene, reason, turning point, emotion, or reader lesson.
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

## Completion Criteria

An issue can be closed only when:

1. code exists for the issue scope,
2. unit tests cover the behavior without external dependencies,
3. relevant scenario command exists or the integration issue explicitly owns it,
4. docs are updated if API/UI behavior changed,
5. `go test ./...` passes.
