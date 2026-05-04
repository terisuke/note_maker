# Open issues field-validation handoff

Date: 2026-05-04
Base branch: `develop`
Relevant merged work:

- PR #98: workspace UI, desktop scaffold, Docker/multi-user scaffold, llama-swap prep
- PR #99: release readiness, live preflight/warmup, corrected local fallback fixture

This handoff is for the remaining open issues that require field validation
rather than more source-tree scaffolding. The goal is to let an operator run
the experiments, capture evidence, and close only the issues whose acceptance
criteria are actually met.

## Current open issues

- #36: local llama.cpp fallback draft quality
- #45: Evo X2 llama.cpp swap orchestration evaluation
- #90: llama-swap adoption on Evo X2 fallback path
- #92: Wails v2 desktop distributable release
- #93: multi-user Docker deployment with Tailscale sidecar

## Shared rules

- Do not stop Ollama, restart Evo X2 services, or deploy llama-swap outside a
  maintenance window.
- Do not close an issue from source-tree checks alone when its acceptance
  requires real runtime, signing, GHCR, Tailscale, or platform evidence.
- Before commenting or closing an issue, rerun the relevant command and include
  exact timestamped metrics. The prior fact-check hook pattern expires quickly,
  so treat fresh terminal output as the source of truth.
- Keep `/llama/v1` fallback validation separate from the primary Ollama
  `/v1` path.
- Keep local fallback validation on loopback only. Do not point #36 at Evo X2
  or an Ollama local API.

## Baseline checks before field work

Run these from the repository root:

```sh
git checkout develop
git pull --ff-only origin develop
go test ./...
node --check static/js/script.js
python3 -m pytest tests/e2e -q
(cd cmd/desktop && go test ./... && go test -tags wails ./...)
docker compose config >/tmp/note-maker-compose.yml
docker compose --profile tailscale config >/tmp/note-maker-compose-tailscale.yml
RUN_MULTI_ARCH_BUILD=0 RUN_TAILSCALE_STARTUP=1 ./scripts/release-readiness-check.sh
git diff --check
```

Expected baseline:

- Go tests pass.
- E2E tests report 14 passed.
- Desktop package tests pass.
- Release readiness passes with `RUN_MULTI_ARCH_BUILD=0`.
- Worktree is clean except any intentionally ignored or user-owned local files.

## Issue #36: local llama.cpp fallback quality

### Purpose

Prove a local loopback llama.cpp fallback can generate a draft without relying
on Evo X2 or the local Ollama API.

### Current status

Source-tree fixes are merged:

- `cmd/scenario/brief_interview` now maps scripted scenario answers by question
  ID, so `must_include`, `exclusions`, `target_length_structure`, and
  `tone_stance` no longer shift.
- `cmd/scenario/local_llamacpp_fallback` forwards the keyword-overlap gate and
  records model, base URL, load flags, thresholds, draft, evaluation, and
  verification artifacts.
- Validation is documented in
  `docs/validation/issue-36-local-llamacpp-fallback-2026-05-04.md`.

The currently tried Gemma 3 27B Q8 GGUF did not pass. Streaming produced
`unexpected EOF`; non-streaming produced an EOF from the llama.cpp server before
draft quality could be evaluated.

### Required setup

Choose a llama.cpp-compatible local model that can load in `llama-server`.
Record:

- `llama-server` binary path
- model file path or source repo/file
- server flags
- model alias
- machine and GPU/CPU notes if relevant

Example server command:

```sh
/opt/homebrew/bin/llama-server \
  -m /path/to/model.gguf \
  --host 127.0.0.1 \
  --port 8081 \
  --alias local-model:alias \
  --ctx-size 8192 \
  --n-gpu-layers 999 \
  --jinja \
  --flash-attn auto
```

Confirm loopback:

```sh
curl -sS --max-time 5 http://127.0.0.1:8081/v1/models
```

### Validation command

