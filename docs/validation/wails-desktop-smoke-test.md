# Wails Desktop Smoke Test

Use this checklist for each desktop artifact produced by the `Desktop Release`
workflow.

## Artifact

- Platform:
- Artifact name:
- SHA-256:
- Tester:
- Date:

## Checks

- App launches without a terminal.
- Existing Note Maker UI is visible.
- `static/vendor/alpine.min.js` loads.
- `static/js/store.js` loads and `Alpine.store('app')` exists.
- Native menu opens the configured data folder.
- Native menu LLM diagnostics reports the configured runtime, endpoint, model, and health result.
- Existing Tier 1 data directory is read without overwriting user data.
- A saved history item can be opened.
- App exits cleanly.

## Result

- Pass/fail:
- Notes:
