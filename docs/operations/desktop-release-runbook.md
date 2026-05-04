# Desktop Release Runbook

This runbook covers the Tier 2 Wails desktop packaging path.

## Local Development Build

The Makefile uses the pinned Wails CLI version from `WAILS_VERSION` and does
not require a global `wails` install.

```sh
make desktop-check
make desktop-dev
```

Build the current platform:

```sh
make desktop-build
```

`make desktop-assets` syncs the existing `static/` assets into
`cmd/desktop/frontend/` before checks and builds. The Wails binary embeds that
copied frontend, while the preview server can still use the repo-local `static/`
tree. No npm, Vite, or Webpack step is required.

On first launch the desktop binary bootstraps the same data paths as
`scripts/launcher.sh`:

- macOS: `~/Library/Application Support/Note Maker`
- Linux: `$XDG_DATA_HOME/note-maker` or `~/.local/share/note-maker`
- Windows: `%APPDATA%\Note Maker`

The bootstrap creates `logs/`, sets `NOTE_MAKER_CONFIG_PATH` and
`WORKFLOW_STORE_PATH`, and writes a default `app_config.json` only when one does
not already exist.

## CI Dry Run

Run the `Desktop Release` workflow manually with `release=0`.

Set `dry_run=1` to upload only Wails dry-run command logs and checksums.

If the workflow file is present on `develop` but not yet on the repository
default branch, GitHub Actions manual dispatch returns `404`. In that state,
run the local release readiness helper instead and promote `develop` to the
default branch before attempting the Actions dispatch:

```sh
./scripts/release-readiness-check.sh
gh workflow list --all
```

Expected output:

- macOS unsigned developer artifact archive.
- Windows unsigned developer artifact archive.
- Linux unsigned developer artifact archive.
- Wails dry-run command log for each platform.
- SHA-256 checksum file covering the archive and dry-run log.

Unsigned artifacts are for validation only. macOS and Windows users should
expect operating-system warnings.

## Signed Release

Create or push a tag matching `v*.*.*`, or manually dispatch the workflow with
`release=1`.

Required GitHub Actions secrets:

- `APPLE_CERTIFICATE`
- `APPLE_CERTIFICATE_PASSWORD`
- `APPLE_TEAM_ID`
- `APP_STORE_CONNECT_KEY_ID`
- `APP_STORE_CONNECT_ISSUER_ID`
- `APP_STORE_CONNECT_KEY`
- `WINDOWS_CERTIFICATE`
- `WINDOWS_CERTIFICATE_PASSWORD`

The workflow verifies that secrets are present before a release run. Final
signing and notarization remain operator-controlled because certificate
material and platform-specific installer policy are outside the source tree.
Use `gh secret list --repo <owner>/<repo>` to verify the secret names are
configured before dispatching a signed release. A missing secret-name check is a
release blocker, but not a source-tree blocker.

## Smoke Test

For each produced platform artifact:

1. Start the app.
2. Confirm the existing Note Maker UI loads.
3. Confirm `/static/vendor/alpine.min.js` and `/static/js/store.js` load.
4. Use the native menu to open the data folder.
5. Use the native menu to run LLM diagnostics against the configured endpoint.
6. Create or open a history item without changing the Tier 1 data directory.

Record platform, artifact name, checksum, tester, and result in
`docs/validation/`.
