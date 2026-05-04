# Evo X2 llama-swap maintenance-window checklist

Use this checklist for the remaining issue #90 live validation. Do not run it
outside a scheduled maintenance window.

## Preconditions

- Confirm no other Evo X2 users are relying on `/llama/v1`.
- Confirm Ollama primary remains available at `http://evo-x2.tailb30e58.ts.net/v1`.
- Confirm llama-swap will own only the fallback path
  `http://evo-x2.tailb30e58.ts.net/llama/v1`.
- Confirm the deployed llama-swap config is derived from
  `deploy/llama-swap/evo-x2.example.yaml`.
- Confirm the active config exposes these three model IDs:
  `gemma4:e2b`, `qwen3.6:27b`, and `gemma4:31b`.
- Confirm backend `llama-server` ports are localhost-only and unique.

## Local checks before touching Evo X2

```bash
make evo-x2-llama-check
go test ./internal/infrastructure/llamacpp ./cmd/scenario/draft_generation
```

Expected result:

- no ssh, systemctl, sudo, curl, or Ollama mutation from local validation,
- llama-swap template validation passes,
- fallback and crash-chain client tests pass.

## Live validation sequence

1. Start or reload llama-swap through the operator's normal Evo X2 maintenance
   procedure. Do not stop Ollama.
2. Check fallback health and model visibility from the maintenance terminal:

   ```bash
   curl -fsS http://evo-x2.tailb30e58.ts.net/llama/health
   curl -fsS http://evo-x2.tailb30e58.ts.net/llama/v1/models
   ```

3. Run the 3-model direct fallback validation:

   ```bash
   RUN_EVO_X2_LLAMA_SWAP_SCENARIO=1 make scenario-evo-x2-llama-swap-brief-draft
   ```

   The draft phase performs a read-only `/models` preflight and a tiny
   streaming warmup before the measured draft request whenever
   `SCENARIO_MAX_FIRST_CHUNK_MS` is set. Record these emitted fields alongside
   the draft metrics:

   - `preflight_passed`
   - `preflight_models`
   - `preflight_required_models`
   - `warmup_passed`
   - `warmup_first_chunk_ms`

4. Run the five-format direct fallback matrix:

   ```bash
   RUN_EVO_X2_LLAMA_SWAP_MEDIA_MATRIX=1 make scenario-evo-x2-llama-swap-media-matrix
   ```

5. Record artifacts from:

   - `tmp/media_matrix/llama_swap/aggregate.json`
   - `tmp/media_matrix/llama_swap/aggregate.md`
   - the `SCENARIO_OUTPUT_DIR` emitted by the brief/draft run.

## Pass criteria

- All live requests use `http://evo-x2.tailb30e58.ts.net/llama/v1`.
- Fallback env vars are empty in the live-gated targets, so success cannot
  silently route through Ollama.
- Preflight reports all required model aliases:
  `gemma4:e2b`, `qwen3.6:27b`, and `gemma4:31b`.
- Warmup succeeds before the measured draft request.
- `scenario_passed=true`.
- `score >= 82`.
- `keyword_overlap >= 70`.
- `first_chunk_ms <= 8000`.
- `runes >= 2800` for long-form draft validation.
- Final verification passes when performed.
- Ollama primary remains available after the run.

## Rollback

- Remove llama-swap from the `/llama/v1` Caddy route or restore the previous
  fallback route.
- Leave Ollama running.
- Do not use the legacy systemd profile-swap path except as a manual diagnostic
  in the same maintenance window.
