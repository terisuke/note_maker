# Issue #45 llama.cpp swap orchestration validation - 2026-05-03

Issue: [#45](https://github.com/terisuke/note_maker/issues/45)

## Scope

This validation covers runtime operations design and local safety checks for
Evo X2 llama.cpp fallback orchestration. It does not restart or kill Evo X2
services.

## Implemented controls

- `scripts/evo-x2-llama-swap.sh inspect` reads the Ollama primary models,
  llama.cpp fallback models, and remote systemd status when SSH is available.
- `scripts/evo-x2-llama-swap.sh plan` prints the systemd profile strategy and
  start/swap commands without changing Evo X2.
- `scripts/evo-x2-llama-swap.sh start` is dry-run by default and requires
  `EVO_X2_LLAMA_CPP_APPLY=1` for remote execution.
- `scripts/evo-x2-llama-swap.sh swap` is dry-run by default and requires both
  `EVO_X2_LLAMA_CPP_APPLY=1` and `EVO_X2_LLAMA_CPP_ALLOW_RESTART=1` before it
  restarts the shared llama.cpp fallback service.
- The script refuses to operate if the Ollama primary URL and llama.cpp fallback
  URL are accidentally identical.

## Make targets

```bash
make evo-x2-llama-status
make evo-x2-llama-plan
make evo-x2-llama-start
make evo-x2-llama-swap
make evo-x2-llama-check
RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 make scenario-evo-x2-llama-brief-draft
```

The live brief/draft scenario is explicitly gated by
`RUN_EVO_X2_LLAMA_CPP_SCENARIO=1`. It targets
`EVO_X2_LLAMA_CPP_LLM_BASE_URL` directly and clears fallback URLs, so it cannot
silently pass through Ollama.

## Local validation commands

```bash
make evo-x2-llama-check
make -n evo-x2-llama-status
make -n evo-x2-llama-plan
make -n evo-x2-llama-start
make -n evo-x2-llama-swap
make -n scenario-evo-x2-llama-brief-draft
```

Results:

- `make evo-x2-llama-check`: passed; `bash -n` accepted `scripts/dev.sh`,
  `scripts/evo-x2-tailnet-preflight.sh`, `scripts/evo-x2-ssh-preflight.sh`,
  and `scripts/evo-x2-llama-swap.sh`.
- `make -n evo-x2-llama-status`: passed; printed the inspect command only.
- `make -n evo-x2-llama-plan`: passed; printed the plan command only.
- `make -n evo-x2-llama-start`: passed; printed the dry-run start command only.
- `make -n evo-x2-llama-swap`: passed; printed the dry-run swap command only.
- `make -n scenario-evo-x2-llama-brief-draft`: passed; printed the env-gated
  scenario command without running it.
- `./scripts/evo-x2-llama-swap.sh plan`: passed; printed start/swap commands
  and post-check curls without SSH or service changes.
- `./scripts/evo-x2-llama-swap.sh start`: passed; dry-run only.
- `./scripts/evo-x2-llama-swap.sh swap`: passed; dry-run only.
- `EVO_X2_LLAMA_CPP_APPLY=1 ./scripts/evo-x2-llama-swap.sh swap`: refused
  before SSH because `EVO_X2_LLAMA_CPP_ALLOW_RESTART=1` was not set.
- `make scenario-evo-x2-llama-brief-draft`: refused before preflight because
  `RUN_EVO_X2_LLAMA_CPP_SCENARIO=1` was not set.
- URL guard check with identical Ollama and llama.cpp URLs: refused with exit 2.
- `make evo-x2-llama-status`: read-only check passed. Ollama primary reported
  installed models including `gemma4:31b`, `gemma4:e2b`, `gemma4:latest`,
  `qwen3.6:27b`, `qwen3:30b-a3b`, and `gpt-oss:120b`. The llama.cpp fallback
  at `/llama/v1/models` reported one active model:
  `gemma-4-E2B-it-Q8_0.gguf`. The default configured systemd service name
  `note-maker-llama-cpp.service` reported `inactive`, so Evo X2's actual
  service name should be supplied through `EVO_X2_LLAMA_CPP_SERVICE` before any
  apply-mode start/swap command.
- `go test ./...`: passed.

## Live validation command

Use the same brief/draft scenario metrics used around #18 streaming and the
current Evo X2 full workflow checks:

```bash
RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 \
EVO_X2_LLAMA_CPP_MODEL=gemma-4-E2B-it-Q8_0.gguf \
EVO_X2_LLAMA_CPP_STYLE_MODEL=gemma-4-E2B-it-Q8_0.gguf \
EVO_X2_LLAMA_CPP_BRIEF_MODEL=gemma-4-E2B-it-Q8_0.gguf \
EVO_X2_LLAMA_CPP_DRAFT_MODEL=gemma-4-E2B-it-Q8_0.gguf \
EVO_X2_LLAMA_CPP_VERIFY_MODEL=gemma-4-E2B-it-Q8_0.gguf \
make scenario-evo-x2-llama-brief-draft
```

Required reported metrics:

- `scenario_passed`
- `score`
- `min_style_score`
- `runes`
- `min_draft_runes`
- `verification_performed`
- `verification_passed`
- `elapsed_seconds`
- `streaming`
- `first_chunk_ms`
- `chunks`
- `llm_base_url`
- `llm_model`
- `verify_model`

## Live Validation Pending Criteria

Use this checklist as the concrete issue comment/update before closing #45.

Preflight and service identity:

- Record the real Evo X2 llama.cpp systemd unit name and pass it as
  `EVO_X2_LLAMA_CPP_SERVICE`; the placeholder
  `note-maker-llama-cpp.service` reported `inactive` during the read-only check.
- Record the active profile name, active env path, model path or HF file,
  exposed alias, context size, and GPU/Vulkan/RADV flags.
- Run `make evo-x2-llama-status` before the scenario and paste the Ollama
  primary `/v1/models` result plus the llama.cpp `/llama/v1/models` active
  model. The llama.cpp result must show exactly the alias being validated.
- If a profile swap is required, perform it only with both
  `EVO_X2_LLAMA_CPP_APPLY=1` and `EVO_X2_LLAMA_CPP_ALLOW_RESTART=1`, and record
  the maintenance window or operator confirmation. No command may kill or
  restart Ollama.

Direct llama.cpp scenario:

- Run `RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 make scenario-evo-x2-llama-brief-draft`
  or the equivalent expanded command from this document.
- Confirm the scenario output includes
  `llm_base_url=http://evo-x2.tailb30e58.ts.net/llama/v1` or the configured
  `EVO_X2_LLAMA_CPP_LLM_BASE_URL`.
- Confirm `LLM_FALLBACK_BASE_URLS` is empty for the run so success cannot come
  from Ollama or workstation-local fallback.
- Attach or link the generated `tmp/author_style`, `tmp/brief_interview`, and
  `tmp/draft_generation` artifacts, especially `draft.md`, `evaluation.json`,
  and `verification.json`.

Pass gates:

- Brief phase completes against `/llama/v1`.
- Draft phase streams with `streaming=true`, `chunks > 0`, and a recorded
  `first_chunk_ms`.
- Warm first chunk meets the #18-style target of `first_chunk_ms <= 3000`; if
  it does not, keep #45 open and record the observed latency as an operations
  miss.
- `scenario_passed=true`.
- `score >= 80.0`.
- `runes >= 2800`.
- `verification_performed=true` and `verification_passed=true`, or an explicit
  reason is recorded if final verification is intentionally unavailable for the
  active llama.cpp profile.
- `elapsed_seconds`, `llm_model`, and `verify_model` are recorded.

Ollama safety gates:

- Run `make evo-x2-models` or equivalent Ollama `/v1/models` check after the
  llama.cpp scenario; it must still pass.
- The run must not change the app default primary from Ollama to llama.cpp.
- No other Evo X2 users should lose the shared Ollama Tailnet primary during
  the test. If there is any service interruption or resource starvation,
  record it and keep #45 open.

Close #45 only when every pass and safety gate above is satisfied. Otherwise,
comment on #45 with the failed gate, observed metrics, active profile, and next
specific follow-up.

## Live validation result - 2026-05-03

Command:

```bash
RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 make scenario-evo-x2-llama-brief-draft
```

Path controls:

- `llm_base_url=http://evo-x2.tailb30e58.ts.net/llama/v1`
- `LLM_FALLBACK_BASE_URLS=""`
- `STYLE/BRIEF/ARTICLE/DRAFT/VERIFY_LLM_FALLBACK_MODELS=""`
- No swap, restart, or apply-mode command was run.
- Post-run `make evo-x2-models` passed, so the Ollama primary endpoint remained
  available after the direct llama.cpp scenario.

Observed output:

| Metric | Value |
|---|---:|
| brief elapsed seconds | `42.59` |
| draft elapsed seconds | `59.33` |
| scenario_passed | `true` |
| score | `88.2 / 80.0` |
| keyword_overlap | `65 / 70` |
| runes | `3075 / 2800` |
| verification_performed | `true` |
| verification_passed | `true` |
| streaming | `true` |
| first_chunk_ms | `14105` |
| chunks | `1785` |
| llm_model | `gemma-4-E2B-it-Q8_0.gguf` |
| verify_model | `gemma-4-E2B-it-Q8_0.gguf` |

Artifacts:

- `tmp/author_style/profile.json`
- `tmp/author_style/guide.md`
- `tmp/brief_interview/brief.json`
- `tmp/draft_generation/draft.md`
- `tmp/draft_generation/evaluation.json`
- `tmp/draft_generation/verification.json`

Outcome:

- The direct `/llama/v1` scenario can complete without falling back to Ollama.
- Draft throughput and final verification were good enough for a fallback proof.
- It is not ready to promote over Ollama: strict style evaluation failed on
  `keyword_overlap=65 below 70`, and first chunk latency was `14105ms`, above
  the #18-style streaming target.

## Closure status

#45 remains pending until the live Evo X2 llama.cpp brief/draft validation runs
against the selected active profile and proves quality plus operations gates.
The 2026-05-03 live run proves the route works, but it missed keyword-overlap
and first-chunk gates. The current implementation is intentionally P2: Ollama
remains primary.
