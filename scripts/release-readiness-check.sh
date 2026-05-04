#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="${RELEASE_READINESS_OUT_DIR:-tmp/release-readiness}"
RUN_MULTI_ARCH_BUILD="${RUN_MULTI_ARCH_BUILD:-1}"
RUN_TAILSCALE_STARTUP="${RUN_TAILSCALE_STARTUP:-1}"
mkdir -p "$OUT_DIR"

log() {
  printf '\n==> %s\n' "$*"
}

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 127
  fi
}

run_actionlint() {
  if command -v actionlint >/dev/null 2>&1; then
    actionlint .github/workflows/desktop-release.yml .github/workflows/docker-publish.yml
    return
  fi

  require_cmd go
  go run github.com/rhysd/actionlint/cmd/actionlint@latest \
    .github/workflows/desktop-release.yml \
    .github/workflows/docker-publish.yml
}

repo_name="terisuke/note_maker"
if command -v gh >/dev/null 2>&1; then
  repo_name="$(gh repo view --json nameWithOwner --jq .nameWithOwner 2>/dev/null || printf '%s' "$repo_name")"
fi

log "Validate GitHub Actions syntax"
run_actionlint | tee "$OUT_DIR/actionlint.log"

log "Record GitHub default-branch workflow availability"
if command -v gh >/dev/null 2>&1; then
  {
    gh repo view --json defaultBranchRef,nameWithOwner
    gh workflow list --all
  } | tee "$OUT_DIR/github-workflows.log"
else
  echo "gh is not installed; skipped remote workflow availability check" | tee "$OUT_DIR/github-workflows.log"
fi

log "Record release/signing secret names visible to this token"
if command -v gh >/dev/null 2>&1; then
  gh secret list --repo "$repo_name" | tee "$OUT_DIR/github-secrets.log"
else
  echo "gh is not installed; skipped remote secret name check" | tee "$OUT_DIR/github-secrets.log"
fi

log "Render Compose configs"
require_cmd docker
docker compose config >"$OUT_DIR/compose-default.yml"
docker compose --profile tailscale config >"$OUT_DIR/compose-tailscale.yml"

log "Validate Tailscale sidecar fails fast without TS_AUTHKEY"
if [ "$RUN_TAILSCALE_STARTUP" = "1" ]; then
  set +e
  TS_AUTHKEY= TAILSCALE_RESTART_POLICY=no docker compose --profile tailscale up --no-build --no-deps tailscale \
    >"$OUT_DIR/tailscale-no-authkey.log" 2>&1
  status=$?
  set -e
  docker compose --profile tailscale down --remove-orphans >/dev/null 2>&1 || true
  grep -q "TS_AUTHKEY is required" "$OUT_DIR/tailscale-no-authkey.log"
  grep -Eq "exited with code [1-9][0-9]*" "$OUT_DIR/tailscale-no-authkey.log"
  echo "tailscale sidecar failed fast without TS_AUTHKEY; compose status: $status"
else
  echo "RUN_TAILSCALE_STARTUP=0; skipped no-auth startup check"
fi

log "Run Dockerfile multi-arch build checks"
docker buildx build --check --platform linux/amd64,linux/arm64 . | tee "$OUT_DIR/docker-buildx-check.log"

if [ "$RUN_MULTI_ARCH_BUILD" = "1" ]; then
  image="ghcr.io/${repo_name}:release-dry-run"
  docker buildx build \
    --platform linux/amd64,linux/arm64 \
    --tag "$image" \
    --label "org.opencontainers.image.source=https://github.com/${repo_name}" \
    --metadata-file "$OUT_DIR/docker-buildx-metadata.json" \
    --output=type=cacheonly \
    --progress=plain \
    .
  grep -q '"buildx.build.provenance/linux/amd64"' "$OUT_DIR/docker-buildx-metadata.json"
  grep -q '"buildx.build.provenance/linux/arm64"' "$OUT_DIR/docker-buildx-metadata.json"
  echo "multi-arch cache-only build metadata includes linux/amd64 and linux/arm64"
else
  echo "RUN_MULTI_ARCH_BUILD=0; skipped cache-only multi-arch build"
fi

log "Release readiness checks completed"
printf 'Evidence directory: %s\n' "$OUT_DIR"
