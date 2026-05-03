# Phase C Persona/History/Card Polish Validation

Date: 2026-05-03
Branch: `codex/phase-c-persona-history-polish`
Route: C, docs/coordination
Base: `develop` after PR #85 merge

## Scope

This validates the Phase C product polish cut for custom persona create/list and editable brief/style cards.

This PR implements:

- built-in plus custom persona listing through `GET /api/personas`;
- custom persona creation through `POST /api/personas`;
- custom persona persistence in memory and SQLite stores, including SQLite reopen coverage;
- add-persona UI that saves, selects, reloads, and aligns the history persona selector;
- saved style-guide and brief-session reuse through `履歴から再開`;
- SQLite-backed project/article/draft/source-snapshot read routes and history cards when the active store supports them;
- human-readable style-guide, brief, project, article, current-draft, draft-version, and source-snapshot cards;
- style-guide card edit/save through `PATCH /api/author-style/{id}` / `POST /api/author-style/{id}/versions`;
- brief card edit/save through `PATCH /api/briefs/{id}`;
- editable draft Markdown and section-regeneration candidate accept/reject flow;
- browser E2E baseline for #13 in `tests/e2e`.

Known limitations:

- custom persona update/delete is not implemented;
- richer persona source editing is not implemented after create;
- brief-card edits update the saved brief artifact but do not rewrite original session answers or create a separate brief-version table;
- project/article/draft/source-snapshot cards remain read-only;
- #14 remains open for broader queryable product memory and version/history semantics.

## Local Validation

Final validation commands:

```sh
node --check static/js/script.js
go test ./cmd/server ./internal/handlers ./internal/infrastructure/repository/memory ./internal/infrastructure/repository/sqlite ./static
python3 -m pytest tests/e2e/test_phase_c_persona_history_polish.py tests/e2e/test_history_stream_regenerate.py -q
git diff --check
```

Final local results:

```text
node --check static/js/script.js
passed

go test ./cmd/server ./internal/handlers ./internal/infrastructure/repository/memory ./internal/infrastructure/repository/sqlite ./static
ok github.com/teradakousuke/note_maker/cmd/server
ok github.com/teradakousuke/note_maker/internal/handlers
ok github.com/teradakousuke/note_maker/internal/infrastructure/repository/memory
ok github.com/teradakousuke/note_maker/internal/infrastructure/repository/sqlite
ok github.com/teradakousuke/note_maker/static

python3 -m pytest tests/e2e/test_phase_c_persona_history_polish.py tests/e2e/test_history_stream_regenerate.py -q
8 passed

git diff --check
passed
```

## Issue Close/Open Proposal

| Issue | Proposal | Required before closure |
|---|---|---|
| [#13](https://github.com/terisuke/note_maker/issues/13) | Closed | Browser E2E validation is already recorded in [Issue #13 Browser E2E Validation](./issue-13-browser-e2e-2026-05-03.md). |
| [#14](https://github.com/terisuke/note_maker/issues/14) | Keep open | This cut adds custom persona persistence and editable brief/style cards, but broader queryable product memory, persona update/delete, and complete artifact version/history semantics remain. |
| [#27](https://github.com/terisuke/note_maker/issues/27) | Close with this PR if create/list satisfies the persona authoring scope | Custom personas can be created, persisted, listed with built-ins, selected, reloaded, and used to fetch persona-scoped history. Track persona update/delete separately if needed. |
| [#28](https://github.com/terisuke/note_maker/issues/28) | Close with this PR if brief/style edit persistence satisfies the card scope | Brief and style cards support edit, cancel, save, and error handling. Style edits create a new saved guide version; brief edits persist the saved artifact while preserving raw/session audit data. |

## Suggested Merge Comment

Phase C persona/history/card polish validated. #13 is closed. This PR adds custom persona create/list with persistence, plus editable brief/style card persistence. Recommend keeping #14 open for broader queryable product memory and closing #27/#28 if the issue owner accepts create/list and brief/style edit persistence as the intended scope.
