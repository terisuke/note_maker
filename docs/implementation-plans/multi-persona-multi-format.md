# Multi-persona, multi-format implementation plan

Date: 2026-05-02

This plan implements [ADR 0002](../adrs/0002-multi-persona-multi-format-extension.md). It extends [ADR 0001](../adrs/0001-three-phase-local-article-generation.md) without throwing away its three-phase pipeline.

## Goal

Take Note Maker from "single-persona, single-format note.com generator" to "writing partner that supports the project owner's two operating identities (てりすけ / クラウディア) and at least four publishing targets (note / cor-jp blog / Zenn / Qiita), with a conversation-first UX and durable memory."

## Non-goals (this plan)

- Cloud or multi-tenant deployment. The system stays local-first and single-user.
- Replacing the three-phase pipeline. Phases stay; orchestration is reshaped.
- Native desktop packaging. Issue [#15](https://github.com/terisuke/note_maker/issues/15) remains separate.
- Additional personas beyond the two seeded ones. The registry must support more, but only two ship.

## Phasing

The four phases below match ADR 0002. Each is independently shippable.

| Phase | Goal | Dominant cost | Issues |
|---|---|---|---|
| A | Conversation UX upgrade | Frontend + handler streaming | [#17](https://github.com/terisuke/note_maker/issues/17), [#18](https://github.com/terisuke/note_maker/issues/18), [#19](https://github.com/terisuke/note_maker/issues/19), [#20](https://github.com/terisuke/note_maker/issues/20) |
| B | Persona + Format axes | Domain refactor + new fetchers | [#21](https://github.com/terisuke/note_maker/issues/21), [#22](https://github.com/terisuke/note_maker/issues/22), [#23](https://github.com/terisuke/note_maker/issues/23), [#24](https://github.com/terisuke/note_maker/issues/24), [#25](https://github.com/terisuke/note_maker/issues/25) |
| C | Memory: SQLite + history UI | Persistence rewrite, extends [#14](https://github.com/terisuke/note_maker/issues/14) | [#26](https://github.com/terisuke/note_maker/issues/26), [#27](https://github.com/terisuke/note_maker/issues/27), [#28](https://github.com/terisuke/note_maker/issues/28) |
| D | Quality & coverage | Tests + thresholds | [#29](https://github.com/terisuke/note_maker/issues/29) (rolls up [#11](https://github.com/terisuke/note_maker/issues/11), [#13](https://github.com/terisuke/note_maker/issues/13)) |

Recommended order: **A → C → B → D**. Phase B benefits from durable storage (Phase C) being in place first, otherwise the JSON store becomes a temporary obstacle for the persona registry.

Current status after the 2026-05-02 merges:

- [#11](https://github.com/terisuke/note_maker/issues/11) strict Terisuke style tuning is closed.
- [#21](https://github.com/terisuke/note_maker/issues/21) B1 landed early: persona/format domain concepts, prompt dispatch, selectors, and validators exist.
- [#38](https://github.com/terisuke/note_maker/issues/38) Tailnet OpenAI-compatible API is now the Evo X2 primary path. SSH tunnel access is diagnostic-only.
- [#36](https://github.com/terisuke/note_maker/issues/36) remains open for local llama.cpp fallback quality; it does not block Phase A work.
- [#40](https://github.com/terisuke/note_maker/issues/40) tracks primary Tailnet Evo X2 quality and runtime-metric stabilization.
- A Tailnet full-workflow run reached the correct Evo X2 endpoint but took `1396.80s` and failed the quality gate (`score=82.0`, `2653` runes, `first_person=49`). This is the practical reason to start with streaming/cancellation rather than more prompt-only tuning.

Near-term implementation cut:

| Order | Issue | Why now | Done when |
|---|---|---|---|
| 1 | [#18](https://github.com/terisuke/note_maker/issues/18) | Long Tailnet inference needs visible progress, heartbeat, and cancellation before more UX is layered on top. | Code streams follow-up/draft output, can be cancelled, and reports endpoint/model/elapsed time; final closure waits for Tailnet Evo X2 validation. |
| 2 | [#17](https://github.com/terisuke/note_maker/issues/17) | The transcript can then use the streaming primitives instead of another spinner path. | Answers render as editable bubbles and edits fork the in-memory session. |
| 3 | [#20](https://github.com/terisuke/note_maker/issues/20) | Deep-dive rationale belongs in the transcript once the transcript exists. | Every follow-up references the parent answer in prompt and UI. |
| 4 | [#19](https://github.com/terisuke/note_maker/issues/19) | Section regeneration is useful only after draft output can stream and be cancelled. | Markdown is editable, preview syncs, and section regeneration replaces only one subtree. |
| 5 | [#26](https://github.com/terisuke/note_maker/issues/26) | Forked answers and draft versions need durable storage before broader persona library work. | SQLite stores sessions, answers, guides, articles, and draft versions. |

## Phase A — Conversation UX

### A1 — Chat-style transcript with editable past answers

- Replace the bounded `question-log` div (currently `max-height: 340px`) with a full-height scrolling transcript.
- Each `BriefAnswer` rendered as a clickable bubble; clicking opens an inline edit affordance.
- Editing an answer creates a new session that forks at that answer (server: `POST /api/sessions/{id}/answers/{answer_id}/edit`, returns the new `session_id`). Original session is retained for comparison.
- Visual: question bubbles left-aligned, answers right-aligned; deep-dive questions visually nested under their parent fixed question.
- Keyboard: `↑` recalls the previous unsubmitted answer; `Cmd/Ctrl+Enter` submits.

Acceptance:

- Editing answer #2 in a 5-answer session produces a new session whose answers list is `[1, 2', …]`.
- Original session is reachable from the new session via `parent_session_id`.

### A2 — Stream LLM responses via SSE

Implementation status: code path implemented on `codex/issue-18-sse-streaming`; validate against real Tailnet Evo X2 before closing the issue.

- Add SSE support to `internal/infrastructure/llamacpp/client.go` (OpenAI-compatible `stream: true`).
- Wire streaming through the application services for `follow-up generation` and `draft generation`. Style analysis can stay non-streaming (single short call).
- Frontend: replace global spinner with token-by-token append into the transcript (for follow-up) or into the draft preview (for draft).
- Tailnet runtime: stream status events before first token (`endpoint`, `model`, `phase`, `started_at`), heartbeat events every 10 seconds, and final metrics (`elapsed_ms`, `runes`, `score` when available).
- Cancellation: closing the browser request or pressing Cancel must cancel the server context and the upstream OpenAI-compatible request.
- Failure mode: if the stream ends because the model times out or quality validation fails, keep the partial draft and surface the evaluation instead of losing the work.

Acceptance:

- A 3000-character draft visibly streams; a status event appears immediately and content chunks append incrementally once the model responds.
- Network tab shows `text/event-stream` content type with incremental chunks.
- Cancelling during a Tailnet Evo X2 run stops the server-side request and leaves the UI in a recoverable state.
- Scenario/validation output records base URL, model, elapsed time, draft length, and score.

### A3 — Editable draft + per-section regenerate

- Make `<textarea id="markdown-output">` writable. Live-sync into the rendered preview via `marked`.
- Add a "Regenerate this section" button that activates when the cursor is inside a `## ` section. Sends the section heading + the brief + the persona/format to a new `POST /api/drafts/{id}/regenerate-section` endpoint.
- Add a "Copy" button to the preview tab as well as the markdown tab.

Acceptance:

- Editing the textarea immediately updates the preview.
- Regenerating section "## 実装" replaces only that subtree of the Markdown; the other sections remain byte-identical.

### A4 — Deep-dive rationale surfaced

- Follow-up prompt (`internal/handlers/workflow.go:437-453`) gains the parent question text and the latest answer summary, plus the active style guide as context.
- UI renders deep-dive bubbles with a quoted excerpt from the parent answer ("「〇〇」というご回答を踏まえて…").
- Fallback (rule-based) text uses the same prefix to keep tone consistent.

Acceptance:

- Every deep-dive bubble in the transcript visibly references its parent answer.
- Falling back to the rule-based path (LLM stub) still produces a contextual prefix.

## Phase B — Persona + OutputFormat

### B1 — Persona and OutputFormat domain concepts

New packages:

- `internal/domain/persona`
  - types: `Persona`, `PersonaID`, `PersonaSeed`
  - registry: in-memory + SQLite-backed once Phase C lands
- `internal/domain/format`
  - types: `OutputFormat`, `FormatID`, `Validator`
  - registry: same dual-mode

Hooks:

- `domain/article.NewDraft` accepts `format_id`, dispatches validation to the format's `Validator`.
- `domain/brief.ArticleBriefSession` gains `persona_id`, `output_format_id`.
- `application/draft.BuildPrompt` becomes `BuildPrompt(persona, format, guide, brief)`; the function pulls a per-format system fragment instead of the current hard-coded string.

Acceptance:

- `go test ./...` passes with all five formats registered.
- A unit test exists per format validator (note / markdown_blog / zenn / qiita / homepage_section).

### B2 — SourceFetcher generalisation

New package `internal/domain/source` with:

```go
type Fetcher interface {
    FetchProfile(ctx context.Context, ref Ref) (*ProfileSnapshot, error)
    FetchArticle(ctx context.Context, ref Ref) (*ArticleSnapshot, error)
    FetchList(ctx context.Context, ref Ref, limit int) ([]ArticleSnapshot, error)
}
```

Concrete implementations under `internal/infrastructure/source/`:

- `note/` — extracted from existing `internal/infrastructure/note/fetcher.go`.
- `zenn/` — public articles + `/{user}/feed`. Robots.txt + 1 req/sec.
- `qiita/` — public REST API (no auth needed for read-only public posts) + HTML fallback.
- `rss/` — generic RSS reader for Astro/Jekyll/Hugo blogs.
- `html/` — generic semantic-content extractor (last resort).

Each fetcher carries its own User-Agent string and rate-limit policy.

Acceptance:

- Scenario test fetches one article from each of {note, zenn, qiita, rss} and produces `tmp/source_fetch/{name}.json`.
- The note.com host check moves out of the application service into the `note` fetcher only; other hosts route to other fetchers.

### B3 — Format-specific prompt templates and validators

Files:

- `internal/domain/format/format.go`
- `internal/application/draft/format_guides/note.md`
- `internal/application/draft/format_guides/markdown_blog.md`
- `internal/application/draft/format_guides/zenn.md`
- `internal/application/draft/format_guides/qiita.md`
- `internal/application/draft/format_guides/homepage_section.md`
- `internal/application/draft/format_guides.go`

Validators (in `internal/domain/format`):

- `NoteValidator` — `# ` first line, no frontmatter, no Zenn/Qiita-specific extended Markdown; plain fences allowed only when needed.
- `MarkdownBlogValidator` — `corsweb2024` Astro frontmatter required, `lang: ja`, category limited to `ai | engineering | founder | lab`, `# ` or `## ` first heading, code fences require language.
- `ZennValidator` — frontmatter required (`title`, `emoji`, `type ∈ {tech, idea}`, `topics: [...]`, `published: bool`), rejects Qiita `:::note` and `diff_language`.
- `QiitaValidator` — frontmatter required (`title`, `tags: [...]`), rejects Zenn `:::message`, `:::details`, `@[card]`, and `diff language`.
- `HomepageSectionValidator` — output is HTML, no `# `, requires at least one `<h2>` and one `<p>`, optional CTA `<a>` block.

Acceptance:

- Each validator has a positive and negative unit test.
- Each registered format has an embedded Markdown guide injected into the final draft prompt.
- Generating the same brief under different formats produces visibly different drafts: Zenn has frontmatter + many code fences; note has narrative paragraphs and ですます調; homepage_section is HTML with no `# `.

### B4 — Persona library seed

Seed file: `internal/domain/persona/seed.go` (or YAML under `data/personas/`).

```yaml
- id: terisuke
  display_name: てりすけ
  default_format: note_article
  sources:
    - kind: note
      ref: cor_instrument
    - kind: rss
      ref: https://cor-jp.com/blog/rss.xml   # confirm URL during implementation
  voice_notes:
    first_person: 僕
    tone: 内省＋実体験ナラティブ
    title_patterns: ["～した話", "～てしまった件", "【〇〇】"]
    anti_patterns: ["過度に断定的な表現", "クラウディア風の感嘆符連打"]

- id: cloudia
  display_name: 宇宙野クラウディア
  default_format: zenn_article
  sources:
    - kind: zenn
      ref: cloudia
    - kind: qiita
      ref: Cloudia_Cor_Inc
  voice_notes:
    first_person: クラウディア
    tone: 博多弁混じりキャラクター解説
    title_patterns: ["クラウディア流！…", "～探検記【前編】", "～を探せ！"]
    anti_patterns: ["てりすけ風の内省的書き出し", "断定的な経営論"]
```

Acceptance:

- A scenario command runs `analyze` for both personas and writes two distinct `WritingStyleGuide` files to `tmp/personas/`.
- Cross-style score: rebuilding Cloudia's guide from Terisuke's articles produces lower style-similarity than Cloudia's own articles (sanity check that the personas are actually distinct).

### B5 — Format- and persona-aware fixed questions

The fixed nine questions in `static/js/script.js` are extracted server-side into `internal/domain/brief/questions/`:

- `base.go` — questions common to all (theme, reader, exclusions).
- `narrative.go` — extension for `note_article` + `markdown_blog` (opening_episode, personal_context, expected_reader_action).
- `technical.go` — extension for `zenn_article` + `qiita_article` (target_stack, runtime_env, prerequisite_knowledge, code_examples, references).
- `homepage.go` — extension for `homepage_section` (target_conversion, primary_cta, brand_voice).

`InterviewService` composes `base + persona_extension + format_extension` at session start.

Acceptance:

- Starting a `cloudia × zenn_article` session asks technical questions including `target_stack`.
- Starting a `terisuke × note_article` session is byte-identical to the current question set.
- Custom questions added via the existing config UI are appended after the composed list.

## Phase C — Memory & history

### C1 — SQLite store (extends Issue [#14](https://github.com/terisuke/note_maker/issues/14))

- New package `internal/infrastructure/repository/sqlite` using `modernc.org/sqlite` (pure Go, no CGO) or `mattn/go-sqlite3` if CGO is acceptable.
- Schema migrations under `internal/infrastructure/repository/sqlite/migrations/` numbered `0001_*.sql`, applied at boot via a tiny in-process migrator.
- Tables (minimum): `personas`, `author_sources`, `writing_style_guides` (versioned), `projects`, `articles`, `brief_sessions`, `brief_answers` (with `parent_answer_id`), `drafts` (versioned).
- The existing JSON file becomes an export/import utility. On first boot, if the JSON file exists, it is imported.
- Default DB path: `data/note_maker.db` (gitignored).

Acceptance:

- All repository interfaces have SQLite implementations. Existing in-memory implementations remain for tests.
- `go test ./...` passes against both implementations.
- Re-opening the app after a restart shows past projects, sessions, and drafts.

### C2 — Persona / past-session picker UI

- Top bar adds a persona switcher (`てりすけ` / `宇宙野クラウディア` / `+ Add persona`).
- Left rail lists projects; each project expands into its articles.
- Selecting a past article rehydrates the right-side artifact panel (brief card + draft history).
- "Start new article from this persona" reuses the current canonical guide for that persona.

Acceptance:

- Switching personas swaps the question set and the default format on the next session start.
- Re-opening yesterday's session restores the transcript exactly.

### C3 — Brief and guide as human-readable cards

- Replace `JSON.stringify` brief preview with a card stack: theme, reader, must-include (chips), exclusions (chips), opening episode (quote block), tone (badge), expected reader action (callout).
- Replace `<pre>` guide preview with a structured guide card: first-person badge, recurring themes (chips), opening pattern, conclusion pattern, anti-patterns (warning callouts).
- Both cards have an "Edit" affordance that updates the underlying brief / guide.

Acceptance:

- Visual: no JSON visible to the end user during normal operation.
- Editing a brief field updates the in-memory session and persists to SQLite.

## Phase D — Quality & ops

### D1 — Handler tests

`internal/handlers/workflow.go` is 467 lines and currently has zero direct test coverage. This is the highest-leverage gap because every Phase A / B / C change touches it.

- Add `internal/handlers/workflow_test.go` with table-driven tests for each handler: `AnalyzeAuthorStyleHandler`, `CreateBriefSessionHandler`, `AnswerBriefSessionHandler`, `GenerateDraftHandler`, plus the new endpoints introduced by Phases A and B.
- Use injected fakes for the application services (the existing pattern from `generate_test.go`).
- Target ≥ 80 % line coverage for `workflow.go`.

Issue [#11](https://github.com/terisuke/note_maker/issues/11) (style threshold tuning) and Issue [#13](https://github.com/terisuke/note_maker/issues/13) (Playwright E2E) are tracked separately but their acceptance criteria are folded into Phase D's exit gate.

Runtime validation treats Evo X2 Ollama's OpenAI-compatible API over Tailscale VPN/MagicDNS as the primary heavy-inference path. SSH tunnels are explicit developer diagnostics only. The fallback chain is Evo X2 Ollama → Evo X2 llama.cpp → workstation-local llama.cpp. Scenario reports must include base URL, model, elapsed time, score, and draft length to prevent accidental local-runtime validation. The 2026-05-02 validation passed on Evo X2 and found local fallback quality/model-compatibility gaps; fallback hardening is tracked in Issue [#36](https://github.com/terisuke/note_maker/issues/36). Future llama.cpp model swap orchestration is tracked in Issue [#45](https://github.com/terisuke/note_maker/issues/45).

## Risk register

| Risk | Mitigation |
|---|---|
| Zenn / Qiita ToS or rate limits restrict scraping | Use public, robots-respecting endpoints first; cache aggressively; document UA string per fetcher. |
| Cloudia voice cross-contaminates Terisuke's guide if persistence schema is wrong | Persona id is a NOT NULL FK on every guide and answer. Tests assert that switching personas does not leak guide ids. |
| SQLite migration breaks the user's current JSON store | First-boot importer + retain the JSON file for fallback for two releases. |
| Streaming complicates handler tests | Keep the non-streaming path as the canonical test vector; stream tests use a goldenfile of concatenated chunks. |
| Format-aware question explosion confuses users | Default question count stays close to current; format extensions are clearly labelled in the UI. |

## Out-of-scope reminders

- A native chat-LLM "free-form" mode where the user can ask the bot anything is **not** in this plan. The interview is still deterministic + targeted deep dives. A free-form mode is a candidate for a future ADR.
- Multi-user accounts, sharing, and cloud sync are deliberately excluded. They would force a security model and an auth story that the local-first product does not need today.
- Native HTML preview of the homepage_section format inside the app (visual editor) is out of scope; the format produces validated HTML strings that the user pastes into their site.

## Immediate next implementation step

Issues [#17](https://github.com/terisuke/note_maker/issues/17)–[#29](https://github.com/terisuke/note_maker/issues/29) are filed. Start with **[#18](https://github.com/terisuke/note_maker/issues/18)** (SSE streaming, progress, and cancellation) on a feature branch off `develop`, then fold the transcript work from [#17](https://github.com/terisuke/note_maker/issues/17) on top of those streaming primitives.