```sh
/usr/bin/time -p env \
  RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 \
  LOCAL_LLAMACPP_FALLBACK_BASE_URL=http://127.0.0.1:8081/v1 \
  LOCAL_LLAMACPP_FALLBACK_MODEL=local-model:alias \
  LOCAL_LLAMACPP_FALLBACK_VERIFY_MODEL=local-model:alias \
  LOCAL_LLAMACPP_FALLBACK_LOAD_FLAGS='record the exact server flags here' \
  LOCAL_LLAMACPP_FALLBACK_TIMEOUT_SECONDS=1200 \
  LOCAL_LLAMACPP_FALLBACK_MAX_ATTEMPTS=2 \
  make scenario-local-llamacpp-fallback
```

If streaming is unstable, run one additional diagnostic pass:

```sh
/usr/bin/time -p env \
  RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 \
  LOCAL_LLAMACPP_FALLBACK_BASE_URL=http://127.0.0.1:8081/v1 \
  LOCAL_LLAMACPP_FALLBACK_MODEL=local-model:alias \
  LOCAL_LLAMACPP_FALLBACK_VERIFY_MODEL=local-model:alias \
  LOCAL_LLAMACPP_FALLBACK_STREAM_DRAFT=0 \
  LOCAL_LLAMACPP_FALLBACK_TIMEOUT_SECONDS=1200 \
  LOCAL_LLAMACPP_FALLBACK_MAX_ATTEMPTS=1 \
  make scenario-local-llamacpp-fallback
```

### Evidence to capture

- `tmp/local_llamacpp_fallback/report.md`
- `tmp/local_llamacpp_fallback/report.json`
- `tmp/local_llamacpp_fallback/draft_generation/draft.md`
- `tmp/local_llamacpp_fallback/draft_generation/evaluation.json`
- `tmp/local_llamacpp_fallback/draft_generation/verification.json`
- `llama-server` command and logs around load/generation

### Close criteria

Close #36 only when the report records:

- `status=passed`
- `result.score >= 82`
- `result.keyword_overlap >= 70`
- `result.runes >= 2800`
- verification passed when verification is performed
- base URL is loopback, not Evo X2 and not local Ollama

## Issues #45 and #90: Evo X2 llama.cpp / llama-swap

### Purpose

Evaluate and then deploy a llama-swap front door for Evo X2's llama.cpp
fallback path. #45 is the strategy/evaluation track. #90 is the adoption track
for actual llama-swap routing and parity gates.

### Current status

Source-tree prep is merged:

- `deploy/llama-swap/evo-x2.example.yaml` contains the expected model aliases.
- `scripts/evo-x2-llama-swap.sh validate` performs network-free template
  validation.
- legacy systemd profile-swap is demoted to manual diagnostic use.
- live scenario targets require explicit environment gates.
- PR #99 added `/models` preflight and a small warmup before measured draft
  generation.

Latest non-mutating live result:

```text
preflight_enabled=true
preflight_passed=false
preflight_models=gemma-4-E2B-it-Q8_0.gguf
preflight_required_models=gemma4:31b,gemma4:e2b,qwen3.6:27b
```

That means `/llama/v1` is still a single-model llama.cpp endpoint, not the
required llama-swap front door.

### Non-mutating checks

Run:

```sh
make evo-x2-llama-check
make -n scenario-evo-x2-llama-swap-brief-draft scenario-evo-x2-llama-swap-media-matrix
curl -sS --max-time 10 http://evo-x2.tailb30e58.ts.net/llama/v1/models
```

Expected before deployment:

- local config validation passes
- dry-run scenario commands render
- `/llama/v1/models` should show whether aliases are deployed

### Maintenance-window deployment checklist

Use `docs/operations/llama-swap-maintenance-checklist.md` as the operational
runbook. During the window:

1. Confirm Ollama primary `/v1/models` is healthy before touching fallback.
2. Install/start llama-swap using a config derived from
   `deploy/llama-swap/evo-x2.example.yaml`.
3. Route `/llama/v1` to llama-swap.
4. Verify `/llama/v1/models` exposes at least:
   - `gemma4:e2b`
   - `qwen3.6:27b`
   - `gemma4:31b`
5. Verify Ollama primary is still healthy.

### Live validation commands

Brief/draft 3-model switching:

```sh
/usr/bin/time -p env RUN_EVO_X2_LLAMA_SWAP_SCENARIO=1 \
  make scenario-evo-x2-llama-swap-brief-draft
```

