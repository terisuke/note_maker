# Docker Deployment Runbook

This runbook covers the Tier 3 LAN/VPN Docker deployment path.

## Prerequisites

- Docker Engine or Docker Desktop with Compose v2.
- A host that can reach the Evo X2 Tailnet LLM endpoint.
- Optional Tailscale sidecar mode: a reusable auth key in `TS_AUTHKEY`.
- A private LAN/VPN reverse proxy if `MULTI_USER=1` is enabled.

## Standalone Start

```sh
docker compose up -d --build app
docker compose ps
curl -fsS http://127.0.0.1:8080/healthz
curl -fsS http://127.0.0.1:8080/api/models
```

The app stores durable data in the `note-maker-data` volume at
`/var/lib/note-maker`. The default Docker store is SQLite at
`/var/lib/note-maker/workflow_store.db`.

## Multi-User Mode

Set `MULTI_USER=1` only behind a trusted reverse proxy that authenticates users
and sets `X-Forwarded-User` on every request.

```sh
MULTI_USER=1 WORKFLOW_STORE_DRIVER=sqlite docker compose up -d --build app
```

Requests without `X-Forwarded-User` return `401`. Single-user mode uses the
`local` user and preserves existing Tier 1/Tier 2 data.
The public `/healthz` endpoint remains available for Docker healthchecks and
does not expose user data.

See `docs/operations/multi-user-quickstart.md` for the trusted-header contract,
proxy examples, and a two-user smoke test.

## Tailscale Sidecar Start

Set the auth key in the shell or in an uncommitted `.env`:

```sh
export TS_AUTHKEY=tskey-auth-...
docker compose --profile tailscale up -d tailscale app-via-tailscale
curl -fsS http://127.0.0.1:${NOTE_MAKER_TAILSCALE_HOST_PORT:-8081}/api/models
```

The app shares the sidecar network namespace with
`network_mode: service:tailscale`. Compose waits for the sidecar healthcheck
before starting the app. The sidecar publishes
`${NOTE_MAKER_TAILSCALE_HOST_PORT:-8081}:8080`, so a default standalone `app`
service can still use `${NOTE_MAKER_HOST_PORT:-8080}:8080` without a host-port
collision.

## Stop

```sh
docker compose down
```

To keep user data, do not delete the `note-maker-data` volume.

## Upgrade

```sh
docker compose pull
docker compose up -d --build
curl -fsS http://127.0.0.1:8080/api/models
```

For GHCR releases, set the image tag in `compose.yaml` or override it from an
environment-specific Compose file.

## Backup

```sh
docker run --rm \
  -v note-maker_note-maker-data:/data:ro \
  -v "$PWD/backups:/backup" \
  alpine:3.20 \
  tar -czf /backup/note-maker-data-$(date +%Y%m%d-%H%M%S).tgz -C /data .
```

## Restore

Stop the app first:

```sh
docker compose down
docker run --rm \
  -v note-maker_note-maker-data:/data \
  -v "$PWD/backups:/backup:ro" \
  alpine:3.20 \
  sh -c 'rm -rf /data/* && tar -xzf /backup/<backup-file>.tgz -C /data'
docker compose up -d
```

## Troubleshooting

- `curl /api/models` fails: confirm `LLM_BASE_URL` is reachable from inside
  the app container.
- `401 unauthorized`: `MULTI_USER=1` is enabled but the reverse proxy is not
  sending `X-Forwarded-User`.
- Tailscale profile does not start: check `docker compose logs tailscale` and
  verify `TS_AUTHKEY` is set.
- Sidecar is unhealthy: run `docker compose exec tailscale tailscale status`.
- Permission errors under `/var/lib/note-maker`: ensure the volume files are
  owned by uid/gid `65534:65534`.
