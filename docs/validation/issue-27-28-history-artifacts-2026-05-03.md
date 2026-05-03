# Issue 27/28 history and artifact UI/API validation

Date: 2026-05-03
Branch: `codex/issue27-28-history-artifacts`; reviewed again from `codex/phase-c-history-e2e`

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

Follow-up review on `codex/phase-c-history-e2e`:

```sh
go test ./...
node --check static/js/script.js
git diff --check
```

These passed after the project/article/draft history follow-up was integrated. The follow-up adds SQLite-backed read routes and UI contract coverage, but it is still browser-contract coverage rather than a real browser E2E close signal for #13.

Final follow-up validation after the fixture alignment:

```sh
go test ./...
go test ./cmd/server ./internal/handlers ./static
node --check static/js/script.js
git diff --check
```

All passed. Project/article/draft history can continue as implementation work, but #13 still needs real browser E2E before it closes.

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
- Project/article/draft history browsing from SQLite has a follow-up implementation through read APIs and history cards. Treat it as a separate #83 product-readiness cut from the original #27/#28 first-cut validation.
- Browser E2E coverage for the new history picker and cards remains under [#13](https://github.com/terisuke/note_maker/issues/13). Static contract tests alone are not enough to close #13.
