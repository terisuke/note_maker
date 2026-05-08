# Note Maker app handoff - 2026-05-03

This handoff records the current app-like release baseline after PR #87 and is
the starting point for develop-to-main promotion.

## Current branch and release state

- Source branch: `develop`
- Latest merged app cut: PR #87, merge commit `4af05cadaeacc5fa10bdd22aa4d5992200c7d8ba`
- Closed by PR #87: [#14](https://github.com/terisuke/note_maker/issues/14), [#15](https://github.com/terisuke/note_maker/issues/15)
- Still open by design: [#36](https://github.com/terisuke/note_maker/issues/36), [#45](https://github.com/terisuke/note_maker/issues/45)

The app is usable as a local web app through the launcher. It is not a signed
native desktop bundle yet.

## How to start the app

Recommended path:

```bash
make launcher
```

Equivalent alias:

```bash
make app
```

The launcher:

- builds and starts the Go server,
- chooses the requested port or the next available port,
- opens the browser unless `--no-open` is used,
- stores app data in a user data directory,
- checks Evo X2 Tailnet primary LLM health before startup,
- reports fallback health,
- stops the managed server on `Ctrl-C` / `SIGTERM`,
- does not start the workstation-local LLM unless explicitly requested.

Useful variants:

```bash
make launcher-status
make launcher-check
make launcher-local
./scripts/launcher.sh --allow-degraded
./scripts/launcher.sh --strict-port
```

Use `make launcher-local` only for an intentional local-LLM diagnostic. The
product default is Evo X2 Ollama over Tailnet.

## Runtime contract

Primary runtime:

```text
http://evo-x2.tailb30e58.ts.net/v1
```

Fallback order:

1. Evo X2 Ollama OpenAI-compatible API over Tailnet.
2. Evo X2 llama.cpp OpenAI-compatible API at `/llama/v1`.
3. Workstation-local llama.cpp at `http://127.0.0.1:8081/v1`.

SSH tunnels are diagnostics only. They must not become the app default because
they depend on per-device authentication and cannot be shared by other Tailnet
devices.

Phase model defaults:

| Phase | Default |
|---|---|
| Style/source summarization | `gemma4:e2b` |
| Brief follow-up questions | `qwen3.6:27b` |
| Article compatibility generation | `gemma4:e2b` |
| Draft generation | `gemma4:31b` |
| Final verification | `gemma4:latest` |

Users can override these from the app settings.

## Storage contract

The settings UI can switch between SQLite and JSON unless environment variables
lock the store. SQLite is the default because the app now persists related
style, brief, project, article, draft, and regeneration records.

- SQLite store: `data/workflow_store.db`
- JSON compatibility store: `data/workflow_store.json`
- Launcher user data directory:
  - macOS: `~/Library/Application Support/Note Maker`
  - Linux/other: `$XDG_DATA_HOME/note-maker` or `~/.local/share/note-maker`

When the SQLite store is empty and a legacy `workflow_store.json` exists next
to it, the server imports existing author styles, sessions, briefs, brief
versions, and custom personas on startup. JSON remains readable as an explicit
compatibility mode.

Current persisted product memory includes:

- author style analyses and writing guides,
- brief sessions and answers,
- saved briefs and explicit brief versions,
- projects, articles, source snapshots, drafts, draft versions, and final verification metadata in SQLite,
- built-in plus custom personas,
- custom persona update/delete with history-reference protection.

## User-facing app surface

Implemented:

- persona and output-format selection for note, Cor blog, Zenn, Qiita, and homepage section output,
- media-aware style-source presets,
- plain fixed questions plus targeted deep dives,
- SSE streaming and cancellation for follow-up and draft generation,
- editable answer transcript with fork-on-edit lineage,
- editable draft Markdown and section regeneration,
- style/brief/project/article/draft/source history cards,
- custom persona create/update/delete,
- brief version history display,
- storage-mode setting,
- model setting.

Known limitations:

- The launcher is app-like, not a signed native bundle.
- The local workstation llama.cpp fallback has tooling but no passing live proof yet.
- Evo X2 llama.cpp `/llama/v1` works as a route, but is not ready to replace Ollama primary.
- Brief version UI is read-only; diff/restore can be separate follow-up work.
- Custom persona source management is minimal beyond the current source fields.

## Validation baseline

PR #87 validation:

```bash
go test ./...
python3 -m pytest tests/e2e -q
bash -n scripts/*.sh && zsh -n scripts/*.sh && ./scripts/check-launcher.sh
git diff --check
make scenario-local-llamacpp-fallback
RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 make scenario-evo-x2-llama-brief-draft
make evo-x2-models
```

Observed #45 direct `/llama/v1` run:

| Metric | Value |
|---|---:|
| `scenario_passed` | `true` |
| score | `88.2 / 80.0` |
| keyword overlap | `65 / 70` |
| runes | `3075 / 2800` |
| verification | passed |
| first chunk | `14105ms` |
| chunks | `1785` |
| draft elapsed | `59.33s` |

This proves the route is usable for fallback experimentation. It does not close
#45 because first-chunk latency and keyword overlap still miss the promotion
gates.

Observed #36 local fallback status:

- `make scenario-local-llamacpp-fallback` writes a plan/report without starting
  a local model.
- `http://127.0.0.1:8081/v1/models` returned `connection refused`.
- #36 remains open until a gated local loopback llama.cpp run records
  `score >= 82.0`, `keyword_overlap >= 70`, and `runes >= 2800`.

## Main promotion checklist

Before promoting develop to main:

1. Confirm open issues are only expected follow-ups (#36 and #45 at the time of this handoff).
2. Confirm no app-breaking PRs are open against develop.
3. Run the fast validation baseline:

```bash
go test ./...
python3 -m pytest tests/e2e -q
./scripts/check-launcher.sh
git diff --check
```

4. Create a develop-to-main PR.
5. Verify GitHub checks.
6. Merge the main PR.

## Next engineering work

Priority order:

1. #36: start an approved workstation-local llama.cpp endpoint and capture a
   live passing fallback report.
2. #45: reduce Evo X2 llama.cpp first-token latency and improve
   keyword-overlap consistency without disrupting Ollama primary.
3. Split optional app packaging follow-ups only if a signed native wrapper,
   icon, signing, or installer is required.
4. Add brief-version diff/restore only after users confirm the read-only
   version history is useful.
