# ADR 0003: Conversation-first workspace UI with Alpine.js

Date: 2026-05-03

## Status

Accepted. Supersedes the **UX direction** section of [ADR 0002 §80-86](0002-multi-persona-multi-format-extension.md#ux-direction). All other parts of ADR 0002 (persona, format, persistence, source acquisition, prompt strategy) remain authoritative.

This ADR is implementation-bound: the product baseline already passed end-to-end functional testing on 2026-05-03, but a conversation-first workspace as designed in ADR 0002 was never actually built. This ADR fixes that gap and locks in the framework choice so the rewrite can be parallelized across Cursor, Codex CLI, and final touch-ups.

## Context

ADR 0002 §80-86 explicitly stated:

> The single-page form becomes a **conversation-first workspace**:
> - Left rail: persona switcher, project history, "previous style guides" picker.
> - Centre: chat-style transcript. Past answers are clickable and editable.
> - Right rail (artifact panel): live brief card and live draft preview.
> - "The conversation feel matches the Claude/ChatGPT/Gemini comparator class."

The 2026-05-03 audit found the actual implementation does not match this:

- `static/index.html` lays out four `<section class="panel">` blocks stacked vertically (Step 0 設定 → Step 1 文体分析 → Step 2 取材 → Step 3 生成と評価) inside a single-column `app-shell` of `width: min(1120px, calc(100% - 32px))`. There is no left or right rail.
- `static/css/style.css` `.workflow { display: grid; gap: 18px }` is single-column.
- The chat bubbles (`question-bubble`, `answer-bubble`, `transcript-item` in `static/js/script.js:561-606`) exist but are scoped to the Step 2 panel, not the page-level transcript.
- The history picker (`#history-persona-select` … `#history-session-select`) is six `<select>` elements stacked inside the Step 0 settings panel. Re-opening a past article requires populating six dropdowns in order rather than clicking one item in a sidebar.
- `static/js/script.js` is a 3,550-line single file with 93 `getElementById`/`querySelector` calls and 54 `addEventListener` calls; no module split, no reactive store. Three-pane sync would require building one from scratch.
- Phase A1/A2/A4 (chat transcript, SSE, deep-dive rationale) shipped, but no issue tracked the page-level layout rewrite that ADR 0002 §80-86 implies.

The 13 Playwright E2E tests in `tests/e2e/` lock in DOM ID selectors (`page.locator("#persona-id-input")` etc), so the rewrite must preserve those IDs to keep regression coverage.

## Decision

Adopt **Alpine.js v3** as the UI reactivity layer. Restructure `static/index.html` and `static/css/style.css` into a three-pane CSS Grid layout that matches ADR 0002 §80-86. Decompose `static/js/script.js` into ES modules with an `Alpine.store('app', …)` as the single shared state store. Keep all DOM IDs that the Playwright tests rely on.

### Framework choice rationale

Three options were evaluated against the constraints (preserve existing 13 E2E tests, keep launcher single-binary, minimum delta, support future Wails desktop wrap and Docker LAN serving).

| Option | Pure vanilla refactor | **Alpine.js v3** | React + Vite |
|---|---|---|---|
| Build step required | none | none | npm + Vite |
| Bundle delta | 0 | ~7 kB (alpine.min.js) | 40+ kB |
| Three-pane reactive sync | self-rolled pub/sub | `Alpine.store()` | React state |
| Existing 13 E2E impact | none | none (IDs preserved) | rewrite (testid migration) |
| ETA to ship 3-column workspace | 3-5 days | 4-6 days | ~2 weeks |
| Compatible with Wails desktop wrap | yes | yes | yes |
| Compatible with Docker LAN serving | yes | yes | yes |

Alpine.js was chosen because it removes the worst of vanilla's pain (3-pane store-driven sync) without adding the npm/Vite build pipeline that React requires. This aligns with the user's "minimum changes if possible" constraint while still producing the conversation-first feel.

`marked.min.js` and `alpine.min.js` are both pinned and committed under `static/vendor/`, so the launcher does not depend on internet access for UI scripts at startup.

### Layout target

```
┌────────────────────────────────────────────────────────────────────────┐
│ topbar  Note Maker  ·  persona switcher dropdown  ·  model status pill │
├──────────────┬──────────────────────────────────┬──────────────────────┤
│ left rail    │ centre transcript                │ right artifact rail  │
│              │                                  │                      │
│ ▸ persona    │ [system] 文体ガイドを読み込み中  │ ◇ 文体ガイドカード   │
│   - terisuke │ [you]    今日は IPv6 移行の話を  │  - 一人称: 僕        │
│   - cloudia  │ [agent]  読者は誰に届けますか?   │  - 構成パターン      │
│   + add      │ [you]    社内の DevOps 担当      │  ...                 │
│              │ [agent]  ストリーミング中...     │                      │
│ ▸ history    │                                  │ ◇ ブリーフカード     │
│   conversation│ [input] あなたの回答を入力...    │  - テーマ            │
│   item list  │  [送信] [停止] [深掘り skip]     │  - 読者              │
│              │                                  │  ...                 │
│ ⚙ 詳細設定   │                                  │ ◇ 下書きプレビュー   │
│              │                                  │  ...                 │
└──────────────┴──────────────────────────────────┴──────────────────────┘
```

CSS Grid:

```css
.workspace { display: grid; grid-template-columns: 240px 1fr 360px; }
@media (max-width: 1024px) { .workspace { grid-template-columns: 1fr; } }
```

### State model

Single `Alpine.store('app', …)` with these slices:

- `personas`: `{ list, selectedId }`
- `formats`: `{ list, selectedId }`
- `transcript`: `{ items, streamState }`
- `artifacts`: `{ styleGuide, brief, draft, draftEvaluation, sourceSnapshot }`
- `history`: `{ articles, selectedArticleId }`
- `config`: `{ models, storage, customQuestions }` (lives in the settings drawer)
- `runtime`: `{ modelStatusPill, fallbackHealth }`

Server interaction stays at the existing JSON / SSE endpoints (`internal/handlers/*.go`); the API is unchanged. Handlers remain the JSON boundary; UI restructuring does not move business logic into the frontend.

### Components

Each region is implemented as an Alpine component (one HTML region with `x-data` plus a small JS module that defines the component).

- `personaSwitcher` — top bar dropdown + "+ Add persona" modal trigger
- `historySidebar` — left rail conversation list (one article = one item, replaces six `<select>`)
- `transcriptPane` — centre scrollable transcript with bubble groups
- `composer` — input box, send/cancel/skip-deep-dive controls, format-aware
- `artifactRail` — right rail with collapsible style guide / brief / draft cards
- `settingsDrawer` — slide-over panel that hosts model selectors, storage mode, custom-question CRUD (退避先)
- `personaForm` — modal for create / edit / delete custom persona

Each component file lives at `static/js/components/<name>.js` and imports a shared `static/js/store.js`. Use ES module `<script type="module">`.

### Test preservation

The 13 Playwright tests in `tests/e2e/` reference DOM IDs:

```
#persona-select, #add-persona-btn, #edit-persona-btn, #delete-persona-btn,
#add-persona-form, #persona-id-input, #persona-display-name-input,
#persona-default-format-select, #persona-description-input,
#persona-source-kind-input, #persona-source-ref-input, #save-persona-btn,
#format-select, #style-model, #brief-model, #draft-model, #verify-model,
#storage-driver, #storage-path, #save-storage-btn,
#refresh-history-btn, #history-persona-select, #history-project-select,
#history-article-select, #history-draft-select, #history-style-select,
#history-session-select, #open-history-btn, #clear-history-selection-btn,
#analyze-style-btn, #use-preset-style-btn, #style-username, #style-limit,
#start-interview-btn, #answer-input, #submit-answer-btn,
#cancel-answer-btn, #skip-deep-dive-btn, #generate-draft-btn,
#cancel-draft-btn, #copy-btn, #copy-preview-btn,
#regenerate-section-btn, #section-candidate-output,
#accept-section-btn, #reject-section-btn,
#brief-card, #brief-preview, #style-guide-card, #guide-preview,
#preview-content, #markdown-output, #evaluation-summary,
#verification-summary, #model-status, #loading, #error-message-area,
#question-log, #section-status, #section-candidate
```

All existing IDs must remain attached to a DOM element with the same role. IDs may move between regions (e.g., `#refresh-history-btn` moves from settings into the left rail) but must not be deleted. New regions add new IDs; removed regions are not allowed unless the corresponding test is updated in the same PR.

## Out of scope

- SPA framework migration (React, Vue, Svelte). A future ADR may revisit this if Alpine clearly limits scope.
- Mobile-native build (Capacitor, React Native). Mobile is handled by the responsive breakpoint only.
- TypeScript adoption. The ES-module split makes TypeScript a future-compatible follow-up but is not required by this ADR.
- Visual redesign beyond layout structure. Existing colors, typography, and component visuals stay; only the spatial arrangement changes.
- Streaming protocol changes. SSE endpoints and cancel semantics from Phase A2 stay.
- Multi-user data isolation. That is owned by [ADR 0004](0004-three-tier-deployment.md).

## Consequences

Positive:

- Conversation feel matches ADR 0002 §80-86 and the user's stated comparators (Claude / ChatGPT / Gemini).
- Three-pane layout exposes brief card and draft preview during the interview, eliminating the "scroll to see what changed" friction.
- Module split makes future feature work (search, multi-conversation, settings deep-dive) a localized change instead of a 3,500-line single-file edit.
- Alpine.store keeps state predictable, which is a prerequisite for the Wails desktop wrap (single-process state) and the Docker LAN deployment (stateless reload).
- Existing E2E coverage stays green throughout the rewrite if IDs are preserved.

Negative:

- Adds one runtime dependency (`alpine.min.js`, ~7 kB, vendored). Mitigated by version pinning and offline vendoring.
- Component file count rises from 1 (`script.js`) to ~7. Mitigated by clear directory structure and module discipline.
- Settings drawer pattern hides model selectors and storage mode behind a button click. Power users may need one extra click to reach those controls; documented in the launcher handoff.

Neutral:

- The framework choice does not constrain Tier 2 (Wails) or Tier 3 (Docker) packaging in [ADR 0004](0004-three-tier-deployment.md).

## References

- [ADR 0002 §UX direction](0002-multi-persona-multi-format-extension.md#ux-direction) — superseded sections.
- [Alpine.js project page](https://alpinejs.dev/)
- [Alpine.js GitHub releases](https://github.com/alpinejs/alpine/releases)
- Implementation plan: [Conversation-first workspace UI](../implementation-plans/conversation-workspace-ui.md)
- Issue: filed at issue creation time and linked from `docs/implementation-plans/issue-adr-guardrails.md`.
