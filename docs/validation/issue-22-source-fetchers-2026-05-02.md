# Issue 22 source fetcher validation

Date: 2026-05-02

## Scope

Validate [#22](https://github.com/terisuke/note_maker/issues/22): route source acquisition beyond note.com to Zenn, Qiita, RSS/Atom, generic HTML, and GitHub-backed Markdown while keeping normal tests offline.

Correction note: the first validation pass only proved one current item per source. That was not enough for Cloudia/Zenn/Qiita/Cor blog style analysis. This document now records the 2026-05-02 corrective validation against historical user/account sources with a high limit and body-length checks.

## Current source facts checked

- Qiita API v2 documents `GET /api/v2/users/:user_id/items` as the public user item list endpoint in newest order, with `page` and `per_page` parameters.
- Qiita API v2 documents unauthenticated access as limited to `60` requests per hour per IP address, so the fetcher uses one request per user-list scenario and no polling loop.
- Zenn's official RSS article documents user feeds as `https://zenn.dev/ユーザー名/feed`.
- Zenn's official RSS article documents `?all=1` for including all public posts instead of the default limited feed, so the Zenn fetcher uses that query for user lists.
- Zenn user RSS is a list source. The implementation now expands each feed item URL through the article page's embedded Next.js data so style analysis receives full article body text, not feed summaries.
- RSS/Atom parsing is implemented through standard XML feeds, with RSS 2.0 item fields and Atom entries handled in one `RSSFetcher`.
- GitHub's repository contents API can read public repository files without authentication. Cor.inc blog Markdown is therefore fetched from `Cor-Incorporated/corsweb2024/src/content/blog/ja/*.md`, preserving Astro frontmatter and body Markdown.
- Cor.inc `https://cor-jp.com/rss.xml` is useful for discovery and ordering, but it only contains short descriptions. The canonical full body source for company blog style analysis is GitHub Markdown.

## Implementation

- `internal/domain/source` defines `Kind`, `Ref`, `ProfileSnapshot`, and `ArticleSnapshot`.
- `internal/infrastructure/source.Router` dispatches by explicit selector or URL host:
  - `note:<user>` and note.com URLs go to the existing note fetcher.
  - `zenn:<user>` and zenn.dev URLs go to Zenn RSS/HTML fetchers.
  - `qiita:<user>` and qiita.com URLs go to Qiita API v2 fetchers.
  - `rss:<url>` and feed-like URLs go to RSS/Atom parsing.
  - `html:<url>` and unknown article URLs go to semantic HTML extraction.
  - `github:<owner>/<repo>/<path>` and GitHub/blob/raw URLs go to GitHub repository Markdown fetching.
- `AnalyzeAuthorStyleHandler` and the legacy article-generation handler now use the router through an adapter that still satisfies the existing `FetchArticle` / `FetchUserLatestArticles` application interface.
- `cmd/scenario/source_fetch` accepts `SOURCE_FETCH_LIMIT`; the default is now multi-item validation rather than a one-item smoke test.

## Scenario

Command:

```bash
RUN_SOURCE_FETCH_SCENARIO=1 \
SCENARIO_OUTPUT_DIR=tmp/source_fetch_cloudia_history \
SOURCE_FETCH_LIMIT=100 \
SOURCE_FETCH_NOTE=note:cor_instrument \
SOURCE_FETCH_ZENN=zenn:cloudia \
SOURCE_FETCH_QIITA=qiita:Cloudia_Cor_Inc \
SOURCE_FETCH_RSS=rss:https://cor-jp.com/rss.xml \
SOURCE_FETCH_GITHUB=github:Cor-Incorporated/corsweb2024/src/content/blog/ja \
go run ./cmd/scenario/source_fetch
```

Result:

| Source | Selector | Articles | Elapsed | Avg content length | Output |
|---|---|---:|---:|---:|---|
| note | `note:cor_instrument` | 20 | 6.99s | not part of this corrective check | `tmp/source_fetch_cloudia_history/note.json` |
| Zenn | `zenn:cloudia` | 7 | 1.08s | 4,139 chars | `tmp/source_fetch_cloudia_history/zenn.json` |
| Qiita | `qiita:Cloudia_Cor_Inc` | 12 | 0.28s | 3,274 chars | `tmp/source_fetch_cloudia_history/qiita.json` |
| Cor RSS | `rss:https://cor-jp.com/rss.xml` | 10 | 0.16s | 69 chars | `tmp/source_fetch_cloudia_history/rss.json` |
| Cor GitHub Markdown | `github:Cor-Incorporated/corsweb2024/src/content/blog/ja` | 10 | 0.62s | 4,218 chars | `tmp/source_fetch_cloudia_history/github.json` |

Historical-source check:

- Zenn `cloudia` returned all 7 public feed items from `?all=1`, with article-page body extraction.
- Qiita `Cloudia_Cor_Inc` returned 12 public user items through the official user items API, with Markdown `body`.
- Cor RSS returned 10 public feed items but only short descriptions, confirming that RSS alone is insufficient for company-blog style analysis.
- Cor GitHub Markdown returned 10 Japanese blog Markdown files with frontmatter and full body content, confirming this is the practical source for company-blog output mode.

## Tests

```bash
go test ./...
```

Focused source tests cover:

- RSS 2.0 `content:encoded` parsing.
- Atom entry parsing.
- explicit `zenn:`, `qiita:`, and `rss:` selector routing.
- explicit `github:` selector routing.
- host-based routing for note.com, zenn.dev, qiita.com, GitHub/blob/raw URLs, RSS-like URLs, and generic HTML.