Five-format fallback-only matrix:

```sh
/usr/bin/time -p env RUN_EVO_X2_LLAMA_SWAP_MEDIA_MATRIX=1 \
  make scenario-evo-x2-llama-swap-media-matrix
```

Crash/fallback-chain behavior:

```sh
go test ./internal/infrastructure/llamacpp
```

If doing a real crash drill, document exactly what process was stopped,
when it was restarted, and whether partial drafts were preserved.

### Evidence to capture

For #45:

- recommended strategy summary
- `/llama/v1/models` output
- brief/draft scenario stdout
- `score`, `keyword_overlap`, `runes`, `first_chunk_ms`,
  `verification_passed`
- Ollama primary postcheck

For #90:

- llama-swap deployment command/config checksum
- `/llama/v1/models` with the three required aliases
- brief/draft pass
- five-format matrix pass
- fallback-chain/crash evidence
- note that legacy systemd profile-swap remains manual diagnostic only

### Close criteria

Close #45 when the recommended strategy is documented and brief/draft live
validation passes quality and operation gates.

Close #90 when all of these are true:

- llama-swap is deployed during a maintenance window
- `/llama/v1/models` exposes required aliases
- brief/draft scenario passes with:
  - `score >= 82`
  - `keyword_overlap >= 70`
  - `runes >= 2800`
  - `first_chunk_ms <= 8000`
- five-format fallback-only matrix passes
- crash/fallback-chain behavior is recorded
- Ollama remains primary

## Issue #92: Wails v2 desktop release

### Purpose

Produce distributable desktop builds for macOS, Windows, and Linux using Wails
v2, with release workflow, checksums, and signing gates.

### Current status

Source-tree prep is merged:

- Wails v2 is pinned in `cmd/desktop/go.mod`.
- `make desktop-build` previously passed locally on macOS arm64.
- `desktop-release.yml` contains dry-run and release gates.
- `scripts/release-readiness-check.sh` records workflow visibility and secret
  availability.

Current external blockers:

- repository default branch is `main`
- new workflows are on `develop` until promoted
- visible repo secrets do not include Apple or Windows signing secrets

### Local checks

```sh
(cd cmd/desktop && go test ./... && go test -tags wails ./...)
make desktop-check
make desktop-build
RUN_MULTI_ARCH_BUILD=0 RUN_TAILSCALE_STARTUP=0 ./scripts/release-readiness-check.sh
```

Remove transient `build/` output before committing unless you are explicitly
archiving artifacts outside git.

### GitHub Actions dry-run

After workflow files are on `main`, run:

```sh
gh workflow run desktop-release.yml --repo terisuke/note_maker --ref main \
  -f dry_run=1 \
  -f release=0
```

Then inspect:

```sh
gh run list --repo terisuke/note_maker --workflow "Desktop Release" --limit 5
gh run view --repo terisuke/note_maker <run-id> --log
```

Download and record artifacts/checksums from the macOS, Windows, and Linux jobs.

### Signed release

Configure required secrets before `release=1`:

- `APPLE_CERTIFICATE`
- `APPLE_CERTIFICATE_PASSWORD`
- `APPLE_TEAM_ID`
- `APP_STORE_CONNECT_KEY_ID`
- `APP_STORE_CONNECT_ISSUER_ID`
- `APP_STORE_CONNECT_KEY`
- `WINDOWS_CERTIFICATE`
- `WINDOWS_CERTIFICATE_PASSWORD`

Run:

```sh
gh workflow run desktop-release.yml --repo terisuke/note_maker --ref main \
  -f dry_run=0 \
  -f release=1
```

### Platform smoke

For each artifact:

- launch the app
- verify the existing UI appears
- verify the data directory is created in the same Tier 1-compatible location
- verify `/healthz` or equivalent embedded handler path is reachable through
  the app
- verify LLM diagnostics can open without requiring internet at app startup
- exit cleanly

### Close criteria

Close #92 only after:

- dry-run workflow artifacts and checksums are produced on GitHub Actions
- signed/notarized macOS evidence is recorded
- signed Windows evidence is recorded
- Linux artifact checksum and launch smoke are recorded
- platform smoke passes on macOS, Windows, and Linux

