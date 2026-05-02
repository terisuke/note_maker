# Runtime UI and DDD alignment audit - 2026-05-03

## Scope

This audit reconciles the current implementation with ADR 0002, the open issue set, and the browser failure reported on 2026-05-03.

## Browser failure analysis

Observed while the app was running from the plain server defaults:

- `GET /api/models` returned HTTP 500 because the LLM client attempted `http://127.0.0.1:8081/v1/models` and the connection was refused.
- `POST /api/drafts` with `Accept: text/event-stream` returned HTTP 200, then emitted `event: error` with `DRAFT_GENERATION_FAILED` because `http://127.0.0.1:8081/v1/chat/completions` was refused.
- `GET /favicon.ico` returned 404. This was noisy but not the draft-generation failure.

Root cause: the Makefile remote path already used Evo X2 Tailnet, but plain web-app startup and the LLM client default still treated workstation-local llama.cpp as the primary endpoint. That contradicts the intended order recorded in ADR 0002 and the guardrails:

1. Evo X2 Ollama over Tailscale OpenAI-compatible API.
2. Evo X2 llama.cpp over Tailscale OpenAI-compatible API.
3. Workstation-local llama.cpp as the last fallback.

## Fix in this cut

Issue [#63](https://github.com/terisuke/note_maker/issues/63) restores the runtime order for normal browser use:

- `internal/infrastructure/llamacpp` now defaults to `http://evo-x2.tailb30e58.ts.net/v1`.
- When that default is used, fallback endpoints are `http://evo-x2.tailb30e58.ts.net/llama/v1` and then `http://127.0.0.1:8081/v1`.
- Phase defaults now match the current operational plan: style/article `gemma4:e2b`, brief `qwen3.6:27b`, draft `gemma4:31b`, verify `gemma4:latest`.
- `make dev` and `scripts/dev.sh` default to remote runtime instead of starting workstation-local llama.cpp.
- The draft UI reports the actual SSE endpoint/model on `runtime_connected`.
- `/favicon.ico` returns 204 to remove the unrelated browser-console noise.

## Verification in this cut

Commands run from a plain `PORT=18080 go run ./cmd/server` startup:

- `curl -i http://127.0.0.1:18080/favicon.ico` returned `HTTP/1.1 204 No Content`.
- `curl -i http://127.0.0.1:18080/api/models` returned `HTTP/1.1 200 OK` with Evo X2 model IDs including `gemma4:e2b`, `gemma4:latest`, `gemma4:31b`, `qwen3.6:27b`, and `qwen3.6:35b`.

This proves the browser-visible model endpoint no longer defaults to the failed workstation-local `127.0.0.1:8081` path. A full draft-generation score run remains under #40 because it is a live Evo X2 quality/latency measurement, not a startup-routing smoke test.

## Issue state

Merged and no longer current-cut work:

- #11, #17, #18, #19, #20: Phase A and strict Terisuke style work.
- #21, #22, #23, #24, #25: Phase B persona, format, source, seed, and question-template work.
- #26: SQLite workflow store foundation.
- #29: handler coverage.
- #57: live media-matrix runner.
- #61: storage driver settings UI.

Open work that still matters:

- #63: runtime default correction and browser evaluation unblocker.
- #40: actual Tailnet Evo X2 quality/runtime scoring across media.
- #27 and #28: history picker plus readable brief/style/draft artifacts.
- #13: browser E2E over the now-stable UI flows.
- #14: umbrella for queryable product memory beyond the schema foundation.
- #36 and #45: fallback quality and llama.cpp swap as P2 runtime work.
- #15: desktop/app-like packaging after the browser workflow is stable.

## DDD alignment

The implementation is partly aligned with DDD:

- Domain concepts are explicit: `internal/domain/persona`, `internal/domain/format`, `internal/domain/brief`, `internal/domain/author`, and `internal/domain/article`.
- Application services own core workflow behavior: `internal/application/brief` handles interview progression and fork-on-edit; `internal/application/draft` handles prompt assembly, validation, section regeneration, scoring, and lightweight verification.
- Infrastructure adapters are separated for LLM access, source fetching, JSON memory, and SQLite persistence.
- HTTP handlers mostly translate request/response shapes into domain/application calls rather than embedding persona or format rules directly.

Known deviations remain:

- `internal/handlers/workflow.go` is still too large and coordinates store lookup, runtime construction, SSE, compatibility handlers, and application calls in one file.
- LLM clients are constructed directly in handlers. A runtime provider/use-case boundary would make Evo X2/fallback behavior easier to test and configure from the UI.
- The SQLite repository persists the right data, but the UI does not yet expose projects, article history, draft versions, verification history, or source snapshots as queryable product memory. That is why #14 stays open.
- Runtime configuration now has storage UI parity, but LLM endpoint/fallback configuration is still env/default driven. #63 fixes the default and visibility problem; a future settings surface can make runtime selection explicit.
- The frontend is still one static JavaScript file. That is acceptable for the current local-first prototype, but #13 should lock behavior with browser E2E before #27/#28 add more stateful UI.

Conclusion: the domain model and application services match ADR 0002 well enough to continue. The largest architectural risk is not the domain vocabulary; it is handler-led orchestration and hidden runtime configuration. The next implementation sequence should reduce those two risks before the full Evo X2 media-matrix evaluation.
