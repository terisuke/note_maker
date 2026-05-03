# Issue 15 launcher validation

Date: 2026-05-03

Scope: packaging/dev launcher files only.

## Implemented behavior

- Added `scripts/launcher.sh` as an app-like launcher for the Go web app.
- Defaults to Evo X2 Tailnet as the primary OpenAI-compatible LLM health path.
- Does not start local `llama-server` unless `--local-llm`, `make launcher-local`, or `NOTE_MAKER_START_LOCAL_LLM=1` is used.
- Selects the requested port or the next available port unless `--strict-port` is set.
- Uses a user data directory for app config, workflow store, logs, and the built server binary.
- Stops the managed server process on `SIGTERM`, `SIGINT`, or shell exit.

## Validation

```bash
./scripts/check-launcher.sh
```

Result: passed. This ran `bash -n` for launcher-related scripts, `shellcheck` when available, the launcher shell tests, and Make dry-runs for launcher targets.

```bash
go test ./...
```

Result: passed.

```bash
make launcher-status
```

Result: passed. Evo X2 Tailnet primary was reachable at `http://evo-x2.tailb30e58.ts.net/v1/models`; Evo X2 llama.cpp fallback was reachable at `http://evo-x2.tailb30e58.ts.net/llama/v1/models`; local fallback `http://127.0.0.1:8081/v1/models` was not running and was not started.

```bash
./scripts/launcher.sh --no-open
kill -TERM <launcher-pid>
lsof -nP -iTCP:8080 -sTCP:LISTEN
```

Result: launcher built the server binary, started the app on `http://127.0.0.1:8080`, then stopped the managed server after `SIGTERM`; no listener remained on port 8080.

## Closure status

This substantially implements issue #15 with a pragmatic shell launcher. A full Tauri/Electron wrapper, app icon, signing, and installer packaging remain future packaging work, but the user-facing startup path, process lifecycle, health checks, storage defaults, and Make/mise entrypoints are in place.
