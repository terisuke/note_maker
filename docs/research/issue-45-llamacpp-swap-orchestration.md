# Issue #45 - Evo X2 llama.cpp model swap orchestration

Issue: [#45](https://github.com/terisuke/note_maker/issues/45)

## Decision

Keep Evo X2 Ollama as the primary runtime. For issue #90, adopt `llama-swap`
as the `/llama/v1` fallback router in front of Evo X2 `llama-server` backends:

```text
primary:  http://evo-x2.tailb30e58.ts.net/v1
fallback: http://evo-x2.tailb30e58.ts.net/llama/v1
```

Do not use llama.cpp as the primary multi-model router yet. The app needs
phase-specific model selection for style, brief, article, draft, and verify
calls; Ollama already supports that operational model per OpenAI-compatible
request. The current conservative fallback adoption path is:

1. validate the checked-in llama-swap template locally,
2. run llama-swap behind the existing `/llama/v1` path during a maintenance
   window,
3. route only live-gated validation traffic directly to `/llama/v1`,
4. keep application fallback defaults unchanged until the gates pass.

The previous explicit systemd service-profile swap is now demoted to a manual
diagnostic path only:

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

## Legacy diagnostic service shape

The systemd shape below is retained for diagnosing the older one-active-profile
fallback service. It is not the llama-swap adoption path and should not be used
for routine model routing.

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

## Legacy phase aliases

For the demoted one-active-profile diagnostic path, the fallback model env vars
should all point at the active alias for validation:

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

Run local validation without network, ssh, curl, or systemd:

```bash
make evo-x2-llama-check
```

Inspect active primary/fallback models and the configured legacy service:

```bash
make evo-x2-llama-status
```

Print the selected legacy start/swap plan without changing Evo X2:

```bash
make evo-x2-llama-plan
```

Dry-run a legacy diagnostic start:

```bash
make evo-x2-llama-start
```

Apply a legacy diagnostic start only during a maintenance window:

```bash
APPLY=1 make evo-x2-llama-start
```

Dry-run a legacy diagnostic profile swap:

```bash
EVO_X2_LLAMA_CPP_PROFILE=fallback-gemma-e2b make evo-x2-llama-swap
```

Apply a legacy diagnostic profile swap only during a maintenance window:

```bash
APPLY=1 \
ALLOW_RESTART=1 \
EVO_X2_LLAMA_CPP_PROFILE=fallback-gemma-e2b \
make evo-x2-llama-swap
```

Run the live-gated llama-swap 3-model validation:

```bash
RUN_EVO_X2_LLAMA_SWAP_SCENARIO=1 make scenario-evo-x2-llama-swap-brief-draft
```

Run the live-gated llama-swap five-format matrix:

```bash
RUN_EVO_X2_LLAMA_SWAP_MEDIA_MATRIX=1 make scenario-evo-x2-llama-swap-media-matrix
```

`EVO_X2_LLAMA_CPP_APPLY=1` and
`EVO_X2_LLAMA_CPP_ALLOW_RESTART=1` remain supported for automation, but the
operator-facing Makefile gate is `APPLY=1` plus `ALLOW_RESTART=1` for swaps.
Without `APPLY=1`, `start` and `swap` print the remote command and exit before
ssh. With `APPLY=1` but without `ALLOW_RESTART=1`, `swap` refuses before ssh.

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

Legacy one-active-profile live validation remains intentionally gated:

```bash
RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 make scenario-evo-x2-llama-brief-draft
```

This command sets `LLM_BASE_URL` directly to `/llama/v1` and clears fallback
URLs so the run cannot silently pass through Ollama.

The issue #90 llama-swap targets use the same direct `/llama/v1` and no-fallback
properties, but validate three model IDs (`gemma4:e2b`, `qwen3.6:27b`,
`gemma4:31b`) across the brief/draft path and all five registered output
formats.

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
