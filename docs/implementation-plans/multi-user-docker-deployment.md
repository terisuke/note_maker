# Multi-user Docker deployment implementation plan

Date: 2026-05-03

This plan implements [ADR 0004 Tier 3](../adrs/0004-three-tier-deployment.md#tier-3--multi-user-docker). It enables a small team of 5–15 writers to share one host running the Note Maker container, with per-user data isolation and Tailscale-mediated LLM access over LAN or VPN.

## Goal

Produce a Docker image and a `compose.yaml` that a team can stand up on a single host. Each authenticated user sees only their own personas, projects, articles, and drafts. LLM traffic routes through a Tailscale sidecar container so all Evo X2 MagicDNS hostnames resolve correctly inside the container network. The deployment scope is LAN or Tailscale VPN only; public-internet access is explicitly out of scope.

## Non-goals

- SaaS or public-internet deployment.
- Anonymous (unauthenticated) access.
- Mobile-native clients. The responsive breakpoint from ADR 0003 handles mobile browsers.
- Billing or usage metering.
- Federated identity or cross-organizational OIDC. One concrete provider example ships; the interface is pluggable.
- Tier 1 or Tier 2 changes. The `MULTI_USER=1` flag is off by default; single-user deployments are unaffected.

## Cut sequence

### Cut E2-1: Dockerfile

Files to create: `Dockerfile`.

Multi-stage build:

1. Build stage: `golang:1.22-alpine` base, `go build -o /app/note-maker-server ./cmd/server`, copy `static/` into the build output.
2. Runtime stage: `gcr.io/distroless/static-debian12` or `alpine:3.20` base, non-root user (`USER 65534:65534`), copy the binary and `static/` from the build stage.

Add `HEALTHCHECK CMD wget -qO- http://localhost:8080/api/models || exit 1` (or `curl` if the base image includes it). Expose port `8080`.

This cut is standalone; it does not depend on any auth work.

Acceptance:

- `docker build -t note-maker:dev .` succeeds.
- `docker run --rm -p 8080:8080 note-maker:dev` starts and `curl http://localhost:8080/api/models` returns a 200 response.
- The container runs as a non-root user (`id` shows uid=65534 inside the container).
- `go test ./...` still passes (no import changes).

Delegation: Codex CLI-friendly. Standard multi-stage Go Dockerfile pattern.

---

### Cut E2-2: compose.yaml

Files to create: `compose.yaml`, `.env.example`.

Define two services:

- `app`: the Note Maker image, volume-mounted at `/var/lib/note-maker` for persistent data, environment variables loaded from `.env` (secrets pattern).
- `tailscale`: the official `tailscale/tailscale` image as an optional sidecar. The `app` service sets `network_mode: service:tailscale` so it shares the sidecar's network namespace. When the sidecar is present, the app container resolves `evo-x2.tailb30e58.ts.net` through the sidecar's MagicDNS.

Document the Tailscale sidecar pattern with reference to the [Tailscale Docker guide](https://tailscale.com/blog/docker-tailscale-guide).

Provide `.env.example` with all required variable names and placeholder values. Never commit a populated `.env` file.

Acceptance:

- `docker compose up -d` starts both services without errors.
- `docker compose exec app wget -qO- http://localhost:8080/api/models` returns 200.
- Removing the `tailscale` service and the `network_mode` line leaves the app running in standalone mode (for teams that do not need Tailscale).

Delegation: Cursor-friendly. Declarative YAML; `.env.example` pattern is mechanical.

---

### Cut E2-3: Tailscale sidecar wiring

Files to touch: `compose.yaml`, `.env.example`.

Add environment variable support for the Tailscale sidecar:

- `TS_AUTHKEY`: Tailscale authentication key (ephemeral, write-only; do not log).
- `TS_HOSTNAME`: the hostname to register in the Tailnet (for example, `note-maker-team`).
- `TS_EXTRA_ARGS`: optional extra flags passed to `tailscaled`.

The `app` container's `LLM_BASE_URL` defaults to `http://evo-x2.tailb30e58.ts.net/v1` and resolves through the sidecar's MagicDNS when the sidecar is running. If the sidecar restarts, the app retries LLM calls using the existing fallback chain.

Document the startup sequence: the Tailscale sidecar must be healthy before the app container starts making LLM calls. Use `depends_on: tailscale: condition: service_healthy` in `compose.yaml` and add a Tailscale healthcheck (`tailscale status --json | jq -e .BackendState == "Running"`).

Acceptance:

- `tailscale status` inside the sidecar container shows the Tailnet connection as active.
- `docker compose exec app curl http://evo-x2.tailb30e58.ts.net/v1/models` returns 200.
- Restarting the sidecar and waiting for it to recover does not require restarting the app container.

Delegation: Codex CLI-friendly. Environment variable mapping and healthcheck DSL follow documented Tailscale patterns.

---

### Cut E2-4: Auth middleware skeleton

Files to create: `internal/handlers/middleware/auth.go`, `internal/handlers/middleware/auth_test.go`.

Define an `AuthProvider` interface:

```go
type AuthProvider interface {
    Principal(r *http.Request) (string, error) // returns userID or error
}
```

Implement three providers:

- `TrustedHeaderProvider`: reads `X-Forwarded-User` from the request. This is the minimum implementation for a reverse-proxy-fronted deployment.
- `OIDCProvider`: placeholder that returns `ErrNotImplemented`; wired in Cut E2-9.
- `MagicLinkProvider`: placeholder; reserved for a future cut.

Add a `MULTI_USER` environment variable flag to `cmd/server/main.go`. When `MULTI_USER=1`, the server wraps every handler with the selected `AuthProvider` middleware. When unset or `0`, the middleware is a no-op that returns `userID = "local"`, preserving full backwards compatibility with Tier 1 and Tier 2.

Acceptance:

- Unit tests: `TrustedHeaderProvider` returns the header value when present and an error when absent. No-op provider always returns `"local"`.
- `go test ./internal/handlers/middleware/...` passes.
- Starting the server without `MULTI_USER=1` behaves identically to today.

Delegation: Codex CLI-friendly. Interface and two concrete implementations are fully specified.

---

### Cut E2-5: Data-model migration — `user_id` column

Files to create: `internal/infrastructure/repository/sqlite/migrations/0004_user_id.sql`.

Add `user_id TEXT NOT NULL DEFAULT 'local'` to the following tables: `personas` (custom rows only; built-in seeds are user-agnostic), `projects`, `articles`, `brief_sessions`, `brief_answers`, `drafts`, `writing_style_guides`, `author_sources`. The migration runs `ALTER TABLE … ADD COLUMN user_id TEXT NOT NULL DEFAULT 'local'` for each table, then `UPDATE … SET user_id = 'local' WHERE user_id = ''` as a backfill guard.

Built-in personas (`terisuke`, `cloudia`) in the seed registry are not stored in the `personas` table; they are compiled in. The migration does not touch them.

Acceptance:

- Running the migration on an existing `workflow_store.db` adds the column without data loss.
- All existing rows have `user_id = 'local'` after the migration.
- `go test ./internal/infrastructure/repository/sqlite/...` passes including a restart test that confirms the column persists.

Delegation: Codex CLI-friendly. SQL `ALTER TABLE` migration following the established migration pattern.

---

### Cut E2-6: Repository per-user scoping

Files to touch: `internal/infrastructure/repository/sqlite/` (all repository methods that return user-scoped data), `internal/handlers/middleware/auth.go`.

Every SQLite query that reads user-owned data gains a `userID string` parameter in its repository method signature. A `RepoContext` struct carries the authenticated principal and is injected by the auth middleware (E2-4) into the request context. Repository methods extract `userID` from `RepoContext` rather than accepting it as a direct argument, keeping the call sites clean.

Add an index on `user_id` for the tables modified in E2-5.

Add a unit test that seeds two users (`alice`, `bob`) with separate records and asserts that querying with `alice`'s context returns zero rows for `bob`'s data.

This cut must be developed in tandem with E2-4. Do not merge E2-6 without E2-4 also being complete; the two cuts constitute one functional unit.

Acceptance:

- `go test ./internal/infrastructure/repository/sqlite/...` passes including the cross-user isolation test.
- No repository method that returns user-scoped rows compiles without a `userID` parameter (enforced by the interface).

Delegation: Codex CLI-friendly for the mechanical signature changes; needs human review after E2-5 to confirm no table is missed.

---

### Cut E2-7: Handler audit

Files to touch: every file in `internal/handlers/`.

Review every handler in `internal/handlers/*.go`. Any handler that returns data scoped to a user (personas, projects, articles, sessions, answers, briefs, drafts, style guides, source snapshots) must receive `RepoContext` from the request context and pass `userID` to the repository call. Handlers that return global data (built-in personas, formats, model status) are unchanged.

Update the Playwright E2E tests in `tests/e2e/` to seed a default `local` user in the test database fixture so the existing 13 tests continue to pass against the per-user-scoped handlers.

Acceptance:

- `go test ./internal/handlers/...` passes with 80%+ coverage (matching the gate from Issue [#29](https://github.com/terisuke/note_maker/issues/29)).
- `python3 -m pytest tests/e2e -q` passes (13 tests green).
- A grep for `repo.List`, `repo.Get`, `repo.Create` in handler files shows no call site that lacks a `userID` source.

Delegation: needs human review checkpoint. This is the highest-risk cut; a missed handler is a data-isolation bug. Codex CLI can make the mechanical changes; a human must review the diff before merge.

---

### Cut E2-8: Trusted-header reverse proxy demo

Files to create: `compose.override.proxy.yaml`, `docs/operations/multi-user-quickstart.md`.

Provide an example `compose.override.proxy.yaml` that adds a Caddy or nginx service configured to:

1. Require HTTP Basic Auth or delegate to OAuth2 Proxy.
2. Set `X-Forwarded-User` to the authenticated username.
3. Forward requests to the `app` service.

Write a 30-minute quickstart guide in `docs/operations/multi-user-quickstart.md` covering: prerequisites, `docker compose -f compose.yaml -f compose.override.proxy.yaml up -d`, adding a second user, and verifying isolation.

Acceptance:

- Following the quickstart guide from a clean Ubuntu 22.04 host produces a running two-user Note Maker instance within 30 minutes.
- User A's drafts are not visible to User B.
- The guide includes a troubleshooting section for the three most common failure modes (Tailscale sidecar not connecting, proxy misconfiguration, volume permission error).

Delegation: Cursor-friendly for the YAML overlay; Codex CLI-friendly for the operations guide prose.

---

### Cut E2-9: OIDC provider implementation

Files to touch: `internal/handlers/middleware/auth.go`, `go.mod`, `go.sum`.

Wire one concrete OIDC provider (suggest Authentik or Auth0 as the documented example, using the `coreos/go-oidc/v3` library). Validate the JWT against the provider's JWKS endpoint. Extract the subject claim as `userID`. Store the session as a short-lived cookie.

This cut is optional and gated on operator readiness. The `TrustedHeaderProvider` from E2-4 remains the recommended path for teams without an existing OIDC provider.

Acceptance:

- Unit test: a valid JWT from a mock OIDC provider is accepted; an expired or tampered JWT is rejected.
- Integration test: `docker compose up` with Authentik configured returns a valid session cookie after login.

Delegation: Codex CLI-friendly for the library wiring; needs human judgment for provider-specific configuration.

---

### Cut E2-10: Audit log

Files to create: `internal/handlers/middleware/audit.go`, `internal/handlers/middleware/audit_test.go`.

Record every cross-user access attempt (a request where the authenticated `userID` does not match the resource owner) to a structured log. Default output: `/var/log/note-maker/audit.log` in the container (rotated by the Docker logging driver or a log-rotation sidecar). Each log line is a JSON object with: `timestamp`, `request_id`, `user_id`, `resource_type`, `resource_owner_id`, `action`, `result` (`allowed` or `denied`).

Normal (allowed) requests are logged at `INFO` level with result `allowed`. Cross-user attempts are logged at `WARN` with result `denied` and the resource owner's `user_id`.

Acceptance:

- Unit test: the middleware logs a `denied` entry when the authenticated user differs from the resource owner.
- `docker compose exec app cat /var/log/note-maker/audit.log` shows structured JSON after a test request.

Delegation: Codex CLI-friendly. Structured logging pattern is well-established in the codebase.

---

### Cut E2-11: GHCR publish workflow

Files to create: `.github/workflows/container-publish.yml`.

GitHub Actions workflow that triggers on version tags (`v*.*.*`) and on pushes to `main`. Builds and pushes `ghcr.io/<owner>/note-maker:<tag>` using `docker buildx build --platform linux/amd64,linux/arm64` (multi-arch). Uses `GITHUB_TOKEN` for GHCR authentication; no additional secrets required.

Acceptance:

- Pushing a tag `v0.4.0` triggers the workflow and produces `ghcr.io/terisuke/note-maker:v0.4.0` and `ghcr.io/terisuke/note-maker:latest` in GHCR.
- The published image runs on both `amd64` and `arm64` hosts.
- `go test ./...` is run as a pre-publish gate in the workflow.

Delegation: Codex CLI-friendly. Standard GHCR publish workflow.

---

### Cut E2-12: Operations runbook

Files to create: `docs/operations/docker-runbook.md`.

Document the full operations lifecycle:

- **Start**: `docker compose up -d`; confirm health at `/api/models`.
- **Stop**: `docker compose down`.
- **Upgrade**: pull the new image, `docker compose pull`, `docker compose up -d`.
- **Backup**: `docker compose exec app tar czf - /var/lib/note-maker > backup-$(date +%Y%m%d).tar.gz`.
- **Restore**: stop the container, restore the volume, restart.
- **Tailscale sidecar troubleshooting**: `docker compose exec tailscale tailscale status`; re-authenticate if the key has expired (`docker compose exec tailscale tailscale login`).
- **LLM connectivity loss during a request**: the app returns the partial draft with an error annotation; the user can retry. The sidecar restart does not lose in-flight HTTP connections from the app because the app uses the fallback chain.

Acceptance:

- A team member unfamiliar with Docker can complete the upgrade procedure in under 10 minutes by following the runbook.
- The backup and restore procedure is tested by restoring a backup to a fresh volume and confirming that `GET /api/workflow/artifacts` returns the expected data.

Delegation: Cursor-friendly. Prose documentation with verified command examples.

---

## Risk register

| Risk | Mitigation |
|---|---|
| Cross-user data leak via a missed handler | E2-7 handler audit with a dedicated cross-user isolation unit test and a human review checkpoint. |
| Tailscale sidecar restarts mid-request | The app's existing fallback chain (`EVO_X2_LLAMA_CPP_LLM_BASE_URL`, then `127.0.0.1:8081`) catches the LLM connectivity loss. Partial drafts are preserved. |
| Docker volume permission drift (files owned by root inside the container) | Run the app as `uid=65534` in E2-1. Mount volumes with `user: "65534:65534"` in `compose.yaml`. Document the `chown` fix in the runbook. |
| Sidecar restarts causing LLM connectivity loss mid-request | `depends_on: tailscale: condition: service_healthy` delays app startup. After startup, the fallback chain handles transient sidecar restarts without losing the draft. |
| Header-trust spoofing if the reverse proxy is misconfigured | Document that `X-Forwarded-User` must only be accepted from the proxy's IP. The Caddy/nginx example in E2-8 includes a `trusted_proxies` directive. Provide a hardening checklist in `docs/operations/multi-user-quickstart.md`. |

## Validation baseline

Run after each cut before merging:

```
go test ./...
python3 -m pytest tests/e2e -q
./scripts/check-launcher.sh
git diff --check
```

Tier 3 additional checks:

```
docker build -t note-maker:dev .
docker compose up -d && sleep 5 && curl -sf http://localhost:8080/api/models
docker compose down
```

Two-user smoke test (to be added to `tests/e2e/` in Cut E2-7):

```
python3 -m pytest tests/e2e/test_multi_user_isolation.py -q
```

This Playwright test case seeds user `alice` and user `bob` with separate drafts. It asserts that:

- Logged in as `alice`, `GET /api/drafts` returns only Alice's drafts.
- Logged in as `bob`, `GET /api/drafts` returns only Bob's drafts.
- Alice's `draft_id` is not accessible to Bob (returns 404 or 403).

## Delegation matrix

| Cut | Best owner | Reason |
|---|---|---|
| E2-1: Dockerfile | Codex CLI | Standard multi-stage Go Dockerfile |
| E2-2: compose.yaml | Cursor | Declarative YAML; `.env.example` pattern |
| E2-3: Tailscale wiring | Codex CLI | Environment mapping follows documented Tailscale patterns |
| E2-4: Auth middleware skeleton | Codex CLI | Interface and two implementations fully specified |
| E2-5: user_id migration | Codex CLI | SQL ALTER TABLE following established migration pattern |
| E2-6: Repository scoping | Codex CLI + human review | Mechanical signature changes; human confirms no table is missed |
| E2-7: Handler audit | Human review checkpoint | Missed handler = data-isolation bug; Codex CLI makes changes, human reviews diff |
| E2-8: Reverse proxy demo | Cursor (YAML) + Codex CLI (guide) | Declarative config + prose |
| E2-9: OIDC provider | Codex CLI + human judgment | Library wiring is mechanical; provider config requires operator input |
| E2-10: Audit log | Codex CLI | Structured logging pattern |
| E2-11: GHCR publish | Codex CLI | Standard GHCR workflow |
| E2-12: Operations runbook | Cursor | Prose documentation with verified commands |
