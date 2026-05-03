# Issue 27/28 History And Artifact API Cut

Date: 2026-05-03
Branch: `codex/issue27-28-history-artifacts`

## Purpose

This cut makes persisted workflow memory visible enough for day-to-day drafting:

- reuse a saved writing style guide without re-fetching sources,
- reopen a saved interview session,
- read completed briefs as cards instead of raw JSON,
- keep the raw Markdown/JSON available for audit.

It is intentionally smaller than the full Phase C vision in ADR 0002. Project/article/draft browsing is left as a follow-up on top of the richer SQLite schema.

## Implemented Store Surface

The handler-facing `workflowStoreBackend` now exposes the current reusable workflow artifacts:

```go
ListAuthorStyles() ([]authorstyle.AnalyzeResult, error)
ListSessions() ([]brief.ArticleBriefSession, error)
ListBriefs() (map[string]brief.ArticleBrief, error)
```

Both the JSON/memory store and SQLite store implement these methods.

SQLite also gained entry-point list methods for later project history work:

```go
ListProjects() ([]ProjectRecord, error)
ListArticlesByProject(projectID string) ([]ArticleRecord, error)
```

Those SQLite methods are not yet surfaced in the web UI.

## Implemented HTTP Surface

The web UI uses the combined index first:

- `GET /api/workflow/artifacts`
- `GET /api/history` as an alias

Response:

```json
{
  "style_guides": [],
  "sessions": [],
  "briefs": []
}
```

Focused read endpoints are also available:

- `GET /api/author-style` - saved style-guide artifacts
- `GET /api/author-style/{id}` - existing detail endpoint by analysis/profile/guide id
- `GET /api/brief-sessions` - saved interview session summaries
- `GET /api/brief-sessions/{id}` - existing session detail endpoint
- `GET /api/briefs` - completed brief artifacts
- `GET /api/briefs/{id}` - completed brief artifact by session id

## Implemented UI Surface

`static/index.html` now includes a `履歴から再開` area:

- persona filter for history selection,
- saved style-guide picker,
- saved interview-session picker,
- refresh/open/clear controls,
- loading, empty, and error states.

`static/js/script.js` restores selected history into the existing workflow state:

- selected style guide sets `profileId`, guide metadata, and style card,
- selected session restores `sessionId`, transcript state, completed brief, persona/format mode, and draft-generation readiness,
- selecting only a session resolves its style guide via `style_profile_id`.

## Human-Readable Artifacts

The UI now renders:

- style guide cards with profile/guide/article metadata and parsed Markdown sections,
- article brief cards with theme, reader, opening episode, action, must-include, context, exclusions, structure, tone, custom answers, and deep-dive answers.

Raw Markdown and JSON are still available behind disclosure controls. This keeps review/debug data accessible without making it the default user experience.

## Tests

Added:

- `internal/handlers/workflow_history_test.go`
- `cmd/server/main_test.go`
- `static/history_ui_test.go`

Validation commands:

```sh
node --check static/js/script.js
go test ./...
git diff --check
```

## Phase C Polish Follow-up

The `codex/phase-c-persona-history-polish` cut adds:

- custom persona create/list with memory and SQLite persistence,
- add-persona UI that saves and selects the custom persona,
- editable style-guide cards that save a new guide version,
- editable brief cards that save the updated brief artifact,
- browser E2E coverage for custom persona add/reload and brief/style save, cancel, and error states.

## Remaining Phase C Work

Not included in this cut:

- custom persona update/delete,
- richer persona source editing after create,
- project/article/draft editing and write workflows beyond the current read cards,
- richer draft-version and section-regeneration artifact operations beyond current browsing,
- browser E2E for the new history flow is covered by the later Issue #13 validation cut.
