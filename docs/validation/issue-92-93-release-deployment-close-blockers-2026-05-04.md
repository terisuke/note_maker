# Issues #92/#93 release and deployment close-blocker validation

Date: 2026-05-04
Branch: `codex/close-runtime-desktop-deployment`

## Scope

This validation covers the remaining release/deployment blockers that can be
closed without external signing credentials, GHCR publication, a real Tailscale
auth key, or platform hardware smoke tests.

## Changes

- Added `scripts/release-readiness-check.sh` to collect local release evidence:
  GitHub Actions lint, default-branch workflow visibility, repository secret
  names, Compose rendering, Tailscale no-auth fail-fast behavior, Dockerfile
  multi-arch checks, and cache-only buildx metadata.
- Added Docker publish workflow `dry_run` dispatch mode so the default branch
  can build `linux/amd64,linux/arm64` without pushing to GHCR.
- Added Docker publish metadata verification before build/publish.
- Hardened desktop release checksum generation for macOS, Linux, and Windows
  shells.
- Changed the Tailscale sidecar to exit before `tailscale up` when `TS_AUTHKEY`
  is empty.
- Documented the default-branch promotion condition for manual workflow
  dispatch.

## Validation

```sh
docker version --format '{{.Client.Version}} {{.Server.Version}}'
```

Result: passed. Docker client/server were both `29.4.1`.

```sh
docker buildx version
```

Result: passed. Buildx was
`github.com/docker/buildx v0.33.0-desktop.1 7f91f038ac14cbf5c4b2a6b76470860814424da1`.

```sh
gh repo view --json defaultBranchRef,nameWithOwner
gh workflow list --all
```

Result: passed. The repository default branch is `main`. The only workflows
visible on the default branch are `CI` and `Dependency Graph`, so
`desktop-release.yml` and `docker-publish.yml` manual dispatch on `develop` are
expected to return `404` until `develop` is promoted or the workflow files land
on `main`.

```sh
gh secret list --repo terisuke/note_maker
```

Result: passed. Visible repository secret names were `FIREBASE_TOKEN`,
`GCP_SA_KEY`, and `GEMINI_API_KEY`. The macOS signing/notarization and Windows
signing secret names required by `desktop-release.yml` were not present.

```sh
go run github.com/rhysd/actionlint/cmd/actionlint@latest \
  .github/workflows/desktop-release.yml \
  .github/workflows/docker-publish.yml
```

Result: passed.

```sh
docker compose --profile tailscale config
```

Result: passed. The rendered config includes `tailscale`,
`app-via-tailscale`, `network_mode: service:tailscale`, the sidecar
healthcheck, and `depends_on: condition: service_healthy`.

```sh
TS_AUTHKEY= TAILSCALE_RESTART_POLICY=no docker compose --profile tailscale up --no-build --no-deps tailscale
```

Result: passed. The sidecar exited non-zero before Tailscale auth and logged
`TS_AUTHKEY is required for the tailscale profile.`

```sh
docker buildx build --check --platform linux/amd64,linux/arm64 .
```

Result: passed with `Check complete, no warnings found.`

```sh
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag ghcr.io/terisuke/note_maker:release-dry-run \
  --label org.opencontainers.image.source=https://github.com/terisuke/note_maker \
  --metadata-file tmp/release-readiness/docker-buildx-metadata.json \
  --output=type=cacheonly \
  --progress=plain \
  .
```

Result: passed. The cache-only build completed without pushing and metadata
contained both `buildx.build.provenance/linux/amd64` and
`buildx.build.provenance/linux/arm64`.

## Close assessment

- #92 source-tree closeable: yes, for workflow syntax, unsigned artifact
  definition, dry-run evidence, and signing/notarization secret detection.
  Remaining work is real release execution only.
- #93 source-tree closeable: yes, for Docker/Compose definitions, multi-user
  behavior, Tailscale no-auth fail-fast, and multi-arch buildx dry-run
  evidence. Remaining work is real external publication/startup only.

## Remaining field validation only

- Promote the workflow files to `main`, then run `Desktop Release` with
  `dry_run=1` and archive macOS/Windows/Linux artifacts and checksums.
- Configure the required Apple and Windows signing secrets, then run a signed
  desktop release and notarization/signing verification.
- Smoke the produced desktop artifacts on macOS, Windows, and Linux hosts.
- Provide a real `TS_AUTHKEY`, start `docker compose --profile tailscale up -d
  tailscale app-via-tailscale`, and verify `/api/models` through the sidecar.
- Run `Docker Publish` from `main` with `dry_run=1`, then run the non-dry
  release publish to GHCR and inspect the pushed `linux/amd64,linux/arm64`
  manifest on target hosts.
