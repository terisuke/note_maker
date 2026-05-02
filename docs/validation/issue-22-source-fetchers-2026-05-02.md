# Issue 22 source fetcher validation

Date: 2026-05-02

## Scope

Validate [#22](https://github.com/terisuke/note_maker/issues/22): route source acquisition beyond note.com to Zenn, Qiita, RSS/Atom, and generic HTML while keeping normal tests offline.

## Current source facts checked

- Qiita API v2 documents `GET /api/v2/users/:user_id/items` as the public user item list endpoint in newest order, with `page` and `per_page` parameters.
- Qiita API v2 documents unauthenticated access as limited to `60` requests per hour per IP address, so the fetcher uses one request per user-list scenario and no polling loop.
- Zenn's official RSS article documents user feeds as `https://zenn.dev/ユーザー名/feed`.
- Zenn's official RSS article documents `?all=1` for including all public posts instead of the default limited feed, so the Zenn fetcher uses that query for user lists.
- RSS/Atom parsing is implemented through standard XML feeds, with RSS 2.0 item fields and Atom entries handled in one `RSSFetcher`.

## Implementation

- `internal/domain/source` defines `Kind`, `Ref`, `ProfileSnapshot`, and `ArticleSnapshot`.
- `internal/infrastructure/source.Router` dispatches by explicit selector or URL host:
  - `note:<user>` and note.com URLs go to the existing note fetcher.
  - `zenn:<user>` and zenn.dev URLs go to Zenn RSS/HTML fetchers.
  - `qiita:<user>` and qiita.com URLs go to Qiita API v2 fetchers.
  - `rss:<url>` and feed-like URLs go to RSS/Atom parsing.
  - `html:<url>` and unknown article URLs go to semantic HTML extraction.
- `AnalyzeAuthorStyleHandler` and the legacy article-generation handler now use the router through an adapter that still satisfies the existing `FetchArticle` / `FetchUserLatestArticles` application interface.

## Scenario

Command:

```bash
RUN_SOURCE_FETCH_SCENARIO=1 \
SCENARIO_OUTPUT_DIR=tmp/source_fetch_issue22 \
SOURCE_FETCH_NOTE=note:cor_instrument \
SOURCE_FETCH_ZENN=zenn:zenn \
SOURCE_FETCH_QIITA=qiita:Qiita \
SOURCE_FETCH_RSS=rss:https://zenn.dev/zenn/feed \
go run ./cmd/scenario/source_fetch
```

Result:

| Source | Selector | Articles | Elapsed | Output |
|---|---|---:|---:|---|
| Qiita | `qiita:Qiita` | 1 | 0.68s | `tmp/source_fetch_issue22/qiita.json` |
| RSS | `rss:https://zenn.dev/zenn/feed` | 1 | 0.18s | `tmp/source_fetch_issue22/rss.json` |
| note | `note:cor_instrument` | 1 | 0.66s | `tmp/source_fetch_issue22/note.json` |
| Zenn | `zenn:zenn` | 1 | 0.19s | `tmp/source_fetch_issue22/zenn.json` |

Recent-source check:

- Qiita returned `Qiita アップデートサマリー - 2026年4月`, which is inside the requested recent two-month window from 2026-05-02.
- Zenn/RSS returned Zenn official content from the current public feed path.
- note returned the latest public `cor_instrument` article available through the existing note source path.

## Tests

```bash
go test ./...
```

Focused source tests cover:

- RSS 2.0 `content:encoded` parsing.
- Atom entry parsing.
- explicit `zenn:`, `qiita:`, and `rss:` selector routing.
- host-based routing for note.com, zenn.dev, qiita.com, RSS-like URLs, and generic HTML.
