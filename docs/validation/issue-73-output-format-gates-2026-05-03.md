# Issue #73 output-format-specific scenario gates

Date: 2026-05-03

## Scope

Implemented output-format-specific gates for the media matrix scenarios without changing draft generation internals.

Owned areas changed:

- `cmd/scenario/media_matrix`
- `cmd/scenario/live_media_matrix`
- `docs/validation`

## Gate Policy

| Case | Gate policy |
|---|---|
| `terisuke_note_essay` | min style `82.0`, min runes `2800`, note long-form structure |
| `cor_blog_technical_report` | min style `80.0`, min runes `2200`, Cor blog report/frontmatter/verification structure |
| `cor_blog_vision_sharing` | min style `80.0`, min runes `1600`, Cor blog company-policy/frontmatter structure |
| `cloudia_zenn_tutorial` | min style `82.0`, min runes `1800`, Zenn frontmatter/topics/message/code structure |
| `cloudia_qiita_how_to` | min style `82.0`, min runes `1400`, Qiita frontmatter/note/diff/repro structure |
| `cor_homepage_section` | min style `72.0`, min runes `350`, short HTML `section`/`h2`/`p`/CTA/concise-copy structure |

The homepage section intentionally uses a short HTML gate instead of long-form article length. Note, Cor blog, Zenn, and Qiita remain strict long-form gates.

## Offline Checks

```sh
go test ./cmd/scenario/media_matrix ./cmd/scenario/live_media_matrix
```

Result:

```text
ok  	github.com/teradakousuke/note_maker/cmd/scenario/media_matrix
ok  	github.com/teradakousuke/note_maker/cmd/scenario/live_media_matrix
```

```sh
SCENARIO_OUTPUT_DIR=tmp/media_matrix go run ./cmd/scenario/media_matrix
```

Result:

```text
media matrix scenario completed
offline_only=true
source_selectors=5
question_template_ids=14
composed_templates=10
cases=6
matrix=tmp/media_matrix/matrix.json
cases_markdown=tmp/media_matrix/cases.md
```

```sh
SCENARIO_OUTPUT_DIR=tmp/media_matrix go run ./cmd/scenario/live_media_matrix
```

Result:

```text
live media matrix completed
live=false
cases=6
aggregate=tmp/media_matrix/live/aggregate.json
report=tmp/media_matrix/live/aggregate.md
```

No Evo X2 live call was run.
