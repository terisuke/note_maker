# Issue #13 Browser Contract Coverage

Date: 2026-05-03
Branch: `codex/phase-c-history-e2e`; reviewed again from `codex/issue13-browser-e2e`

## Scope

Added practical browser-adjacent coverage that remains part of `go test ./...`:

- Static HTML contract checks for model selectors, question controls, history controls, and artifact card containers.
- Static JavaScript contract checks for model config persistence, custom question add/edit/delete/reset behavior, history loading/opening flow, and readable artifact-card rendering.
- `httptest` coverage that the server route table exposes workflow read APIs and serves the browser entrypoint plus the production script.

This validates contract shape, not user behavior in a real browser. The later Playwright validation cut covers the actual browser flows for Issue [#13](https://github.com/terisuke/note_maker/issues/13).

## Original Playwright Gap

At the time of the contract cut, the frontend script was a single `DOMContentLoaded` closure with no importable UI functions. A later cut added the heavier test harness with stubbed API routes and browser setup.

This pass keeps CI friction low by expanding Go tests first. The contract tests intentionally lock DOM IDs, event bindings, persistence calls, fetch endpoints, and card-rendering structure so a later Playwright suite has stable selectors and scenarios to target.

## Playwright Path

Scenarios carried into the browser E2E cut:

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

This browser-contract document is a validation checkpoint. The later browser E2E validation over stubbed API responses closed #13.

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

#13 is closed by the browser E2E cut. Remaining work is Phase C product scope rather than browser coverage scope.

## Issue Policy

- #13: closed with the browser E2E cut.
- #14: closed later by PR #87 for the current app baseline after custom persona update/delete and brief-version history landed.
- #27: closed by the Phase C history/persona picker work.
- #28: closed by the readable artifact card work.
