# Issue #90 - llama-swap dry-run and local validation

Date: 2026-05-04
Branch: `feat/d2-3-grid-scaffold`

## Scope

Validate that Evo X2 llama.cpp swap adoption remains dry-run by default, uses
an explicit `APPLY=1` gate for remote mutation, and has a network-free local
check suitable for routine validation.

The old systemd profile-symlink path is retained only as a manual diagnostic.
The issue #90 adoption path is llama-swap behind `/llama/v1`, validated locally
first and then through live-gated scenario targets during a maintenance window.

The cut now also validates the checked-in llama-swap template at
`deploy/llama-swap/evo-x2.example.yaml`, which maps the required three model
IDs:

- `gemma4:e2b`
- `qwen3.6:27b`
- `gemma4:31b`

The template may also expose `gemma4:latest` as an optional compatibility alias
when both the model key and `llama-server --alias` match.

## Commands

```bash
make evo-x2-llama-check
```

Result: passed.

- `bash -n` accepted `scripts/dev.sh`,
  `scripts/evo-x2-tailnet-preflight.sh`, `scripts/evo-x2-ssh-preflight.sh`,
  and `scripts/evo-x2-llama-swap.sh`.
- `scripts/evo-x2-llama-swap.sh validate` passed without curl, ssh, or
  systemd calls.
- The validator confirmed distinct primary/fallback URLs, `APPLY=0` dry-run
  default, `ALLOW_RESTART=0` restart gate default, deterministic start/swap
  command construction, symlink update command shape, and service-specific
  restart command shape.
- The validator confirmed all required llama-swap model aliases and matching
  `llama-server --alias` values exist in the template, backend ports are
  unique, backends bind to localhost, and active config lines do not call
  Ollama, systemctl, sudo, or ssh.

```bash
./scripts/evo-x2-llama-swap.sh start
```

Result: passed.

- Printed the start command:
  `sudo systemctl start 'note-maker-llama-cpp.service'`
- Exited dry-run before ssh with:
  `Dry-run only. Set APPLY=1 or EVO_X2_LLAMA_CPP_APPLY=1 to run this on evo-x2.`

```bash
APPLY=1 ./scripts/evo-x2-llama-swap.sh swap
```

Result: passed as a refusal gate. Exit code: `2`.

- Printed the swap command:
  `sudo ln -sfn '/etc/note-maker/llama-cpp/fallback-gemma-e2b.env' '/etc/note-maker/llama-cpp/active.env' && sudo systemctl restart 'note-maker-llama-cpp.service'`
- Refused before ssh because `ALLOW_RESTART=1` was not set.

```bash
make -n APPLY=1 evo-x2-llama-start
make -n APPLY=1 ALLOW_RESTART=1 evo-x2-llama-swap
```

Result: passed.

- `evo-x2-llama-start` expands `EVO_X2_LLAMA_CPP_APPLY="1"`.
- `evo-x2-llama-swap` expands both `EVO_X2_LLAMA_CPP_APPLY="1"` and
  `EVO_X2_LLAMA_CPP_ALLOW_RESTART="1"`.

## Conclusion

The llama-swap operator path is dry-run by default. Remote mutation requires
`APPLY=1`; profile swaps additionally require `ALLOW_RESTART=1`. The
`make evo-x2-llama-check` target now provides network-free local validation for
the gate and command plan.

Additional live-gated targets are now available:

- `scenario-evo-x2-llama-swap-brief-draft`
- `scenario-evo-x2-llama-swap-media-matrix`

Both point directly at `/llama/v1`, clear fallback URLs to avoid silent Ollama
success, use exactly the three required llama-swap model IDs, and set the #90
gates for style score, keyword overlap, first chunk latency, and draft length.

The media-matrix target explicitly selects the full five-format validation set:

- `terisuke_note_essay`
- `cor_blog_technical_report`
- `cor_blog_vision_sharing`
- `cloudia_zenn_tutorial`
- `cloudia_qiita_how_to`
- `cor_homepage_section`

## Additional local tests

```bash
go test ./internal/infrastructure/llamacpp ./cmd/scenario/draft_generation
go test ./cmd/scenario/media_matrix ./cmd/scenario/live_media_matrix
make -n scenario-evo-x2-llama-swap-brief-draft scenario-evo-x2-llama-swap-media-matrix
```

Result: passed.

- Added local crash-chain coverage proving non-streaming and streaming clients
  skip a crashed intermediate fallback and recover on the next fallback.
- Confirmed the llama-swap targets are still live-gated and expand direct
  `/llama/v1` URLs with empty fallback env vars.

## Remaining live validation

Operator checklist:
`docs/operations/llama-swap-maintenance-checklist.md`

Do not run the remaining live validation outside a maintenance window. The
remaining checks are the 3-model brief/draft scenario and the five-format
llama-swap media matrix against Evo X2 `/llama/v1`.

## Non-mutating latency hardening update - 2026-05-04

Implemented without SSH, service restart, Ollama stop, or Tailscale changes:

- `cmd/scenario/draft_generation` now writes `preflight.json` and `warmup.json`.
- When `SCENARIO_MAX_FIRST_CHUNK_MS` is set, the draft scenario defaults to:
  - read-only `/models` preflight against the configured DRAFT client,
  - required alias check for the phase models on `/llama/v1`,
  - tiny streaming warmup before the measured draft generation.
- The measured `first_chunk_ms` remains the first chunk of the real draft
  request, not the warmup request.
- `cmd/scenario/live_media_matrix` now parses and reports
  `keyword_overlap`, `min_keyword_overlap`, `first_chunk_ms`, and
  `max_first_chunk_ms` in aggregate JSON/Markdown quality gates.

Latest known non-mutating live result before this hardening:

| Metric | Value |
|---|---:|
| score | `84.2 / 82.0` |
| keyword_overlap | `75 / 70` |
| runes | `3096 / 2800` |
| verification_passed | `true` |
| first_chunk_ms | `16868 / 8000` |

Outcome: quality gates passed except first-chunk latency. #90 cannot close
until a maintenance-window run records `scenario_passed=true` with
`first_chunk_ms <= 8000`, then the fallback-only five-format matrix and crash
fallback acceptance are recorded.

Additional non-live commands passed after this update:

```bash
go test ./internal/infrastructure/llamacpp ./cmd/scenario/draft_generation ./cmd/scenario/live_media_matrix
make evo-x2-llama-check
go test ./cmd/scenario/brief_interview ./cmd/scenario/local_llamacpp_fallback ./cmd/scenario/media_matrix ./cmd/scenario/live_media_matrix
make -n scenario-evo-x2-llama-swap-brief-draft scenario-evo-x2-llama-swap-media-matrix
```
