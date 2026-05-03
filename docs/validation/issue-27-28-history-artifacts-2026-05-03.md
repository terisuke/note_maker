# Issue 27/28 history and artifact UI/API validation

Date: 2026-05-03
Branch: `codex/issue27-28-history-artifacts`; reviewed again from `codex/phase-c-history-e2e`

## Scope

This validation covers:

- [#27](https://github.com/terisuke/note_maker/issues/27) first saved-history picker cut.
- [#28](https://github.com/terisuke/note_maker/issues/28) first human-readable style-guide and brief artifact card cut.

It deliberately did not claim completion for add-persona authoring UI, broader edit persistence, project/article/draft history browsing, draft version browsing, or Browser E2E coverage. Later cuts add those pieces incrementally; see [Phase C persona/history/card polish validation](./phase-c-persona-history-card-polish-2026-05-03.md) for the custom persona and editable brief/style card follow-up.

## What changed

- Added history read routes:
  - `GET /api/history`
  - `GET /api/workflow/artifacts`
  - `GET /api/author-style`
  - `GET /api/brief-sessions`
  - `GET /api/briefs`
  - `GET /api/briefs/{id}`
- Added store list methods:
  - memory: `ListAuthorStyles`, `ListSessions`, `ListBriefs`
  - SQLite: `ListAuthorStyles`, `ListSessions`, `ListBriefs`, `ListProjects`, `ListArticlesByProject`
- Added the web UI `履歴から再開` section with saved style-guide/session pickers.
- Added readable style-guide and article-brief cards while keeping raw Markdown/JSON disclosures.
- Added focused handler and static UI contract tests.

## API Contract

`GET /api/workflow/artifacts` and `GET /api/history` return one reusable index:

```json
{
  "style_guides": [],
  "sessions": [],
  "briefs": []
}
```

The UI then opens details through existing or focused endpoints:

- style guide detail: `GET /api/author-style/{id}`
- session detail: `GET /api/brief-sessions/{id}`
- brief detail: `GET /api/briefs/{id}`

## Verification

Command:

```sh
go test ./internal/handlers ./static
```

Result:

```text
ok  	github.com/teradakousuke/note_maker/internal/handlers	(cached)
ok  	github.com/teradakousuke/note_maker/static	(cached)
```

Follow-up review on `codex/phase-c-history-e2e`:

```sh
go test ./...
node --check static/js/script.js
git diff --check
```

These passed after the project/article/draft history follow-up was integrated. The follow-up adds SQLite-backed read routes and UI contract coverage; the later #13 Playwright cut supplies the browser E2E close signal.

Final follow-up validation after the fixture alignment:

```sh
go test ./...
go test ./cmd/server ./internal/handlers ./static
node --check static/js/script.js
git diff --check
```

All passed. Project/article/draft history can continue as Phase C product work under #14/#27/#28; #13 browser E2E is covered by the later Playwright validation cut.

## Acceptance Status

- Saved style guides can be listed for picker UIs: done.
- Saved interview sessions can be listed with completion and brief availability metadata: done.
- Completed briefs can be listed and retrieved by session id: done.
- Combined workflow artifact index returns style guides, sessions, and briefs: done.
- UI includes `履歴から再開`, refresh/open/clear controls, saved style/session selects, and status messaging: done.
- Style guide is rendered as a readable card and raw Markdown remains available: done.
- Article brief is rendered as a readable card and raw JSON remains available: done.

## Remaining Work

- Add-persona authoring UI is implemented by the Phase C persona/history/card polish cut for create/list. Custom persona update/delete remains follow-up work.
- Brief/style card edit persistence is implemented by the Phase C polish cut. Brief edits update the saved artifact without rewriting original session answers; style edits create a new saved guide version.
- Project/article/draft history browsing from SQLite has a follow-up implementation through read APIs and history cards. Treat remaining full product-memory semantics as #14 follow-up.
- Browser E2E coverage for the new history picker and cards is recorded in [Issue #13 Browser E2E Validation](./issue-13-browser-e2e-2026-05-03.md).
