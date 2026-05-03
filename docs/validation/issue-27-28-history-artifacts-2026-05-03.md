# Issue 27/28 history and artifact UI/API validation

Date: 2026-05-03
Branch: `codex/issue27-28-history-artifacts`

## Scope

This validation covers:

- [#27](https://github.com/terisuke/note_maker/issues/27) first saved-history picker cut.
- [#28](https://github.com/terisuke/note_maker/issues/28) first human-readable style-guide and brief artifact card cut.

It deliberately does not claim completion for add-persona authoring UI, broader edit persistence, project/article/draft history browsing, draft version browsing, or Browser E2E coverage.

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

## Acceptance Status

- Saved style guides can be listed for picker UIs: done.
- Saved interview sessions can be listed with completion and brief availability metadata: done.
- Completed briefs can be listed and retrieved by session id: done.
- Combined workflow artifact index returns style guides, sessions, and briefs: done.
- UI includes `履歴から再開`, refresh/open/clear controls, saved style/session selects, and status messaging: done.
- Style guide is rendered as a readable card and raw Markdown remains available: done.
- Article brief is rendered as a readable card and raw JSON remains available: done.

## Remaining Work

- Add-persona authoring UI is still unimplemented.
- Broader edit persistence beyond the existing fork-on-edit/session save flow is still unimplemented.
- Project/article/draft history browsing from SQLite remains unimplemented in the UI.
- Browser E2E coverage for the new history picker and cards remains under [#13](https://github.com/terisuke/note_maker/issues/13).
