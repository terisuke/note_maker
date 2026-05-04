# Issue 92: Wails desktop scaffold validation

Date: 2026-05-04
Branch: `feat/d2-3-grid-scaffold`

## Scope

- Added `cmd/desktop` scaffold for the desktop entrypoint.
- Reused the existing `internal/handlers` HTTP handlers and the existing `static/` asset tree.
- Added `wails.json` configured for Wails v2 with `frontend:dir` set to `cmd/desktop/frontend` and empty frontend install/build hooks.
- Pinned Wails v2 in `cmd/desktop/go.mod` as `github.com/wailsapp/wails/v2 v2.12.0`.
- Added desktop Makefile targets with preflight checks instead of network-dependent dependency installation.
- Added desktop first-run bootstrap for launcher-compatible data, config, workflow store, and LLM defaults.
- Added Wails native menu actions for opening the data folder and running LLM diagnostics.
- Added embedded frontend assets under `cmd/desktop/frontend/` so packaged apps do not depend on repo-local `static/`.
- Added a `cmd/desktop/wails.json` module-local Wails config and kept root `wails.json` metadata aligned.
- Updated the desktop release workflow to emit Wails dry-run logs, tarred artifact archives, and SHA-256 checksums.

## Validation

| Command | Result |
|---|---|
| `go test ./cmd/desktop` | Pass |
| `(cd cmd/desktop && go test ./...)` | Pass |
| `(cd cmd/desktop && go test -tags wails ./...)` | Pass |
| `git diff --check -- cmd/desktop wails.json Makefile .github/workflows/desktop-release.yml docs/operations/desktop-release-runbook.md docs/validation/issue-92-wails-desktop-scaffold-2026-05-04.md docs/validation/wails-desktop-smoke-test.md` | Pass |
| `make desktop-check` | Pass |
| `(cd cmd/desktop && go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0 build -dryrun)` | Pass |
| `make desktop-build` | Pass on macOS darwin/arm64; built `build/bin/note-maker.app/Contents/MacOS/note-maker` |

## Notes

- The default desktop package test path remains dependency-safe because the Wails entrypoint is isolated behind the `wails` build tag.
- `cmd/desktop/main.go` provides a local desktop preview server for handler and asset validation without requiring Wails.
- `cmd/desktop/main_wails.go` wires Wails `AssetServer.Assets` and `AssetServer.Handler` to embedded frontend assets and the desktop API router.
- `desktop-dev` and `desktop-build` invoke the pinned Wails CLI with `go run github.com/wailsapp/wails/v2/cmd/wails@v2.12.0`, so they do not depend on a global Wails install.
