# Media matrix integrated evaluation

Date: 2026-05-03

## Scope

This validation plan covers the integrated cross-media path after the new source selectors and output modes are available. It intentionally starts offline and deterministic, then documents the live source/LLM runs to execute when a local model and network access are explicitly enabled.

The evaluation must vary all of these dimensions:

- Theme: reflective creation, implementation report, company vision, Zenn tutorial, Qiita how-to, homepage copy.
- Medium: note, Cor.inc company blog, Zenn, Qiita, homepage HTML section.
- Style: essay, technical report, vision sharing, tutorial, practical how-to, concise product section.
- Length: 400-700字 homepage copy through 3000字 note essay.

## Offline matrix scenario

Command:

```sh
SCENARIO_OUTPUT_DIR=tmp/media_matrix go run ./cmd/scenario/media_matrix
```

Expected output:

```text
media matrix scenario completed
offline_only=true
source_selectors=5
question_template_ids=9
composed_templates=10
cases=6
matrix=tmp/media_matrix/matrix.json
cases_markdown=tmp/media_matrix/cases.md
```

The command does not call public sources or an LLM. It verifies:

- Seeded personas expose these proven selectors:
  - `note:cor_instrument`
  - `zenn:cloudia`
  - `qiita:Cloudia_Cor_Inc`
  - `rss:https://cor-jp.com/rss.xml`
  - `github:Cor-Incorporated/corsweb2024/src/content/blog/ja`
- The baseline Terisuke note interview template still contains the required brief fields.
- All 10 composed templates across 2 personas x 5 formats are generated, including Cloudia technical questions and homepage CTA questions.
- Each case can assemble a completed `ArticleBrief` through the domain session flow using its own `persona_id x output_format_id` question template.
- Each case prompt contains the expected persona, output format, theme, target length, and medium-specific guide fragments.

Artifacts:

- `tmp/media_matrix/matrix.json`: machine-readable planned run matrix.
- `tmp/media_matrix/cases.md`: human-readable matrix and per-case live LLM command.
- `tmp/media_matrix/briefs/*.json`: deterministic `ArticleBrief` inputs for draft generation.
- `tmp/media_matrix/prompts/*.prompt.md`: generated prompts for inspection.

## Matrix cases

The offline scenario currently covers:

| Case | Medium | Style | Primary selectors |
|---|---|---|---|
| `terisuke_note_essay` | note | reflective essay | `note:cor_instrument` |
| `cor_blog_technical_report` | Cor.inc company blog | technical report | `rss:https://cor-jp.com/rss.xml`, `github:Cor-Incorporated/corsweb2024/src/content/blog/ja` |
| `cor_blog_vision_sharing` | Cor.inc company blog | vision sharing | `rss:https://cor-jp.com/rss.xml`, `github:Cor-Incorporated/corsweb2024/src/content/blog/ja` |
| `cloudia_zenn_tutorial` | Zenn | tutorial | `zenn:cloudia` |
| `cloudia_qiita_how_to` | Qiita | practical how-to | `qiita:Cloudia_Cor_Inc` |
| `cor_homepage_section` | homepage | concise product section | `rss:https://cor-jp.com/rss.xml`, `github:Cor-Incorporated/corsweb2024/src/content/blog/ja` |

## Optional live source phase

Run only when live network validation is intended:

```sh
RUN_SOURCE_FETCH_SCENARIO=1 \
SCENARIO_OUTPUT_DIR=tmp/source_fetch_media_matrix \
SOURCE_FETCH_LIMIT=100 \
SOURCE_FETCH_NOTE=note:cor_instrument \
SOURCE_FETCH_ZENN=zenn:cloudia \
SOURCE_FETCH_QIITA=qiita:Cloudia_Cor_Inc \
SOURCE_FETCH_RSS=rss:https://cor-jp.com/rss.xml \
SOURCE_FETCH_GITHUB=github:Cor-Incorporated/corsweb2024/src/content/blog/ja \
go run ./cmd/scenario/source_fetch
```

2026-05-03 live result:

| Selector | Articles | Avg content length | Elapsed seconds | Output |
|---|---:|---:|---:|---|
| `note:cor_instrument` | 20 | 5,177 chars | 8.91 | `tmp/source_fetch_media_matrix/note.json` |
| `zenn:cloudia` | 7 | 4,139 chars | 1.06 | `tmp/source_fetch_media_matrix/zenn.json` |
| `qiita:Cloudia_Cor_Inc` | 12 | 3,274 chars | 0.30 | `tmp/source_fetch_media_matrix/qiita.json` |
| `rss:https://cor-jp.com/rss.xml` | 10 | 69 chars | 0.10 | `tmp/source_fetch_media_matrix/rss.json` |
| `github:Cor-Incorporated/corsweb2024/src/content/blog/ja` | 10 | 4,218 chars | 2.61 | `tmp/source_fetch_media_matrix/github.json` |

## Optional live LLM phase

First generate the offline matrix so the brief files exist:

```sh
SCENARIO_OUTPUT_DIR=tmp/media_matrix go run ./cmd/scenario/media_matrix
```

Then run draft generation for one case at a time with an available local LLM and style artifacts. Example:

```sh
RUN_LOCAL_LLM_SCENARIO=1 \
AUTHOR_PROFILE_PATH=tmp/author_style/profile.json \
WRITING_GUIDE_PATH=tmp/author_style/guide.json \
ARTICLE_BRIEF_PATH=tmp/media_matrix/briefs/cloudia_zenn_tutorial.json \
SCENARIO_OUTPUT_DIR=tmp/media_matrix/live/cloudia_zenn_tutorial \
go run ./cmd/scenario/draft_generation
```

Use the `planned_llm_command` entries in `tmp/media_matrix/matrix.json` or `tmp/media_matrix/cases.md` for the remaining cases, adding `AUTHOR_PROFILE_PATH` and `WRITING_GUIDE_PATH` to point at the style artifacts for the phase under test.

## Comparison table

Compare results across phases and cases with this schema:

| Case | Phase | Medium | Style | Target length | Elapsed seconds | Score | Verification passed | Runes | Output |
|---|---|---|---|---|---:|---:|---|---:|---|
| `terisuke_note_essay` | offline matrix | note | reflective essay | 3000字前後 |  |  | prompt checks only |  | `tmp/media_matrix/prompts/terisuke_note_essay.prompt.md` |
| `terisuke_note_essay` | live draft | note | reflective essay | 3000字前後 |  |  |  |  |  |
| `cor_blog_technical_report` | live draft | company blog | technical report | 2200-2800字 |  |  |  |  |  |
| `cor_blog_vision_sharing` | live draft | company blog | vision sharing | 1600-2200字 |  |  |  |  |  |
| `cloudia_zenn_tutorial` | live draft | Zenn | tutorial | 1800-2400字 |  |  |  |  |  |
| `cloudia_qiita_how_to` | live draft | Qiita | practical how-to | 1400-2000字 |  |  |  |  |  |
| `cor_homepage_section` | live draft | homepage | concise product section | 400-700字 |  |  |  |  |  |

Acceptance criteria for the integrated evaluation:

- Offline matrix passes before any live work.
- Live source phase confirms each selector still returns usable material, with GitHub Markdown preferred over RSS for full Cor.inc blog body text.
- Each live draft records `elapsed_seconds`, style `score`, `passed`, `verification_performed`, `verification_passed`, and `runes` from `cmd/scenario/draft_generation`.
- Failures are grouped by dimension: source selector, persona, medium/output format, style, target length, or verifier result.
