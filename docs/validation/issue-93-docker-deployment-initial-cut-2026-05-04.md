# Issue #93 Docker Deployment Initial Cut Validation

Date: 2026-05-04
Branch: `feat/d2-3-grid-scaffold`

## Scope

Implemented the Docker and multi-user readiness slice:

- Multi-stage Go Docker build for `cmd/server`.
- Alpine runtime running as the existing non-root uid/gid `65534:65534`.
- Runtime healthcheck against public `GET /healthz`, so container readiness
  still works when `MULTI_USER=1` protects API routes.
- Persistent `/var/lib/note-maker` data volume wiring.
- Default standalone Compose service.
- Optional Tailscale sidecar Compose pattern with an `app-via-tailscale` service sharing `network_mode: service:tailscale`.
- Tailscale sidecar host publishing now uses `${NOTE_MAKER_TAILSCALE_HOST_PORT:-8081}:8080`, so the default `app` service can remain on `${NOTE_MAKER_HOST_PORT:-8080}:8080` without a profile port conflict.
- Tailscale sidecar healthcheck using `tailscale status --peers=false`, with the app gated by `depends_on: condition: service_healthy`.
- Docker-focused Makefile targets.
- `MULTI_USER=1` trusted-header auth using `X-Forwarded-User`.
- SQLite `0004_user_id.sql` migration with `user_id` backfill and indexes.
- Request-scoped SQLite store plumbing through workflow/history handlers.
- Two-user HTTP e2e isolation coverage for `GET /api/workflow/artifacts`.
- Cross-user natural ID conflict detection: if another user already owns a
  natural ID such as a custom persona or project ID, the second scoped write is
  rejected instead of returning false success.

## Validation

```sh
docker --version
```

Result: passed. Docker CLI is installed: `Docker version 29.4.0, build 9d7ad9f`.

```sh
docker compose config
```

Result: passed. The default standalone Compose service renders successfully.
The rendered default config publishes only the app service on host port `8080`.

```sh
docker compose --profile tailscale config
```

Result: passed. The optional Tailscale profile renders successfully with `tailscale` and `app-via-tailscale`.
The rendered profile includes the sidecar healthcheck, `condition: service_healthy`, the default app published on `8080`, and the sidecar path published on `8081`.

```sh
docker build -t note-maker:dev .
```

Result: passed after Docker Desktop was started.

Image inspection:

```text
8378935 arm64 linux
```

The image is approximately 8.4 MB, below the 100 MB target.

```sh
docker run --rm -d --name note-maker-smoke -p 18081:8080 \
  -e LLM_RUNTIME=remote \
  -e LLM_BASE_URL=http://evo-x2.tailb30e58.ts.net/v1 \
  -e LLM_MODEL=gemma4:31b \
  note-maker:dev
curl -fsS http://127.0.0.1:18081/api/models
```

Result: passed. The container returned `200` from `/api/models` and listed the
Evo X2 model catalog, including `gemma4:31b` and `qwen3.6:27b`.

A static/API smoke also passed:

- `/healthz` returned `204`.
- `/` served the workspace page with `class="workspace"`.
- `/static/js/script.js` included the D2-4 history rail code.
- `/api/personas` returned JSON.
- The container process ran as `65534:65534`.

`MULTI_USER=1` Docker smoke also passed:

- `/healthz` returned `204` without `X-Forwarded-User`.
- `/api/models` returned `401` without `X-Forwarded-User`.
- `/api/models` returned `200` with `X-Forwarded-User: alice`.

```sh
python3 -m pytest tests/e2e/test_multi_user_isolation.py -q
```

Result: passed. The test starts the real server with `MULTI_USER=1` and SQLite,
seeds Alice and Bob draft rows directly in the migrated database, then confirms
`X-Forwarded-User: alice` only sees `alice-draft`, `X-Forwarded-User: bob` only
sees `bob-draft`, a same-ID persona create returns `409` for the second user,
`/healthz` remains public, and a request without the trusted header returns
`401` for protected API routes.

```sh
go test ./...
```

Result: passed.

```sh
python3 -m pytest tests/e2e -q
```

Result: passed. `14 passed in 7.96s`.

```sh
./scripts/check-launcher.sh
```

Result: passed.

```sh
git diff --check
```

Result: passed.

## Notes

For standalone local validation after Docker Desktop or another daemon is running:

```sh
make docker-build
docker run --rm -p 8080:8080 note-maker:dev
```

For the optional Tailscale sidecar path, set `TS_AUTHKEY` in the shell or an uncommitted `.env`, then start only the sidecar services:

```sh
docker compose --profile tailscale up -d tailscale app-via-tailscale
curl -fsS http://127.0.0.1:${NOTE_MAKER_TAILSCALE_HOST_PORT:-8081}/api/models
```
