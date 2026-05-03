# Issue #36 local llama.cpp fallback status - 2026-05-03

## Summary for Issue/PR

Issue #36 is substantially implemented from the validation-tooling side, but it should not close yet because no real local llama.cpp fallback draft run was executed on available local hardware/model runtime.

The new dedicated fallback validation command is in place:

```sh
make scenario-local-llamacpp-fallback
```

The live draft run is intentionally gated and requires:

```sh
RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 \
LOCAL_LLAMACPP_FALLBACK_BASE_URL=http://127.0.0.1:8081/v1 \
LOCAL_LLAMACPP_FALLBACK_MODEL=qwen3:30b-a3b \
LOCAL_LLAMACPP_FALLBACK_VERIFY_MODEL=qwen3:30b-a3b \
LOCAL_LLAMACPP_FALLBACK_LOAD_FLAGS='--host 127.0.0.1 --port 8081 --alias qwen3:30b-a3b --reasoning off ...' \
make scenario-local-llamacpp-fallback
```

Without `RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1`, the command records a plan/report only. It does not start Ollama, does not start `llama-server`, and does not run heavy local inference.

## Why this cannot close yet

The dry-run validation checked the configured loopback endpoint:

- Base URL: `http://127.0.0.1:8081/v1`
- Model: `qwen3:30b-a3b`
- Verify model: `qwen3:30b-a3b`
- Output report: `tmp/local_llamacpp_fallback/report.md`
- JSON report: `tmp/local_llamacpp_fallback/report.json`

The endpoint was not available:

```text
list llama.cpp models: Get "http://127.0.0.1:8081/v1/models": dial tcp 127.0.0.1:8081: connect: connection refused
```

Because `/v1/models` returned `connection refused`, the runner correctly stayed in `plan_only` mode and did not attempt draft generation. This means the implementation path exists, but there is still no hardware/model evidence that local llama.cpp fallback can meet the draft quality gate.

## Pass/fail thresholds for closure

To close #36, a future live run must record all of the following in `tmp/local_llamacpp_fallback/report.json`:

| Field | Required value |
|---|---:|
| `status` | `passed` |
| `explicit_run` | `true` |
| `runtime.local_fallback_only` | `true` |
| `runtime.fallback_chain_disabled` | `true` |
| `endpoint.available` | `true` |
| `thresholds.min_style_score` | `82.0` |
| `thresholds.min_keyword_overlap` | `70` |
| `thresholds.min_draft_runes` | `2800` |
| `result.scenario_passed` | `true` |
| `result.score` | `>= 82.0` |
| `result.keyword_overlap` | `>= 70` |
| `result.runes` | `>= 2800` |
| `result.verification_passed` | `true` when verification is performed |

The report must also preserve the exact base URL, model alias, verify model, elapsed seconds, score, keyword overlap, runes, and `LOCAL_LLAMACPP_FALLBACK_LOAD_FLAGS` used for the real hardware/model run.

## Evo X2 primary safety

This validation path is separate from Evo X2 primary validation:

- The default primary remains Evo X2 Ollama through `make scenario-evo-x2`.
- The local fallback runner accepts only loopback hosts such as `127.0.0.1`, `localhost`, or `::1`.
- Evo X2 hostnames are rejected by the local fallback runner.
- The runner clears the normal fallback chain before draft generation, so a local fallback result cannot be confused with Evo X2 primary or Evo X2 llama.cpp fallback.
- Plan mode is the default unless the explicit live env var is present.

Therefore, the #36 tooling does not block or alter the Evo X2 primary path.

## Validation performed

Commands run after the implementation:

```sh
go test ./cmd/scenario/local_llamacpp_fallback
make scenario-local-llamacpp-fallback
```

Expected current outcome:

- The Go test passes.
- The Make target succeeds in `plan_only` mode.
- The Make target records `connection refused` if no local llama.cpp endpoint is listening on `127.0.0.1:8081`.

## Closure decision

Do not close #36 yet. Close it only after a gated live run against an already-running local llama.cpp endpoint records `status=passed`, `result.score >= 82.0`, `result.keyword_overlap >= 70`, `result.runes >= 2800`, and the exact load flags used for the model.
