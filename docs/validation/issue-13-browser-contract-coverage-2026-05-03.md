# Issue #13 Browser Contract Coverage

Date: 2026-05-03
Branch: `codex/phase-c-history-e2e`; reviewed again from `codex/issue13-browser-e2e`

## Scope

Added practical browser-adjacent coverage that remains part of `go test ./...`:

- Static HTML contract checks for model selectors, question controls, history controls, and artifact card containers.
- Static JavaScript contract checks for model config persistence, custom question add/edit/delete/reset behavior, history loading/opening flow, and readable artifact-card rendering.
- `httptest` coverage that the server route table exposes workflow read APIs and serves the browser entrypoint plus the production script.

This validates contract shape, not user behavior in a real browser. Issue [#13](https://github.com/terisuke/note_maker/issues/13) stays open until Playwright or equivalent browser E2E covers the actual flows.

## Why Not Playwright Yet

The current frontend script is a single `DOMContentLoaded` closure with no importable UI functions. A Playwright suite is still the right next step, but adding it now would require a heavier test harness with stubbed API routes and browser setup that is not yet present in this repository.

This pass keeps CI friction low by expanding Go tests first. The contract tests intentionally lock DOM IDs, event bindings, persistence calls, fetch endpoints, and card-rendering structure so a later Playwright suite has stable selectors and scenarios to target.

## Next Playwright Path

Recommended scenarios when browser E2E is introduced:

1. Stub `/api/models`, `/api/personas`, `/api/formats`, `/api/brief-sessions/templates`, and `/api/workflow/artifacts`.
2. Assert all four model selectors populate from `/api/models`, save into `localStorage["note-maker-config-v1"]`, and restore on reload.
3. Add a custom question, edit it, delete it, reload with saved custom questions, and reset back to template-only state.
4. Select saved style/session history, assert the open button enables, clear disables it, and opening fetches `/api/author-style/{id}` or `/api/brief-sessions/{id}` as needed.
5. Assert non-empty style and brief cards render `.artifact-card-header`, `.artifact-meta`, and `.artifact-section`, while raw Markdown/JSON previews remain populated.

## Commands

```sh
go test ./static ./cmd/server
go test ./...
```

## Review Result

Commands passed after aligning the parallel history test fixture with the selected output-format validator:

```sh
go test ./cmd/server ./internal/handlers ./static
go test ./internal/handlers ./static
go test ./static ./cmd/server
go test ./...
node --check static/js/script.js
git diff --check
```

This browser-contract document is still a validation checkpoint, not a close signal for #13. The next cut should add browser E2E over stubbed API responses once the project has a Playwright or equivalent harness.

## Browser E2E Follow-up

Review date: 2026-05-03 on `codex/issue13-browser-e2e`.

The follow-up cut adds real browser E2E coverage using Python `pytest` plus Playwright. It starts the real Go server on a free localhost port and uses Playwright route handlers to stub `/api/*` and the external `marked` CDN script. This keeps the tests deterministic and avoids Evo X2/local LLM calls while still exercising the browser DOM, production HTML, and production JavaScript.

```sh
python3 -m pytest tests/e2e -q
```

Result from the final local review:

```text
........                                                                 [100%]
8 passed in 5.80s
```

Repository validation also passed:

```sh
go test ./...
git diff --check
```

See [Issue #13 Browser E2E Validation](./issue-13-browser-e2e-2026-05-03.md) for the scenario list.

Issue #13 can close with the browser E2E cut. Remaining work is Phase C product scope rather than browser coverage scope.

## Issue Policy

- #13: close with the browser E2E cut.
- #14: keep open. SQLite exists, but queryable product memory is not fully exposed.
- #27: keep open unless the owner explicitly splits and closes the first saved-history picker cut.
- #28: keep open unless the owner explicitly splits and closes the first readable-card cut.
