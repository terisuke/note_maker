# Issue #36 local llama.cpp fallback validation template

Date: YYYY-MM-DD

## Purpose

Validate only the workstation-local llama.cpp fallback path. This is not Evo X2 primary validation and must not use the normal fallback chain.

## Required live gate

Live draft generation requires:

```sh
RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1
```

Without that variable, the runner writes a plan/report and exits without starting or using a local model. The runner never starts Ollama or `llama-server`.

## Endpoint and load flags

Record the exact already-running llama.cpp endpoint:

- Base URL: `http://127.0.0.1:8081/v1`
- Model alias: `qwen3:30b-a3b`
- Load flags: `PASTE llama-server flags here`

Example command to record a plan or unavailable-endpoint report:

```sh
make scenario-local-llamacpp-fallback
```

Example live command after the local llama.cpp endpoint is already available:

```sh
RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 \
LOCAL_LLAMACPP_FALLBACK_BASE_URL=http://127.0.0.1:8081/v1 \
LOCAL_LLAMACPP_FALLBACK_MODEL=qwen3:30b-a3b \
LOCAL_LLAMACPP_FALLBACK_VERIFY_MODEL=qwen3:30b-a3b \
LOCAL_LLAMACPP_FALLBACK_LOAD_FLAGS='--host 127.0.0.1 --port 8081 --alias qwen3:30b-a3b --reasoning off ...' \
make scenario-local-llamacpp-fallback
```

## Required artifacts

The dedicated runner writes:

- Report: `tmp/local_llamacpp_fallback/report.md`
- JSON: `tmp/local_llamacpp_fallback/report.json`
- Draft: `tmp/local_llamacpp_fallback/draft_generation/draft.md`
- Evaluation: `tmp/local_llamacpp_fallback/draft_generation/evaluation.json`
- Verification: `tmp/local_llamacpp_fallback/draft_generation/verification.json`
- Command logs: `tmp/local_llamacpp_fallback/logs/*.stdout` and `*.stderr`

## Acceptance threshold

Record the final values from `report.json`:

| Field | Required |
|---|---:|
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

## Result

Paste the actual run summary:

- Status:
- Base URL:
- Model:
- Load flags:
- Elapsed seconds:
- Score / min:
- Keyword overlap / min:
- Runes / min:
- Verification:
- Issue #36 closure decision:
