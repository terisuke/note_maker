# Issue #45 - Evo X2 llama.cpp model swap orchestration

Issue: [#45](https://github.com/terisuke/note_maker/issues/45)

## Decision

Keep Evo X2 Ollama as the primary runtime. Use Evo X2 `llama-server` only as a
single active fallback profile behind the existing Tailnet/Caddy path:

```text
primary:  http://evo-x2.tailb30e58.ts.net/v1
fallback: http://evo-x2.tailb30e58.ts.net/llama/v1
```

Do not use llama.cpp as the primary multi-model router yet. The app needs
phase-specific model selection for style, brief, article, draft, and verify
calls; Ollama already supports that operational model per OpenAI-compatible
request. The conservative llama.cpp path is an explicit service-profile swap:

1. define one systemd service for the shared fallback path,
2. keep profile env files under `/etc/note-maker/llama-cpp/`,
3. point `/etc/note-maker/llama-cpp/active.env` at the selected profile,
4. restart only the llama.cpp fallback service,
5. verify `/llama/health` and `/llama/v1/models`,
6. run brief/draft scenario validation before changing app fallback defaults.

This keeps Ollama and other Evo X2 users unaffected unless they explicitly use
the `/llama/v1` fallback endpoint during the maintenance window.

## Rationale

The current upstream `llama-server` documentation says `/v1/models` returns the
loaded model and that the returned list has one element. It also documents
`--alias` as the way to expose a custom model id. That model fits a stable
"one active fallback alias" service better than per-phase dynamic routing.

Upstream has newer router/profile work, but current issue traffic still shows
operational risk around router mode and multiple model handling. In particular,
a recent router-mode bug report described extra GPU memory use when loading
models through `--models-preset` / model-directory router mode. Until Evo X2
validation proves memory headroom and first-token latency with Ollama still
serving primary traffic, multi-instance or router-mode llama.cpp should remain
out of the default path.

Primary sources:

- `llama-server` README, `/v1/models` and `--alias`: <https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md#openai-compatible-api-endpoints>
- Multiple alias feature request: <https://github.com/ggml-org/llama.cpp/issues/17860>
- Router-mode GPU memory regression report: <https://github.com/ggml-org/llama.cpp/issues/21692>

## Recommended service shape

Example systemd unit on Evo X2:

```ini
[Unit]
Description=Note Maker llama.cpp fallback
After=network-online.target

[Service]
EnvironmentFile=/etc/note-maker/llama-cpp/active.env
ExecStart=/usr/local/bin/llama-server \
  --model ${LLAMA_CPP_MODEL_PATH} \
  --alias ${LLAMA_CPP_ALIAS} \
  --host ${LLAMA_CPP_HOST} \
  --port ${LLAMA_CPP_PORT} \
  --ctx-size ${LLAMA_CPP_CTX_SIZE} \
  ${LLAMA_CPP_EXTRA_ARGS}
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Example profile:

```bash
LLAMA_CPP_MODEL_PATH=/srv/models/gemma-4-E2B-it-Q8_0.gguf
LLAMA_CPP_ALIAS=gemma-4-E2B-it-Q8_0.gguf
LLAMA_CPP_HOST=127.0.0.1
LLAMA_CPP_PORT=18081
LLAMA_CPP_CTX_SIZE=8192
LLAMA_CPP_EXTRA_ARGS=--jinja
```

Caddy should continue to publish only the fallback path, for example:

```text
/llama/* -> http://127.0.0.1:18081/*
```

## Phase aliases

Because one active llama.cpp model is exposed at a time, the fallback model env
vars should all point at the active alias for validation:

```bash
EVO_X2_LLAMA_CPP_MODEL=gemma-4-E2B-it-Q8_0.gguf
EVO_X2_LLAMA_CPP_STYLE_MODEL=gemma-4-E2B-it-Q8_0.gguf
EVO_X2_LLAMA_CPP_BRIEF_MODEL=gemma-4-E2B-it-Q8_0.gguf
EVO_X2_LLAMA_CPP_ARTICLE_MODEL=gemma-4-E2B-it-Q8_0.gguf
EVO_X2_LLAMA_CPP_DRAFT_MODEL=gemma-4-E2B-it-Q8_0.gguf
EVO_X2_LLAMA_CPP_VERIFY_MODEL=gemma-4-E2B-it-Q8_0.gguf
```

If a profile is intended only for a narrow phase, keep it out of the app's
default fallback chain and invoke it only through explicit scenario commands.

## Operations commands

Inspect active primary/fallback models and the configured remote service:

```bash
make evo-x2-llama-status
```

Print the selected start/swap plan without changing Evo X2:

```bash
make evo-x2-llama-plan
```

Dry-run a start:

```bash
make evo-x2-llama-start
```

Apply a start only when the service is known to be safe:

```bash
EVO_X2_LLAMA_CPP_APPLY=1 make evo-x2-llama-start
```

Dry-run a profile swap:

```bash
EVO_X2_LLAMA_CPP_PROFILE=fallback-gemma-e2b make evo-x2-llama-swap
```

Apply a profile swap only in a maintenance window:

```bash
EVO_X2_LLAMA_CPP_APPLY=1 \
EVO_X2_LLAMA_CPP_ALLOW_RESTART=1 \
EVO_X2_LLAMA_CPP_PROFILE=fallback-gemma-e2b \
make evo-x2-llama-swap
```

## Validation gate

The llama.cpp fallback cannot replace Ollama until it passes the same runtime
signals used around #18's streaming work and current full workflow scenarios:

- brief interview completes against the active fallback endpoint,
- streamed draft generation reports `first_chunk_ms` and `chunks`,
- `scenario_passed=true`,
- `score >= SCENARIO_MIN_STYLE_SCORE`,
- `runes >= SCENARIO_MIN_DRAFT_RUNES`,
- final verification passes when performed,
- report includes `llm_base_url`, `llm_model`, `verify_model`, and elapsed time.

Live validation is intentionally gated:

```bash
RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 make scenario-evo-x2-llama-brief-draft
```

This command sets `LLM_BASE_URL` directly to `/llama/v1` and clears fallback
URLs so the run cannot silently pass through Ollama.

## ADR / plan alignment

This strategy is consistent with ADR 0001, ADR 0002, and the implementation
plans because it preserves the runtime order:

1. Evo X2 Ollama Tailnet primary.
2. Evo X2 llama.cpp fallback through `/llama/v1`.
3. Workstation-local llama.cpp as last resort.

It also preserves the current phase-model split. llama.cpp is not treated as a
drop-in per-phase router because the selected conservative service shape exposes
one active alias at a time. Any future multi-instance or router-mode approach
must be validated as a separate operational change with memory/latency evidence
while Ollama is still serving primary traffic.

The #45 implementation therefore satisfies design/script readiness and proves
that the direct `/llama/v1` route can complete a brief/draft scenario. It does
not satisfy promotion readiness: the 2026-05-03 live run missed keyword overlap
(`65 / 70`) and first-chunk latency (`14105ms`) gates. Keep the issue open
until the pending criteria in
`docs/validation/issue-45-llamacpp-swap-orchestration-2026-05-03.md` pass.