## Issue #93: multi-user Docker deployment

### Purpose

Ship the Tier 3 Docker deployment with per-user data isolation, optional
Tailscale sidecar, and GHCR multi-arch publish/runtime evidence.

### Current status

Source-tree prep is merged:

- non-root Docker image
- Compose app service
- optional Tailscale sidecar profile
- `MULTI_USER=1` trusted-header mode
- SQLite `user_id` migration
- two-user e2e isolation test
- Docker publish workflow dry-run path
- Tailscale no-auth fail-fast behavior
- release readiness helper

Current external blockers:

- no real `TS_AUTHKEY` was provided during validation
- Docker publish workflow is not on `main` until promotion
- GHCR publish/runtime evidence is not recorded yet

### Local checks

```sh
docker build -t note-maker:dev .
docker image inspect note-maker:dev --format '{{.Size}} {{.Architecture}} {{.Os}}'
docker compose config
docker compose --profile tailscale config
python3 -m pytest tests/e2e/test_multi_user_isolation.py -q
RUN_MULTI_ARCH_BUILD=0 RUN_TAILSCALE_STARTUP=1 ./scripts/release-readiness-check.sh
```

Expected:

- image size under 100 MB
- app runs as non-root
- `/healthz` is public
- protected API returns 401 without trusted header under `MULTI_USER=1`
- protected API returns 200 with trusted header
- two-user isolation test passes

### Real Tailscale sidecar validation

Use an uncommitted shell or `.env` with a reusable auth key:

```sh
export TS_AUTHKEY='tskey-auth-...'
export TS_HOSTNAME='note-maker-test'
docker compose --profile tailscale up -d tailscale app-via-tailscale
```

Check:

```sh
docker compose --profile tailscale ps
docker compose --profile tailscale logs --tail=200 tailscale
docker compose --profile tailscale logs --tail=200 app-via-tailscale
curl -i http://127.0.0.1:${NOTE_MAKER_TAILSCALE_HOST_PORT:-8081}/healthz
curl -i http://127.0.0.1:${NOTE_MAKER_TAILSCALE_HOST_PORT:-8081}/api/models
```

Shut down:

```sh
docker compose --profile tailscale down
```

### GHCR dry-run and publish

After workflow files are on `main`, dry-run:

```sh
gh workflow run docker-publish.yml --repo terisuke/note_maker --ref main \
  -f dry_run=1
```

For real publish, push a release tag or dispatch without dry run, depending on
the chosen release process.

Verify manifest:

```sh
docker buildx imagetools inspect ghcr.io/terisuke/note_maker:<tag>
```

Smoke on target hosts:

```sh
docker run --rm -p 8080:8080 ghcr.io/terisuke/note_maker:<tag>
curl -i http://127.0.0.1:8080/healthz
```

Repeat on at least one `linux/amd64` host and one `linux/arm64` host.

### Close criteria

Close #93 only after:

- real `TS_AUTHKEY` sidecar startup passes
- app behind the sidecar serves `/healthz` and `/api/models`
- GHCR multi-arch publish succeeds
- manifest includes `linux/amd64` and `linux/arm64`
- runtime smoke passes on target architectures
- operations runbook is sufficient for start, upgrade, backup, restore, and
  Tailscale troubleshooting

## Suggested execution order

1. Promote `develop` to `main` or otherwise land workflow files on `main`.
2. Run #92 and #93 dry-run workflows from `main`.
3. Perform #93 real Tailscale sidecar validation.
4. Publish and smoke GHCR images for #93.
5. Configure signing secrets and run #92 signed release.
6. Schedule Evo X2 maintenance window for #45/#90.
7. Revisit #36 with a different local GGUF or llama.cpp server configuration.

## Issue comment template

Use this shape when posting fresh validation:

```text
Validation date: YYYY-MM-DD
Branch/commit:
Command:
Result:
Metrics:
- score=
- keyword_overlap=
- runes=
- first_chunk_ms=
- verification_passed=
Artifacts:
Close decision:
Remaining blocker:
```
