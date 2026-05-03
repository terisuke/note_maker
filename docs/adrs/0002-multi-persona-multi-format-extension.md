# ADR 0002: Multi-Persona, Multi-Format Article Generation

Date: 2026-05-02

## Status

Accepted. Implementation started in Phase B. Extends and partially supersedes [ADR 0001](0001-three-phase-local-article-generation.md). The three-phase workflow (style analysis → interview → draft) is preserved. This ADR adds two orthogonal axes — **author persona** and **output format** — and reshapes UX, persistence, and prompt construction accordingly.

## Context

ADR 0001 produced a working three-phase pipeline for `cor_instrument` (Terisuke) on note.com. Real usage by the project owner exposed three structural gaps:

1. **The owner publishes under multiple identities and to multiple platforms.**
   - Terisuke (本人): [note.com/cor_instrument](https://note.com/cor_instrument), [cor-jp.com/blog](https://cor-jp.com/blog/) — reflective entrepreneur essays mixed with technical experience reports. First person 「僕」「私」, narrative arcs ("～した話", "～てしまった件"), philosophical framing.
   - Cloudia / 宇宙野クラウディア (架空キャラクター, also operated by the owner): [zenn.dev/cloudia](https://zenn.dev/cloudia), [qiita.com/Cloudia_Cor_Inc](https://qiita.com/Cloudia_Cor_Inc) — character-branded technical tutorials. Hakata-ben (博多弁) flavour, exclamation-driven titles ("クラウディア流！…", "AI探検記【前編】〜最強のお助けAIを探せ！〜"), strong code-block density.
   - Treating these as one author profile contaminates the style guide and produces drafts that read as neither voice.

2. **The output format is hard-coded to "note.com paste-ready Markdown article".**
   - `internal/domain/article/draft.go:25-35` requires a `# title` first line and rejects code fences.
   - `internal/application/draft/prompt.go:12-51` injects "Noteにそのまま貼り付けられる日本語Markdown記事" into every system message.
   - `internal/infrastructure/note/fetcher.go:234-246` rejects any host other than `note.com` for source ingestion.
   - HTML homepage sections, Zenn/Qiita Markdown articles (which carry frontmatter), and Astro-blog posts cannot be produced today.

3. **The current UX does not feel like working with a writing partner.**
   - The interview is a forward-only, edit-locked Q&A; users cannot revise an earlier answer.
   - The completed brief is rendered as `JSON.stringify` (`static/js/script.js:178`); accumulated knowledge is not legible.
   - The draft textarea is `readonly` (`static/index.html:153`); there is no per-section regenerate.
   - The full-screen spinner blocks the page during every LLM call; nothing streams.
   - There is no visible history of past style profiles or past sessions; reuse is invisible.

The user's stated comparators are Claude, ChatGPT, and Gemini chat UIs — continuous transcripts with editable history, streaming output, and a side-pinned working artifact.

## Decision

Add two orthogonal first-class concepts to the domain and let the rest of the system depend on them via strategy boundaries.

### Persona

A `Persona` is a named, reusable bundle of:

- one or more `AuthorSource` adapters that supply training material,
- one canonical `WritingStyleGuide` (regenerable when sources change),
- a default `OutputFormat` (overridable per article),
- a default question-set variant for the interview phase,
- optional preset values for the brief (preferred first person, signature opening patterns, recurring themes, anti-patterns).

Two personas ship pre-loaded:

| Persona | Display name | Sources | Default format | Voice notes |
|---|---|---|---|---|
| `terisuke` | てりすけ | `note.com/cor_instrument`, `cor-jp.com/blog/*` | `note_article` | 一人称「僕」/「私」、内省＋実体験ナラティブ、起業・キャリア・AI駆動開発、「～した話」「～てしまった件」 |
| `cloudia` | 宇宙野クラウディア | `zenn.dev/cloudia`, `qiita.com/Cloudia_Cor_Inc` | `zenn_article` | 一人称「クラウディア」/「うち」、博多弁混じり、感嘆符・【前編】等の装飾、AI/JS/Pythonチュートリアル、感情的訴求 (「劇的に」「最強の」) |

Personas are user-extensible. Phase C is not complete with built-in persona selection alone. The `codex/phase-c-persona-history-polish` cut implements custom persona create/list with persistence; custom persona update/delete and richer source management remain future product work. Adding a third persona must not require code changes inside the prompt builder.

### OutputFormat

An `OutputFormat` is a typed strategy with:

- a Markdown/HTML template surface (frontmatter, heading rules, code-fence policy, length envelope),
- a validator (e.g., Zenn requires frontmatter; note rejects code fences; homepage sections forbid `# title`),
- a default question-set extension (technical formats add "対象スタック", "実行環境", "想定読者の前提知識"; narrative formats add "導入エピソード", "結末で読者に届けたい感情"),
- a system-prompt fragment merged into the draft prompt (instead of a single hard-coded fragment),
- an embedded Markdown guide for notation details that are too verbose for the registry fragment.

Formats shipped in v2:

| Format | Validator highlights | Source/target |
|---|---|---|
| `note_article` | Starts with `# `, no frontmatter, paste-safe note Markdown subset; plain code fences only when needed | note.com paste |
| `markdown_blog` | `corsweb2024` Astro frontmatter required (`title`, `description`, `pubDate`, `author`, `category`, `tags`, `lang`, `featured`), Japanese-first, code fences require language | cor-jp.com |
| `zenn_article` | Frontmatter required (`title`, `emoji`, `type`, `topics`, `published`), Zenn extensions (`:::message`, `:::details`, `@[card]`), `diff language` fences | zenn.dev/cloudia |
| `qiita_article` | Frontmatter required (`title`, `tags`), Qiita extensions (`:::note`, HTML `details`), `diff_language` fences, `math` code blocks | qiita.com/Cloudia_Cor_Inc |
| `homepage_section` | HTML output, no `# title`, semantic sectioning, CTA placement guidance | static HTML embed |

The two axes compose: any persona can produce any format. The application layer rejects nonsensical combinations only when the persona explicitly excludes a format (configurable, not hard-coded).

### UX direction

The single-page form becomes a **conversation-first workspace**:

- **Left rail**: persona switcher, project history, "previous style guides" picker.
- **Centre**: chat-style transcript. Past answers are clickable and editable, which forks a new session draft from that point. Deep-dive questions render with the parent question quoted as context.
- **Right rail (artifact panel)**: live brief card and live draft preview, replacing the current JSON dump. Updates as the conversation progresses.
- **Streaming**: SSE for follow-up generation and draft generation. The full-screen spinner is removed.
- **Draft editor**: editable in-place, with "regenerate this section" (selection-based) and "copy to clipboard" buttons for both raw Markdown and rendered HTML output.

The static-JS prototype is preserved as a fallback. The new UX can ship as a progressive overlay (no SPA-framework rewrite required for v2; if desired, a later ADR can introduce one).

### Persistence direction

The flat `data/workflow_store.json` snapshot is replaced by a SQLite-backed store with explicit aggregates:

- `personas` — id, display name, default format, source bundle, current canonical guide id.
- `author_sources` — persona-scoped fetched articles with raw text snapshot for re-derivation.
- `writing_style_guides` — versioned per persona; previous versions retained.
- `projects` — a writing project (e.g., "Q3 cor-jp blog series") that owns multiple articles.
- `articles` — one target deliverable: persona id, output format, brief id, draft history.
- `brief_sessions`, `brief_answers` — unchanged in shape, gain a `parent_answer_id` for fork-on-edit.
- `drafts` — versioned per article with score history.

Acceptance criteria:

- any prior session can be reopened, its accumulated context shown as a transcript, and a new draft regenerated from any point in history;
- custom personas can be created, persisted, selected, and reopened after restart;
- human-readable style-guide and brief artifacts can be edited through explicit product actions without losing raw Markdown/JSON audit data;
- artifact edits create recoverable history or version records rather than silently replacing prior state.

This subsumes Issue [#14](https://github.com/terisuke/note_maker/issues/14) (queryable database). Issue [#14](https://github.com/terisuke/note_maker/issues/14) is kept open as the umbrella tracker; the SQLite migration becomes its acceptance.

### Source acquisition direction

`SourceFetcher` becomes a strategy interface with concrete implementations:

- `NoteFetcher` — existing `note.com` page + RSS (kept).
- `ZennFetcher` — public articles (`https://zenn.dev/{user}/articles/{slug}`) and the user's article list.
- `QiitaFetcher` — public REST (no auth needed for public posts; respect rate limits).
- `GenericRSSFetcher` — for Astro/Jekyll/Hugo blogs that expose RSS (cor-jp.com is included here once its RSS is confirmed).
- `HTMLFetcher` — fallback for arbitrary HTML pages with semantic-content extraction.

Per-fetcher rate-limit and User-Agent policy live alongside each adapter.

## Domain Model Changes

New domain types under `internal/domain`:

- `persona` package
  - `Persona` (id, display_name, sources, default_format, default_questions_extension)
  - `PersonaRegistry` (in-memory + persisted, seeded with `terisuke` and `cloudia`)
- `format` package
  - `OutputFormat` (id, validator, template fragment, default_question_extension)
  - `FormatRegistry`
  - `Validator` interface; per-format implementations (e.g., `NoteValidator`, `ZennValidator`)
- `application/draft/format_guides`
  - embedded Markdown guides for the final draft prompt (`note`, `markdown_blog`, `zenn`, `qiita`, `homepage_section`)
  - guides encode editor notation differences that are too detailed for the short registry fragment
- `domain/article`
  - `Draft` gains a `format_id`; `NewDraft` dispatches to the format's validator instead of hard-coded rules.
- `domain/brief`
  - `ArticleBriefSession` gains `persona_id`, `output_format_id`, `parent_answer_id`.
  - `FixedQuestions` becomes a base list extended by the persona and the format.
- `domain/project` (new)
  - `Project`, `Article` aggregates as defined in the persistence section.

## Application Service Changes

- `AnalyzeAuthorStyleService` accepts a `persona_id` and persists the resulting guide as a new version under that persona; previous versions are preserved.
- `InterviewService` consults the active persona and format to assemble the question list before the first question.
- `GenerateDraftService` resolves the prompt template fragment from the format's strategy, injects the active format's embedded Markdown guide, merges persona-specific tone hints, and runs a lightweight final verification step after the 31B draft is validated.
- New `RegenerateSectionService` accepts a draft id, a section selector (heading anchor or character range), the brief, and the persona+format; returns a candidate replacement for human review.
- New `StreamingDraftService` produces SSE chunks for the draft phase.

## Infrastructure Changes

- `internal/infrastructure/repository/sqlite` — new package implementing every repository interface; the JSON file repository becomes an export/import utility for portability.
- `internal/infrastructure/source/{note,zenn,qiita,rss,html,github}` — per-source fetchers behind a common interface in `internal/domain/source` (or kept under `infrastructure` and bound by interface in `domain/persona`). GitHub-backed Markdown is required for Cor.inc blog because the public RSS feed is a discovery source with summaries, while `corsweb2024/src/content/blog/ja/*.md` is the canonical full-body source.
- `internal/infrastructure/llamacpp` — gains streaming (SSE) support; existing non-streaming path retained for tests.

## API Changes

Additions:

- `GET /api/personas` / `POST /api/personas` — built-in plus custom persona listing and custom persona creation. `PATCH /api/personas/{id}` remains future work.
- `GET /api/formats` — read-only registry of available formats.
- `GET /api/brief-sessions/templates?persona_id=X&format_id=Y` — composed fixed-question template for the selected persona and output format.
- `POST /api/projects` / `GET /api/projects` / `GET /api/projects/{id}` — project management.
- `GET /api/sessions/{id}/transcript` — chat-style transcript including parent links.
- `POST /api/sessions/{id}/answers/{answer_id}/edit` — fork-on-edit for past answers.
- `POST /api/drafts/{id}/regenerate-section` — section-level regenerate.
- `POST /api/author-style/analyze`, `POST /api/drafts` — gain `Accept: text/event-stream` for streaming.

Existing `/api/generate` remains a compatibility facade.

Implemented history/artifact read subset as of the #27/#28 cut:

- `GET /api/history` and `GET /api/workflow/artifacts` — combined reusable workflow artifact index with `style_guides`, `sessions`, and `briefs`.
- `GET /api/author-style` — saved writing style-guide list for picker UIs.
- `GET /api/brief-sessions` — saved interview session summaries.
- `GET /api/briefs` and `GET /api/briefs/{id}` — completed brief artifacts for readable card rendering and reuse.

Implemented project/article/draft read subset as of the Phase C history follow-up:

- `GET /api/projects` and `GET /api/projects/{id}` — SQLite-backed project history and source snapshots when the active store supports them.
- `GET /api/articles/{id}` — one article with brief, current draft, draft versions, and source snapshots.
- `GET /api/drafts/{id}` — one draft with regeneration history.
- `GET /api/workflow/artifacts` includes project, article, and draft summaries when SQLite history is available.

Implemented in the `codex/phase-c-persona-history-polish` cut:

- `POST /api/personas` — stores a user-authored persona after validating required fields, reserved ids, and output format.
- `GET /api/personas` — returns built-in personas followed by persisted custom personas.
- `PATCH /api/author-style/{id}` and `POST /api/author-style/{id}/versions` — store edited style-guide cards as new saved guide versions.
- `PATCH /api/briefs/{id}` — updates the saved brief artifact without rewriting the original session answers.

These endpoints are still narrower than the full Phase C product surface described above. They do not implement custom persona update/delete, rich persona source editing, brief-version tables, or editable project/article/draft/source-snapshot cards.

## Testing Strategy

- Unit tests added per format validator, per persona seed, per fetcher.
- Integration tests for full transcript edit-and-fork flow.
- Scenario tests:
  - `cmd/scenario/persona_terisuke_note` — current behaviour.
  - `cmd/scenario/persona_terisuke_blog` — new, targets cor-jp blog format.
  - `cmd/scenario/persona_cloudia_zenn` — new, validates Zenn frontmatter and code-fence presence.
  - `cmd/scenario/persona_cloudia_qiita` — new.
- HTTP handler tests added for every endpoint in `internal/handlers/workflow.go` (currently uncovered).
- Playwright E2E (extends Issue [#13](https://github.com/terisuke/note_maker/issues/13)) covers persona switch, format switch, edit-and-fork, streaming completion, copy-clipboard, regenerate-section.

## Phased Rollout

The full work is broken into four phases tracked by issues. Each phase is independently shippable and behind a UI toggle until ready.

- **Phase A — Conversation UX (1–2 weeks)**
  - Chat transcript with editable answers, streaming, deep-dive context display, draft editor + per-section regenerate.
  - Issues: [#17](https://github.com/terisuke/note_maker/issues/17), [#18](https://github.com/terisuke/note_maker/issues/18), [#19](https://github.com/terisuke/note_maker/issues/19), [#20](https://github.com/terisuke/note_maker/issues/20).
- **Phase B — Multi-persona, multi-format (2–3 weeks)**
  - Persona registry seeded with `terisuke` and `cloudia`, OutputFormat strategy, source fetcher generalisation, format-aware question sets.
  - Issues: [#21](https://github.com/terisuke/note_maker/issues/21), [#22](https://github.com/terisuke/note_maker/issues/22), [#23](https://github.com/terisuke/note_maker/issues/23), [#24](https://github.com/terisuke/note_maker/issues/24), [#25](https://github.com/terisuke/note_maker/issues/25).
- **Phase C — Memory & history (2 weeks, integrates Issue [#14](https://github.com/terisuke/note_maker/issues/14))**
  - SQLite store with project/article schema, profile/session reuse UI, brief and guide rendered as cards.
  - Issues: [#26](https://github.com/terisuke/note_maker/issues/26), [#27](https://github.com/terisuke/note_maker/issues/27), [#28](https://github.com/terisuke/note_maker/issues/28).
- **Phase D — Quality & ops**
  - Handler test coverage, Issue [#11](https://github.com/terisuke/note_maker/issues/11) (style threshold), Issue [#13](https://github.com/terisuke/note_maker/issues/13) (Playwright), Issue [#15](https://github.com/terisuke/note_maker/issues/15) (desktop packaging) follow-up.
  - Issue: [#29](https://github.com/terisuke/note_maker/issues/29).

Original recommended order was A → C → B → D. Implementation intentionally pulled the minimum B work forward because source acquisition, format validation, persona seeds, and question templates were required before a realistic cross-media evaluation could be defined. With Phases A and B now implemented, the 2026-05-03 implementation cut lands C1, D1, and the #57 runner foundation in parallel. The next order is C2/C3, one bounded Evo X2 runner validation, then the full Evo X2 media-matrix evaluation under #40.

Current implementation status as of 2026-05-03:

- ADR 0001's strict Terisuke style threshold work is complete ([#11](https://github.com/terisuke/note_maker/issues/11)).
- Phase B1 is complete ahead of the original order: `Persona` and `OutputFormat` concepts, prompt dispatch, and format validators are in place ([#21](https://github.com/terisuke/note_maker/issues/21)). The remaining Phase B work stays deferred until after Phase A/C foundations.
- Evo X2 Ollama is the primary heavy-inference runtime through the Tailnet OpenAI-compatible API (`http://evo-x2.tailb30e58.ts.net/v1`). The runtime fallback chain is Evo X2 Ollama → Evo X2 llama.cpp (`/llama/v1`) → workstation-local llama.cpp. SSH tunnel access is an explicit developer diagnostic only.
- Phase model defaults are intentionally split: lightweight `gemma4:e2b` for source/style summarization, `qwen3.6:27b` for deeper interview questions, and `gemma4:31b` for final Japanese draft generation. This is an operational default, not a hard domain rule; users can override it per phase.
- Final verification uses lightweight Gemma by default (`gemma4:latest`, currently the Evo X2 E4B-class Ollama model) to check brief coverage, style consistency, output-format notation, and unsupported factual assertions before the UI presents the final draft ([#47](https://github.com/terisuke/note_maker/issues/47)).
- Runtime validation showed that Tailnet inference can take 20+ minutes and still miss quality gates because of generation variance. Therefore, Phase A started with streaming and cancellation ([#18](https://github.com/terisuke/note_maker/issues/18)) before the broader transcript rewrite ([#17](https://github.com/terisuke/note_maker/issues/17)). Primary-runtime quality stabilization is tracked separately in [#40](https://github.com/terisuke/note_maker/issues/40).
- Phase A2 is implemented and merged: `llamacpp.Client.GenerateStream`, streaming follow-up/draft service paths, `Accept: text/event-stream` handlers, browser Cancel controls, heartbeat events, and final runtime metrics ([#18](https://github.com/terisuke/note_maker/issues/18)).
- Phase A1 is implemented and merged: the interview surface now renders a chat-style transcript, answer bubbles can be edited inline, and edits create child sessions via fork-on-edit while retaining `parent_session_id` lineage ([#17](https://github.com/terisuke/note_maker/issues/17)).
- Phase A4 is implemented and merged: follow-up prompts include parent question, parent answer, and active style guide context; rule-based fallback questions use the same quoted parent-answer prefix; the transcript labels the parent answer as the deep-dive rationale ([#20](https://github.com/terisuke/note_maker/issues/20)). Validation is recorded in [Issue 20 deep-dive rationale validation](../validation/issue-20-deep-dive-rationale-2026-05-02.md).
- Phase A3 is implemented in code: the generated Markdown textarea is editable, preview rendering live-syncs through `marked`, both preview and Markdown tabs can copy content, and `POST /api/drafts/{id}/regenerate-section` rewrites exactly one `## ` subtree while preserving the rest of the draft byte-for-byte ([#19](https://github.com/terisuke/note_maker/issues/19)). Validation is recorded in [Issue 19 section regeneration validation](../validation/issue-19-section-regeneration-2026-05-02.md).
- Phase B2/B3/B4 are implemented: historical source acquisition works for note, Zenn, Qiita, Cor RSS, and Cor GitHub Markdown; all five formats have prompt fragments, embedded guides, and validators; `terisuke` and `cloudia` ship as distinct seed personas. Validation is recorded in [Issue 22 source fetcher validation](../validation/issue-22-source-fetchers-2026-05-02.md) and [Issue 23/24 format and persona seed validation](../validation/issue-23-24-format-persona-seed-2026-05-02.md).
- Phase B5 is implemented: fixed interview questions are composed server-side by `persona_id × output_format_id`, Cloudia technical modes include extra viewpoint/context prompts, the frontend reads `GET /api/brief-sessions/templates`, and `cmd/scenario/media_matrix` produces a six-case cross-media evaluation matrix for note, Cor blog, Zenn, Qiita, and homepage output ([#25](https://github.com/terisuke/note_maker/issues/25)).
- Phase C1 is implemented and merged: `internal/infrastructure/repository/sqlite` adds migrations and storage for author styles, sessions, briefs, projects, articles, source snapshots, draft versions, final verification, and section-regeneration versions. The JSON store remains the compatibility path, while storage mode can now be inspected and switched from the web settings UI unless environment variables lock it ([#26](https://github.com/terisuke/note_maker/issues/26), [#61](https://github.com/terisuke/note_maker/issues/61)).
- Phase C2/C3 has an implemented first product cut for workflow history and readable artifacts ([#27](https://github.com/terisuke/note_maker/issues/27), [#28](https://github.com/terisuke/note_maker/issues/28)): the web app now exposes reusable history through `GET /api/history` and `GET /api/workflow/artifacts`, plus focused read endpoints `GET /api/author-style`, `GET /api/brief-sessions`, `GET /api/briefs`, and `GET /api/briefs/{id}`. The memory and SQLite stores both expose `ListAuthorStyles`, `ListSessions`, and `ListBriefs`; SQLite also gained project/article/draft/source-snapshot history methods for the richer #26 schema. The UI adds `履歴から再開`, saved style-guide/session/project/article/draft pickers, human-readable style-guide cards, article-brief cards, project/article cards, current-draft cards, draft-version cards, and source-snapshot cards while keeping raw Markdown/JSON details available. Validation is recorded in [Issue 27/28 history and artifact UI/API validation](../validation/issue-27-28-history-artifacts-2026-05-03.md).
- The #13 follow-up has real browser E2E coverage: Python `pytest` plus Playwright starts the real Go server on a free localhost port, stubs application APIs in the browser, and covers model config persistence, custom question CRUD/reset, legacy localStorage migration, persona/format switching, interview start payloads, history opening/readable cards, edit/fork, streaming/cancel, and section regeneration. Validation is recorded in [Issue #13 Browser E2E Validation](../validation/issue-13-browser-e2e-2026-05-03.md).
- A Phase C project/article/draft history follow-up now builds on the #26 SQLite schema: `GET /api/workflow/artifacts` includes project/article/draft summaries when SQLite is active, focused read routes expose project/article/draft details, and the history UI renders project, article, brief, current draft, draft versions, and source snapshot cards. Handler/server/static tests and browser E2E are green for this surface.
- The `codex/phase-c-persona-history-polish` cut implements custom persona create/list and editable brief/style card persistence. Memory and SQLite stores persist custom personas, SQLite restores them after reopen, the UI can add a persona and select/reload it, style-card edits save as a new guide version, and brief-card edits update the saved brief artifact. Validation is recorded in [Phase C persona/history/card polish validation](../validation/phase-c-persona-history-card-polish-2026-05-03.md).
- Phase D1 is implemented and merged: handler tests now cover template selection, edit/fork errors, SSE follow-up and draft paths, completed-session draft fallback, regenerate-section context recovery, Analyze/Generate compatibility handlers, and SQLite driver selection. `go test ./internal/handlers -cover` reports 80%+ statement coverage ([#29](https://github.com/terisuke/note_maker/issues/29)).
- Runtime runner support is implemented and merged: `cmd/scenario/live_media_matrix` reads the offline matrix, emits planned aggregate JSON/Markdown by default, and executes live Evo X2 draft runs only when `RUN_LIVE_MEDIA_MATRIX=1` or `make scenario-media-matrix-live` is used ([#57](https://github.com/terisuke/note_maker/issues/57)).
- The 2026-05-03 browser 500 analysis showed an implementation drift: plain web-app startup still defaulted to workstation-local `127.0.0.1:8081`, while this ADR requires Evo X2 Tailnet as primary. Issue [#63](https://github.com/terisuke/note_maker/issues/63) restores the default order to Evo X2 Ollama over Tailnet → Evo X2 llama.cpp → workstation-local llama.cpp and makes the UI show the actual endpoint/model reported by SSE.
- The interview question set was simplified before the next Evo X2 run ([#66](https://github.com/terisuke/note_maker/issues/66)): broad editorial questions are now split into smaller plain-Japanese prompts, medium-specific prompts cover note/Zenn/Qiita/Cor blog needs, and optional questions can be advanced as `未定`. Validation is recorded in [Issue 66 plain brief questions validation](../validation/issue-66-plain-brief-questions-2026-05-03.md).
- Style analysis is now persona/format-aware ([#68](https://github.com/terisuke/note_maker/issues/68)): the web UI shows a general `文体ソース` selector instead of `Noteユーザー名`, defaults it to note/Zenn/Qiita/Cor GitHub Markdown based on the selected mode, and makes persona presets include output-format notes. Validation is recorded in [Issue 68 media-aware style source validation](../validation/issue-68-media-aware-style-source-2026-05-03.md).
- The 2026-05-03 full Tailnet Evo X2 media-matrix run proved that the runtime path works but also proved that the current scenario is not sufficient as an interview-template acceptance test: only `terisuke_note_essay` passed, Cor blog failed on assistant preamble leakage, Zenn/Qiita failed on cross-format notation leakage, and homepage failed long-form gates despite being a short HTML section. Runtime stabilization is therefore decomposed under epic [#40](https://github.com/terisuke/note_maker/issues/40) into [#70](https://github.com/terisuke/note_maker/issues/70) template/brief scenario coverage, [#71](https://github.com/terisuke/note_maker/issues/71) failed draft artifacts, [#72](https://github.com/terisuke/note_maker/issues/72) bounded format repair, [#73](https://github.com/terisuke/note_maker/issues/73) output-format-specific gates, and [#74](https://github.com/terisuke/note_maker/issues/74) staged Evo X2 reruns.
- The follow-up #74 validation completed the current publishing-target acceptance scope. On 2026-05-03, the full Tailnet Evo X2 matrix passed all five article targets: note, two Cor.inc company-blog modes, Zenn, and Qiita. The run used Evo X2 Ollama primary at `http://evo-x2.tailb30e58.ts.net/v1`, `gemma4:31b` for draft generation, and `gemma4:latest` for lightweight final verification. Results are recorded in [Issue #74 staged Evo X2 rerun](../validation/issue-74-staged-evo-x2-rerun-2026-05-03.md) and `tmp/media_matrix/live/aggregate.{json,md}`. Summary: `5/5` passed, average `122.01s`, average style score `86.0`, average `3742` runes, all final verification and structural gates passed.
- The #74 pass required additional hardening that is now part of the architecture: frontmatter preamble/fence normalization, recoverable frontmatter repair, bounded repair/revision/final-verification calls, case-specific style profiles, final verifier format-guide grounding, best-attempt selection across retries, and explicit quality-gate aggregate output.
- The #70-#73 prerequisite slice is now in place for #74: interview-template coverage exists, failed draft artifacts are preserved, format-only failures can be repaired once without relaxing validators, and scenario gates are split by output format. The first staged #74 Tailnet rerun used `cloudia_zenn_tutorial` and moved past the original failure class but failed strict style (`73.6 / 82.0`). The current cut fixes the reliability gap behind that score by generating case-specific style profile/guide artifacts, passing them into live runs, rejecting profile/guide/brief mismatches, making final verification block `scenario_passed`, enforcing structural signals, and exposing UI `quality_gate` details. The bounded reruns now pass both Cloudia technical proofs: `cloudia_zenn_tutorial` scored `88.1 / 82.0` with `5040 / 1800` runes, and `cloudia_qiita_how_to` scored `83.7 / 82.0` with `3318 / 1400` runes. Both used the Tailnet endpoint `http://evo-x2.tailb30e58.ts.net/v1` and passed lightweight verification. Validation is recorded in [Issue 74 staged Evo X2 rerun](../validation/issue-74-staged-evo-x2-rerun-2026-05-03.md).

Near-term execution order:

1. Close [#74](https://github.com/terisuke/note_maker/issues/74) and [#40](https://github.com/terisuke/note_maker/issues/40) for the current note/Qiita/Zenn/Cor blog publishing-target scope after linking the final `5/5` aggregate artifacts. Homepage remains a separate short-format check.
2. Treat [#13](https://github.com/terisuke/note_maker/issues/13) as covered by the browser E2E validation cut; do not move Phase C product gaps back into browser-coverage scope.
3. Keep [#14](https://github.com/terisuke/note_maker/issues/14) open for broader queryable product memory and split custom persona update/delete or brief-version history if those become required beyond this cut.
4. Keep fallback-quality and runtime packaging follow-up ([#36](https://github.com/terisuke/note_maker/issues/36), [#45](https://github.com/terisuke/note_maker/issues/45), [#15](https://github.com/terisuke/note_maker/issues/15)) outside the #40 closure gate.

## Tracked issues

Filed 2026-05-02 as part of the PR that introduced this ADR.

- A1 — [#17](https://github.com/terisuke/note_maker/issues/17) Refactor interview UI to chat-style transcript with editable past answers
- A2 — [#18](https://github.com/terisuke/note_maker/issues/18) Stream LLM responses via SSE for follow-up and draft generation. Promoted to the immediate next implementation target because Tailnet Evo X2 generation is too slow for a spinner-only UX.
- A3 — [#19](https://github.com/terisuke/note_maker/issues/19) Editable draft Markdown + per-section regenerate API
- A4 — [#20](https://github.com/terisuke/note_maker/issues/20) Surface deep-dive question rationale in prompt and UI
- B1 — [#21](https://github.com/terisuke/note_maker/issues/21) Introduce Persona and OutputFormat domain concepts (registry + strategy). Implemented by the first Phase B PR: `internal/domain/persona`, `internal/domain/format`, API selectors, prompt dispatch, and format-aware draft validation.
- B2 — [#22](https://github.com/terisuke/note_maker/issues/22) Generalize SourceFetcher beyond note.com (Zenn, Qiita, RSS, HTML)
- B3 — [#23](https://github.com/terisuke/note_maker/issues/23) Format-specific prompt templates and draft validators (note / markdown_blog / zenn / qiita / homepage_section). Implemented: validators, embedded format guides, prompt injection, and deterministic scenario samples exist.
- B4 — [#24](https://github.com/terisuke/note_maker/issues/24) Seed persona library with `terisuke` and `cloudia` profiles. Implemented for the built-in registry: seeds include sources, default formats, and voice notes; live source re-analysis remains under [#22](https://github.com/terisuke/note_maker/issues/22).
- B5 — [#25](https://github.com/terisuke/note_maker/issues/25) Format- and persona-aware fixed question sets
- C1 — [#26](https://github.com/terisuke/note_maker/issues/26) Replace JSON store with SQLite-backed schema (extends [#14](https://github.com/terisuke/note_maker/issues/14)) — implemented in the current cut as an opt-in SQLite workflow store.
- C2 — [#27](https://github.com/terisuke/note_maker/issues/27) Persona / past-session picker UI — implemented for built-in and custom persona history reuse through `履歴から再開`, backed by `GET /api/workflow/artifacts`, `GET /api/author-style`, `GET /api/brief-sessions`, SQLite project/article/draft read routes, and `POST /api/personas`. Custom persona update/delete remains follow-up work.
- C3 — [#28](https://github.com/terisuke/note_maker/issues/28) Render brief and style guide as human-readable cards — implemented for style-guide and article-brief card edit/save flows, plus read-only project, article, draft-version, current-draft, and source-snapshot cards, with raw Markdown/JSON details preserved. Brief edits persist the saved artifact; style edits create new saved guide versions.
- D1 — [#29](https://github.com/terisuke/note_maker/issues/29) HTTP handler tests for `internal/handlers/workflow.go` — implemented in the current cut with 80.0% handler package coverage.
- Runtime runner — [#57](https://github.com/terisuke/note_maker/issues/57) Add live LLM media-matrix runner and aggregate evaluator, feeding [#40](https://github.com/terisuke/note_maker/issues/40) — implemented in the current cut.
- Runtime stabilization epic — [#40](https://github.com/terisuke/note_maker/issues/40) Stabilize Tailnet Evo X2 draft quality and runtime metrics. #70-#73 provide the prerequisite validation and diagnostics. [#74](https://github.com/terisuke/note_maker/issues/74) has passed the bounded Cloudia/Zenn and Cloudia/Qiita proofs plus the final `5/5` publishing-target matrix.

## Consequences

Positive:

- The product can serve both Terisuke (本人) and Cloudia (キャラクター) without contaminating either voice.
- Adding a new platform (e.g., Substack, dev.to) becomes one fetcher + one format, not a fork.
- Memory becomes legible: persona library, project history, and brief cards make accumulated context visible.
- The conversation feel matches the Claude/ChatGPT/Gemini comparator class.

Tradeoffs:

- Domain surface grows. The win is mitigated by registries and strategy boundaries instead of conditionals.
- SQLite migration is unavoidable; the JSON file becomes export-only.
- Streaming requires keeping non-streaming paths for tests; carries minor duplication.

## Rejected alternatives

- **One persona with `style_variant` flag.** Rejected because the two voices share *no* tonal substrate; mixing them in one guide demonstrably degrades both. Two distinct guides are operationally cleaner than one guide with branches.
- **Output format as a free-text instruction in the brief.** Rejected because validators must be code-enforced (Zenn frontmatter, note `# title`, HTML semantics). Free text gives no validator handle.
- **Single-page rewrite to React/Vue first.** Rejected as premature; the UX wins (chat transcript, streaming, editable draft) are achievable as progressive enhancements. A framework rewrite is worth a later ADR if profiling shows the static prototype slows iteration.
- **Skip persona model and let the user paste a custom style guide.** Rejected because re-deriving the guide from real articles is the strongest signal we have; manual paste defeats the style-comparator scoring.
