# Issue #91 - D2-3 Grid Scaffold and D2-4 History Rail

Date: 2026-05-04
Branch: `feat/d2-3-grid-scaffold`

## Scope

- Replaced the single-column `.workflow` shell with a three-pane `.workspace`
  CSS Grid: `240px | 1fr | 360px`.
- Placed configuration and history in the left rail, style analysis and
  interview in the center pane, and draft generation in the right rail.
- Added the D2-4 left-rail conversation list as a one-click history entry
  point backed by the existing `/api/workflow/artifacts` history index.
- Kept the six legacy history `<select>` controls in the DOM inside
  `<details class="history-advanced-picker" open>` so existing Playwright
  selectors and operator workflows remain compatible during the migration.
- Mirrored loaded history articles and the selected article id into
  `Alpine.store('app').history`.
- Kept the store mirror in sync when a draft implicitly selects an article and
  when the history selection is cleared.

## Browser Layout Check

Python Playwright layout probe against `http://127.0.0.1:8090`:

```json
{
  "desktop": {
    "columns": "240px 840px 360px",
    "detailsOpen": true,
    "listItems": 3,
    "ids": true,
    "panes": [240, 840, 360]
  },
  "mobile": {
    "columns": "390px",
    "bodyWidth": 390,
    "viewport": 390
  }
}
```

## Validation

| Command | Result |
|---|---|
| `go test ./...` | Pass |
| `python3 -m pytest tests/e2e -q` | Pass, `13 passed in 8.88s` |
| `./scripts/check-launcher.sh` | Pass |
| `git diff --check` | Pass |

## Notes

- D2-5 is deliberately not included in this cut. `static/history_ui_test.go`
  currently asserts `#style-result #style-guide-card` and
  `#brief-result #brief-card`; moving artifact cards to the right rail needs a
  matching compatibility decision or static contract update.
