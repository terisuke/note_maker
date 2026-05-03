# ADR 0004: Three-Tier Deployment Model

Date: 2026-05-03

## Status

Accepted. Extends the near-term execution order recorded in [ADR 0002 §implementation-phases](0002-multi-persona-multi-format-extension.md#phased-rollout). This ADR does not commit to delivery dates; it fixes the architecture so that packaging, desktop distribution, and multi-user hosting can each converge without undoing each other.

## Context

The 2026-05-03 audit found that "distributable desktop app" (Issue [#15](https://github.com/terisuke/note_maker/issues/15)) and "multi-PC LAN web service" were being treated as one problem. They are not.

The existing `scripts/launcher.sh` (lines 1-502) is a developer-ergonomic local launcher:

- It binds to `127.0.0.1` on a dynamically chosen port (line 97 port-check, line 480 app URL).
- It builds `cmd/server` via `go build` at each start (`scripts/launcher.sh:263-275`).
- It opens the system browser on the loopback URL (`scripts/launcher.sh:197-205`).
- It assumes single-user, no authentication, and Tailnet LLM access from the developer's own machine.
- It stores data under the OS user data directory (macOS: `~/Library/Application Support/Note Maker` at `scripts/launcher.sh:73`, Linux: `$XDG_DATA_HOME/note-maker` at `scripts/launcher.sh:76-77`).

This is complete and correct for its purpose. It shipped in PR #87, which closed Issue [#15](https://github.com/terisuke/note_maker/issues/15).

What the launcher is not:

- It is not a signed, redistributable native application that a non-developer can double-click.
- It is not a container that a small team can share over a LAN.

Conflating these two missing capabilities with the launcher causes scope creep in every packaging ticket. This ADR names three distinct tiers, assigns separate entry points, and records what is shared and what is not.

## Decision

Adopt a three-tier deployment model with separate entry points but a shared Go handler layer, JSON contract, and repository interfaces.

### Tier matrix

| Attribute | Tier 1 — Launcher | Tier 2 — Wails Desktop | Tier 3 — Multi-user Docker |
|---|---|---|---|
| Audience | Developer / project owner | Non-developer single user | Small team (5–15 writers) |
| Process | bash + `go build` + browser | Single native binary + OS WebView | Container + Tailscale sidecar |
| Network bind | `127.0.0.1` (loopback) | `127.0.0.1` (in-process) | `0.0.0.0:8080` |
| Auth | None (loopback trust) | None (loopback trust) | Required; `user_id`-scoped data |
| Data scope | Single user | Single user | Per-user isolation |
| LLM transport | Tailnet MagicDNS from host | Tailnet MagicDNS from host | Tailnet MagicDNS through sidecar |
| Distribution | `git pull` + `make launcher` | Signed binary download | Docker image from GHCR |
| New work owner | No further work in this ADR | [Tier 2 plan](../implementation-plans/wails-desktop-packaging.md) | [Tier 3 plan](../implementation-plans/multi-user-docker-deployment.md) |

### Tier 1 — Current launcher

Entry point: `scripts/launcher.sh` + `cmd/server/main.go`.

Already shipped as of PR #87. Described fully in [Note Maker app handoff](../handoffs/app-handoff-2026-05-03.md). No further architectural work in this ADR.

### Tier 2 — Wails desktop

Entry point: `cmd/desktop/main.go` (to be created).

A single redistributable binary embedding the existing Go handler code and serving the existing `static/` assets through an OS-native WebView. The binary is 10–20 MB depending on platform, carries no Node.js sidecar, and requires no browser to be open. The installer is signed on macOS (Developer ID) and Windows (code-signing certificate); Linux ships as an AppImage with a checksum.

Framework: [Wails v2](https://wails.io/) stable release. As of 2026-05, Wails v3 is in alpha at [v3alpha.wails.io](https://v3alpha.wails.io/). Wails v3 will be revisited when it reaches a stable release; the architecture of this tier does not depend on v2-specific APIs that would make migration difficult.

Wails was chosen over Electron because the backend is already Go. Wails provides direct Go method bindings to the WebView JavaScript context, eliminating the Node.js sidecar that Electron requires and avoiding the Chromium bundle that inflates Electron app sizes.

The existing static assets, including the Alpine.js-based three-pane workspace introduced by [ADR 0003](0003-conversation-first-workspace-ui.md), are served through Wails without a separate build step. `static/vendor/alpine.min.js` is vendored, so the Wails binary has no runtime internet dependency.

Tier 1 and Tier 2 share the same data directory schema and the same default data directory paths. Upgrading from Tier 1 to Tier 2 requires no data migration.

### Tier 3 — Multi-user Docker

Entry point: `Dockerfile` + `compose.yaml` (to be created).

A container image built from the existing Go server, exposed on `0.0.0.0:8080` for LAN or VPN access. LLM connectivity uses a Tailscale sidecar container that shares the application container's network namespace via `network_mode: service:tailscale`, as described in the [Tailscale Docker guide](https://tailscale.com/blog/docker-tailscale-guide). The application container resolves Evo X2 MagicDNS hostnames (`evo-x2.tailb30e58.ts.net`) through the sidecar's Tailscale tunnel.

Authentication is required. The minimum implementation uses a `TrustedHeaderProvider` that reads `X-Forwarded-User` set by an upstream reverse proxy (Caddy or nginx with OAuth2 Proxy). An OIDC provider implementation (Authentik or Auth0) is a later optional cut.

The largest single ticket for Tier 3 is a data-model migration that adds `user_id TEXT NOT NULL DEFAULT 'local'` to the following SQLite tables: `personas` (custom), `projects`, `articles`, `brief_sessions`, `brief_answers`, `drafts`, `writing_style_guides`, and `author_sources`. Built-in personas (`terisuke`, `cloudia`) stay user-agnostic as seed data. All existing rows are backfilled to `user_id = 'local'`. This migration is filed as `0004_user_id.sql` under the existing migrations directory.

Enabling multi-user mode requires `MULTI_USER=1` at startup; the default mode (single user) remains backwards-compatible with Tier 1 and Tier 2.

## Out of scope

- Native mobile (Capacitor, React Native). Mobile access is handled by the responsive breakpoint introduced in ADR 0003 only.
- Public-internet SaaS. Tier 3 is LAN/VPN scope only.
- Migration tooling between tiers. Tier 1 to Tier 2 requires no migration. Moving data from Tier 1/2 to Tier 3 is an operations procedure, not a product feature.
- OIDC provider choice for Tier 3. The auth middleware interface is specified; the concrete provider is left to the operator.
- Auto-update for any tier.
- Wails v3 transition. Deferred until v3 reaches a stable release.

## Consequences

Positive:

- Three distinct audiences (developer, solo desktop user, small team) each have a first-class deployment path.
- The UI rewrite described in [ADR 0003](0003-conversation-first-workspace-ui.md) is independent of all three tiers; it applies to Tier 1 immediately and the static assets are reused unchanged by Tier 2 and Tier 3.
- Adding a fourth tier (public SaaS) in a future ADR would not require redesigning the existing three.

Negative:

- Three entry points exist in the repository; keeping them consistent requires discipline. Mitigation: the shared handler layer (`internal/handlers/`) and repository interfaces (`internal/infrastructure/repository/`) are the authoritative logic; entry points are thin wrappers.
- Tier 3 requires a real data-model migration with a `user_id` column. Any missed handler that reads user-scoped data without the `userID` filter is a data-isolation bug. Mitigation: the handler audit cut (E2-7 in the Tier 3 plan) reviews every handler, and a unit test asserts that cross-user reads return zero rows.
- Wails v3 transition is deferred. If v3 reaches stable before Tier 2 ships, the team will evaluate migration at that point.

Neutral:

- Execution order between Tier 2 and Tier 3 is left to the respective implementation plans. Neither blocks the other. The UI rewrite from ADR 0003 can ship before, during, or after either packaging tier.

## References

- [ADR 0002 §implementation-phases](0002-multi-persona-multi-format-extension.md#phased-rollout)
- [ADR 0003 — Conversation-first workspace UI with Alpine.js](0003-conversation-first-workspace-ui.md)
- [Note Maker app handoff 2026-05-03](../handoffs/app-handoff-2026-05-03.md) — Tier 1 baseline
- [Wails v2 documentation](https://wails.io/)
- [Wails v3 alpha documentation](https://v3alpha.wails.io/)
- [Tailscale Docker guide](https://tailscale.com/blog/docker-tailscale-guide)
- Implementation plan — Tier 2: [Wails desktop packaging](../implementation-plans/wails-desktop-packaging.md)
- Implementation plan — Tier 3: [Multi-user Docker deployment](../implementation-plans/multi-user-docker-deployment.md)
- Implementation plan — UI rewrite: [Conversation-first workspace UI](../implementation-plans/conversation-workspace-ui.md)
