# Wails desktop packaging implementation plan

Date: 2026-05-03

This plan implements [ADR 0004 Tier 2](../adrs/0004-three-tier-deployment.md#tier-2--wails-desktop). It produces a signed, redistributable single-binary desktop application for macOS, Windows, and Linux, reusing the existing Go handlers and static assets without modification.

## Goal

Ship a Wails v2 desktop build of Note Maker that a non-developer can install and run without `git`, `go`, or a running browser. The binary embeds the same `internal/handlers/` code and serves the same `static/` assets that Tier 1 (the launcher) uses. Tier 1 data directories are shared, so moving from the launcher to the desktop app requires no data migration.

## Non-goals

- Mobile (iOS, Android). Mobile access is the responsive breakpoint only.
- Auto-update on first ship. The binary can be replaced manually; an auto-update mechanism is a separate future cut.
- Custom branding and icons beyond a placeholder icon. A production-quality icon set is a separate design task.
- Multi-user mode. That is Tier 3 ([ADR 0004 §Tier 3](../adrs/0004-three-tier-deployment.md#tier-3--multi-user-docker)).
- Wails v3 migration. Deferred until Wails v3 reaches a stable release.

## Wails version pin

Wails v2 latest stable as of 2026-05. This plan will be revisited and updated when Wails v3 reaches a stable release. The v2 API surface used here (`wails.Run`, `runtime.BrowserOpenURL`, menu APIs) has documented equivalents in the v3 alpha; a future migration should not require rewriting the handler layer.

## Cut sequence

### Cut E1-1: Wails v2 scaffolding

Files to create: `cmd/desktop/main.go`, `wails.json`.

Add Wails as a Go module dependency (`go get github.com/wailsapp/wails/v2`). Create `cmd/desktop/main.go` that calls the existing handler setup (the same function that `cmd/server/main.go` calls to register routes) as the Wails app's startup function. Create `wails.json` pointing at `cmd/desktop` as the application entry. Run `wails dev` to confirm the existing UI loads inside the OS WebView (WebKit on macOS and Linux, Microsoft WebView2 (the Chromium-based embedded browser component for Windows, distributed separately from Wails) on Windows).

Files to touch: `go.mod`, `go.sum`, `cmd/desktop/main.go` (new), `wails.json` (new).

Acceptance:

- `wails dev` starts the app and opens the existing UI in the OS WebView without errors.
- `go test ./...` still passes (no import cycle introduced).

E2E expectation: manual verification that the three-pane workspace from ADR 0003 renders correctly inside the WebView.

Delegation: Codex CLI-friendly. Boilerplate Wails entry point with an existing handler function wired in.

---

### Cut E1-2: Asset pipeline reuse

Files to touch: `wails.json`, `cmd/desktop/main.go`.

Point Wails at the existing `static/` directory rather than generating a new frontend scaffold. In `wails.json`, set the `frontend.dir` to `static` and disable any build-step hooks (no `npm install`, no `npm run build`). The post-ADR-0003 `static/` already contains `vendor/alpine.min.js`, `vendor/marked.min.js`, `js/store.js`, and component files, so Wails treats them as static assets served over the embedded HTTP layer.

Confirm that the `AssetFS` embed in `cmd/desktop/main.go` uses `//go:embed static` or the Wails `assetserver` configuration pointing at the on-disk `static/` directory for `wails dev`, and embedded assets for `wails build`.

Acceptance:

- `wails dev` serves `static/vendor/alpine.min.js` at the expected URL.
- `wails build` produces a binary that loads the UI without an internet connection.
- No separate `npm` or build tool is required.

Delegation: Cursor-friendly. Configuration change in `wails.json`.

---

### Cut E1-3: Native windowing and menus

Files to touch: `cmd/desktop/main.go`, `cmd/desktop/menu.go` (new).

Add a minimal native menu bar with three menus:

- **File**: "Open data folder" (opens the Tier 1 data directory in the OS file manager), "Quit".
- **View**: "Show LLM diagnostics" (mirrors the output of `make launcher-status` in a dialog).
- **Help**: "About Note Maker" (version string), "Open documentation".

Use the Wails v2 menu API (`runtime.MenuSetApplicationMenu`).

Acceptance:

- "Open data folder" opens `~/Library/Application Support/Note Maker` on macOS and the corresponding XDG path on Linux.
- "Show LLM diagnostics" displays the primary and fallback LLM endpoint health.
- The app quits cleanly from the File menu without leaving orphan processes.

Delegation: Codex CLI-friendly. Menu API is well-documented and calls existing health-check logic.

---

### Cut E1-4: First-run data directory bootstrap

Files to touch: `cmd/desktop/main.go`.

On first launch, if the data directory does not exist, create it and write the same default `app_config.json` that `scripts/launcher.sh` would write on first run. Use the same OS-conditional path logic (`~/Library/Application Support/Note Maker` on macOS, `$XDG_DATA_HOME/note-maker` or `~/.local/share/note-maker` on Linux, `%APPDATA%\Note Maker` on Windows). This ensures that a user who previously ran the Tier 1 launcher finds their existing data when they first open the Tier 2 desktop binary.

Acceptance:

- On a fresh macOS machine, opening the desktop app creates `~/Library/Application Support/Note Maker/app_config.json` with sane defaults.
- On a machine that already ran `make launcher`, opening the desktop app reads the existing `app_config.json` without overwriting it.

Delegation: Cursor-friendly. Path logic mirrors `scripts/launcher.sh` lines 59-82.

---

### Cut E1-5: macOS code signing and notarization

Files to touch: `.github/workflows/desktop-release.yml` (new or extended), `Makefile`.

Gate: this cut requires the project owner to provide an Apple Developer ID certificate and an App Store Connect API key for notarization. Without those credentials, unsigned developer builds still work; macOS will show a Gatekeeper warning.

Store signing credentials as GitHub Actions secrets: `APPLE_CERTIFICATE`, `APPLE_CERTIFICATE_PASSWORD`, `APPLE_TEAM_ID`, `APP_STORE_CONNECT_KEY_ID`, `APP_STORE_CONNECT_ISSUER_ID`, `APP_STORE_CONNECT_KEY`.

Use `wails build -platform darwin/amd64` and `wails build -platform darwin/arm64` for Apple Silicon and Intel targets. Run `xcrun notarytool submit` after signing. Produce a universal binary or two separate signed DMGs.

Acceptance:

- The signed macOS DMG opens without a Gatekeeper warning on a clean macOS machine.
- `xcrun stapler validate` passes on the notarized binary.
- Unsigned local builds (`RELEASE=0`) still produce a runnable binary; only signing and notarization are gated on credentials.

Delegation: needs human judgment. Signing credential setup and Apple notarization toolchain require the project owner's involvement.

---

### Cut E1-6: Windows code signing and MSI

Files to touch: `.github/workflows/desktop-release.yml`, `Makefile`.

Gate: this cut requires a Windows code-signing certificate (Extended Validation or Organization Validation).

Use `wails build -platform windows/amd64`. Sign the resulting `.exe` with `signtool.exe` or an equivalent CI action. Package into an MSI with a Wails-compatible installer tool (the Wails docs recommend NSIS or WiX for MSI generation).

Acceptance:

- The signed Windows installer runs without a SmartScreen warning on a clean Windows 10/11 machine.
- Unsigned local builds still produce a runnable `.exe`.

Delegation: needs human judgment. Windows signing credential setup requires the project owner's involvement.

---

### Cut E1-7: Linux AppImage

Files to touch: `.github/workflows/desktop-release.yml`, `Makefile`.

Use `wails build -platform linux/amd64`. Package the binary as an AppImage. No code signing is required; provide a SHA-256 checksum file alongside the AppImage download.

Note: on some Linux distributions, the system WebKit version may be outdated. Document the minimum WebKit version requirement (WebKitGTK 4.0 or later) in `docs/operations/`.

Acceptance:

- The AppImage runs on Ubuntu 22.04 LTS and Fedora 40 without additional dependencies beyond WebKitGTK.
- A SHA-256 checksum file is published alongside the AppImage.

Delegation: Codex CLI-friendly. Standard AppImage packaging; checksum generation is mechanical.

---

### Cut E1-8: Build pipeline

Files to touch: `.github/workflows/desktop-release.yml`, `Makefile`.

Add a `make desktop-build` target that runs `wails build` for the current platform. Add a GitHub Actions matrix job that runs on `macos-latest`, `windows-latest`, and `ubuntu-latest`, producing three release artifacts. The workflow triggers on version tags (`v*.*.*`).

Set `RELEASE=1` as an environment variable to gate real signing and notarization steps. Without `RELEASE=1`, the workflow builds unsigned binaries for validation.

Acceptance:

- `make desktop-build` produces a runnable binary on the developer's local machine.
- A GitHub Actions dry run (without signing secrets) produces binaries for all three platforms without errors.
- A `RELEASE=1` run (with secrets) produces signed artifacts for macOS and Windows.

Delegation: Codex CLI-friendly. Matrix CI configuration is repetitive; the signing steps follow the patterns from E1-5 and E1-6.

---

### Cut E1-9: Smoke test on each platform

Files to create: `docs/validation/wails-desktop-smoke-test.md`.

Manual smoke test procedure documented in `docs/validation/wails-desktop-smoke-test.md`:

1. Install the platform binary.
2. Confirm the three-pane workspace loads.
3. Select the `terisuke` persona and start an interview with a stub LLM response.
4. Confirm the Tailnet LLM status pill shows the expected endpoint.
5. Confirm "Open data folder" (File menu) opens the correct directory.
6. Quit the app and confirm no orphan processes remain.

This test must be run by a human on each target platform before a version tag is pushed.

Acceptance:

- All six steps pass on macOS (arm64), Windows 10/11 (amd64), and Ubuntu 22.04 (amd64).
- Results are recorded in `docs/validation/wails-desktop-smoke-test.md` with platform, binary version, and tester name.

Delegation: human judgment required. Platform-specific verification cannot be automated in CI.

---

## Risk register

| Risk | Mitigation |
|---|---|
| Wails v3 reaches stable before Tier 2 ships | Evaluate migration at that point; v2 API surface used here has v3 equivalents. No lock-in to v2-only APIs. |
| WebView2 (Microsoft's Chromium-based embedded browser component for Windows) is not installed on older Windows machines | Document the WebView2 prerequisite. Include a bootstrapper in the MSI that installs WebView2 if absent. |
| WebKit version on older Linux distros renders the UI incorrectly | Document the minimum WebKitGTK 4.0 requirement. Test on Ubuntu 22.04 LTS as the minimum baseline. |
| Code-signing secrets rotate | Store all secrets in GitHub Actions; document the rotation procedure in `docs/operations/`. Never embed secrets in source. |
| Unsigned macOS builds warn Gatekeeper before E1-5 | Expected behavior for developer builds. Document the `xattr -d com.apple.quarantine` workaround for testers. |

## Validation baseline

Run after each cut before merging:

```
go test ./...
python3 -m pytest tests/e2e -q
./scripts/check-launcher.sh
git diff --check
```

Tier 2 additional checks:

```
wails build -dry-run   # if supported; otherwise: wails build -platform <current>
make desktop-build
```

## Delegation matrix

| Cut | Best owner | Reason |
|---|---|---|
| E1-1: Wails scaffolding | Codex CLI | Boilerplate entry point; established handler wiring pattern |
| E1-2: Asset pipeline reuse | Cursor | `wails.json` configuration change |
| E1-3: Native menus | Codex CLI | Well-specified menu API; calls existing health-check logic |
| E1-4: First-run data dir | Cursor | Mirrors existing launcher path logic |
| E1-5: macOS signing | Human judgment | Requires project owner credentials and Apple toolchain |
| E1-6: Windows signing | Human judgment | Requires project owner credentials and Windows toolchain |
| E1-7: Linux AppImage | Codex CLI | Standard packaging; checksum generation is mechanical |
| E1-8: Build pipeline | Codex CLI | Matrix CI configuration is repetitive |
| E1-9: Smoke test | Human judgment | Platform-specific manual verification |
