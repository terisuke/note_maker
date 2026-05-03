# Issue #13 Browser E2E Validation

Date: 2026-05-03
Branch: `codex/issue13-browser-e2e`

## Scope

This cut adds real browser E2E coverage for Issue #13 using Python `pytest` plus Playwright. The tests start the real Go web server on a free localhost port and stub application API calls through Playwright route handlers, so the browser exercises the production `static/index.html` and `static/js/script.js` without connecting to Evo X2 or any local LLM.

## Implementation

- `tests/e2e/e2e_server.py` starts `go run ./cmd/server` on an ephemeral port with temporary workflow/config storage and LLM endpoints disabled.
- `tests/e2e/conftest.py` provides `base_url`, `page`, and `api_stub` fixtures and replaces the external `marked` CDN script with a local browser stub.
- `tests/e2e/test_config_questions.py` covers model selector persistence/reload restore, persona/output-format template switching, custom question add/edit/delete/reset, legacy `localStorage` migration, preset style seeding, interview start payloads, and answer SSE submission.
- `tests/e2e/test_history_stream_regenerate.py` covers saved history opening, readable history cards, answer edit/fork, answer and draft streaming/cancel recovery, and section regeneration reject/accept.
- Missing Playwright or browser binaries fail the E2E target instead of reporting a skipped green run.
- `make e2e` runs the browser suite.

## Validation

```sh
python3 -m pytest tests/e2e -q
go test ./...
git diff --check
```

Result from the final local review:

```text
........                                                                 [100%]
8 passed in 5.80s
go test ./... passed
git diff --check passed
```

## Closure Decision

Issue #13 is closed by this cut. The original acceptance and later comments are covered by real browser tests:

- model dropdown population and persisted phase-model choices;
- custom question add/edit/delete/reset;
- legacy `questions` localStorage migration to `customQuestions`;
- persona and output-format switching with template reloads;
- starting an interview with custom questions in the `/api/brief-sessions` payload;
- answer SSE submission and cancel recovery;
- saved history style/session/project/article/draft opening and readable cards;
- edit-and-fork endpoint behavior;
- streaming draft UI, cancellation, failed/partial state recovery surface;
- section regeneration candidate reject and accept flow.

Remaining work after this cut belongs to broader Phase C product scope, not #13. The later Phase C persona/history/card polish cut adds custom persona create/list and editable brief/style card persistence; remaining product-memory work stays under #14 or follow-up issues.
