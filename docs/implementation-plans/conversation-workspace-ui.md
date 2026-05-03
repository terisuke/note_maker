# Conversation-first workspace UI implementation plan

Date: 2026-05-03

This plan implements [ADR 0003 — Conversation-first workspace UI with Alpine.js](../adrs/0003-conversation-first-workspace-ui.md). It also satisfies the UX direction that [ADR 0002 §UX direction](../adrs/0002-multi-persona-multi-format-extension.md#ux-direction) (lines 80-86) mandated but that was never built.

## Goals

- Restructure `static/index.html` and `static/css/style.css` into a three-pane CSS Grid layout (`240px | 1fr | 360px`) matching the target in ADR 0003.
- Replace ad-hoc DOM manipulation in `static/js/script.js` with an `Alpine.store('app', …)` as the single shared state source for all panes.
- Preserve all DOM IDs that the 13 Playwright end-to-end (E2E) tests reference, so that the test suite remains green throughout the rewrite.
- Ship without adding a build step. No npm, no Vite, no Webpack. Alpine.js and any other new libraries are vendored under `static/vendor/`.
- Fold model selectors, storage mode, and custom-question CRUD into a settings drawer so the central transcript pane has unobstructed focus.

## Non-goals

These match the out-of-scope section of ADR 0003:

- SPA framework migration (React, Vue, Svelte).
- Mobile-native build. Mobile is handled by the `@media (max-width: 1024px)` breakpoint only.
- TypeScript adoption.
- Visual redesign beyond layout structure. Colors, typography, and component visuals are unchanged.
- Streaming protocol changes. SSE endpoints and cancel semantics stay.
- Multi-user data isolation. That is owned by [ADR 0004](../adrs/0004-three-tier-deployment.md) Tier 3.

## Cut sequence

### Cut D2-1: Vendor Alpine.js

Files to touch: `static/vendor/alpine.min.js`, `static/vendor/lockfile.json` (new).

Pin Alpine.js v3 (latest stable as of 2026-05). Download the minified build. Record the SHA-256 digest in `static/vendor/lockfile.json` alongside the pinned version string. Add the `<script defer src="/static/vendor/alpine.min.js"></script>` tag to `static/index.html` before the closing `</body>` tag. Do not activate any `x-data` or `x-init` attributes yet.

Acceptance:

- `static/vendor/alpine.min.js` is present and the SHA-256 in `lockfile.json` matches.
- The page loads without JavaScript errors in the browser console.
- All 13 Playwright tests pass unchanged.

E2E expectation: green (no change to DOM structure).

Delegation: Cursor-friendly. Mechanical file addition with no logic.

---

### Cut D2-2: Introduce `static/js/store.js`

Files to touch: `static/js/store.js` (new), `static/index.html`, `static/js/script.js`.

Create `static/js/store.js` as an ES module that registers `Alpine.store('app', …)` with the seven slices defined in ADR 0003: `personas`, `formats`, `transcript`, `artifacts`, `history`, `config`, `runtime`. Rewire approximately five of the most-used event handlers in `script.js` to read and write through the store as a proof-of-life. Choose handlers that already have clear input/output boundaries (for example, the persona-select change handler and the model-status update path).

Load `store.js` with `<script type="module" src="/static/js/store.js"></script>` in `static/index.html`. The Alpine `defer` script must load before the module scripts to ensure the store registration fires before any component reads it.

No visible UI change in this cut.

Acceptance:

- Opening the browser console shows `Alpine.store('app')` returns the expected shape.
- The five rewired handlers continue to behave identically to before.
- All 13 Playwright tests pass.

E2E expectation: green.

Delegation: Codex CLI-friendly. Store shape is fully specified in ADR 0003; wiring five handlers is repetitive.

---

### Cut D2-3: Three-pane CSS Grid scaffold

Files to touch: `static/css/style.css`, `static/index.html`.

Replace the `.workflow { display: grid; gap: 18px }` single-column grid with a `.workspace` container:

```css
.workspace {
  display: grid;
  grid-template-columns: 240px 1fr 360px;
  gap: 0;
  height: calc(100vh - 48px); /* subtract topbar height */
}
@media (max-width: 1024px) {
  .workspace { grid-template-columns: 1fr; }
}
```

Move the existing four `<section class="panel">` blocks into three grid regions. Keep their content and IDs completely unchanged; only the spatial container changes. The `.app-shell` wrapper around `width: min(1120px, calc(100% - 32px))` is replaced by the full-width `.workspace` grid.

Acceptance:

- The three-column layout renders in a viewport wider than 1024px.
- All existing panel content is visible and reachable.
- All 13 Playwright tests pass.

E2E expectation: green. This is the highest-risk cut for test breakage; run the full suite before merging.

Delegation: needs human judgment. Layout decisions (which existing panel goes into which region, overflow behavior) are not purely mechanical.

---

### Cut D2-4: Left-rail history sidebar

Files to touch: `static/index.html`, `static/js/components/historySidebar.js` (new), `static/js/store.js`.

Replace the six `<select>` history pickers (`#history-persona-select`, `#history-project-select`, `#history-article-select`, `#history-draft-select`, `#history-style-select`, `#history-session-select`) with a single conversation-list view in the left rail. The original six selects are wrapped in a `<details>` element with summary text "詳細選択" and remain in the DOM. This keeps Playwright tests that interact with those IDs fully functional and also documents the migration path for any operator tooling that references them.

The new conversation list calls the existing `/api/history` endpoint and the project/article endpoints (`GET /api/projects`, `GET /api/articles/{id}`) introduced in ADR 0002. Clicking a conversation item populates the `history` store slice and fires the same logic that `#open-history-btn` currently fires.

`#refresh-history-btn` is moved from the settings panel into the left-rail header. The `id` attribute stays on the element.

Acceptance:

- Clicking a conversation item in the list opens the corresponding session (transcript, brief card, draft).
- `#refresh-history-btn` click in the new location still triggers a history refresh.
- The `<details>詳細選択</details>` fallback reveals all six original selects.
- All 13 Playwright tests pass (they interact with the selects inside `<details>` and with `#refresh-history-btn`, both of which remain in the DOM).

E2E expectation: green.

Delegation: Codex CLI-friendly. The API calls are already implemented; the component is mostly HTML restructuring driven by the store.

---

### Cut D2-5: Right-rail artifact panel

Files to touch: `static/index.html`, `static/js/components/artifactRail.js` (new), `static/js/store.js`.

Pin `#brief-card`, `#style-guide-card`, and a new `#draft-preview-card` to the right rail. The card elements are moved within the page; their `id` attributes stay on the same DOM elements. Each card subscribes to the corresponding `artifacts` store slice and updates when the slice changes. The existing card rendering logic (brief card, style-guide card) is extracted into `artifactRail.js`; the inline script calls in `script.js` that currently update those elements are replaced by store mutations.

`#draft-preview-card` wraps the existing `#preview-content` div and updates when `artifacts.draft` changes.

Acceptance:

- Brief card updates during the interview phase without a page scroll.
- Style-guide card is visible in the right rail when a guide is loaded.
- Draft preview card shows the current draft after generation.
- `#brief-card`, `#brief-preview`, `#style-guide-card`, `#guide-preview`, and `#preview-content` IDs remain on their elements.
- All 13 Playwright tests pass.

E2E expectation: green.

Delegation: Codex CLI-friendly. Mechanical element relocation; store subscription pattern is established by D2-2.

---

### Cut D2-6: Centre transcript

Files to touch: `static/index.html`, `static/js/components/transcriptPane.js` (new), `static/js/components/composer.js` (new), `static/js/store.js`.

Promote `#question-log` from the Step 2 panel to the page-level central pane. The element retains its `id` and its existing CSS classes. The central column is a flex column: `#question-log` occupies the scrollable upper portion; the composer (`#answer-input`, `#submit-answer-btn`, `#cancel-answer-btn`, `#skip-deep-dive-btn`) sticks to the bottom.

The Step 2 panel header and any remaining controls that are not the transcript or composer are folded into the settings drawer (Cut D2-7). `#start-interview-btn` and `#generate-draft-btn` move to the topbar or to the composer area; their IDs stay attached.

`transcriptPane.js` manages scroll-to-bottom behavior and bubble rendering. `composer.js` manages the input state and button enabled/disabled logic, reading from `transcript.streamState`.

Acceptance:

- The transcript is the default focused region when the app loads.
- New messages append at the bottom and the pane auto-scrolls.
- `#question-log`, `#answer-input`, `#submit-answer-btn`, `#cancel-answer-btn`, `#skip-deep-dive-btn` retain their IDs and remain accessible.
- All 13 Playwright tests pass.

E2E expectation: green.

Delegation: needs human judgment for the composer layout and the placement of `#start-interview-btn` / `#generate-draft-btn` within the topbar or composer area.

---

### Cut D2-7: Settings drawer

Files to touch: `static/index.html`, `static/js/components/settingsDrawer.js` (new), `static/css/style.css`.

Create a slide-over drawer triggered by a gear icon (`⚙ 詳細設定`) in the topbar. The drawer contains: model selectors (`#style-model`, `#brief-model`, `#draft-model`, `#verify-model`), storage mode (`#storage-driver`, `#storage-path`, `#save-storage-btn`), and custom-question CRUD. All those input IDs stay attached to their existing elements; the elements move under the drawer container.

Add `role="dialog"`, `aria-modal="true"`, `aria-labelledby` to the drawer element. Add `aria-label` to the trigger button.

The `#loading` and `#error-message-area` elements stay at the page level (not inside the drawer) so they are visible regardless of drawer state.

Acceptance:

- The drawer opens and closes without a page reload.
- All model selector IDs and storage IDs remain accessible in the DOM (inside the drawer).
- `#loading` and `#error-message-area` are visible when triggered regardless of drawer state.
- All 13 Playwright tests pass.

E2E expectation: green.

Delegation: Cursor-friendly for the HTML structure; Codex CLI-friendly for the ARIA attributes and CSS transition.

---

### Cut D2-8: Polish and accessibility

Files to touch: `static/css/style.css`, `static/index.html`, component JS files as needed.

- Keyboard navigation: Tab order through the three panes must be logical. The drawer must trap focus when open and return focus to the trigger on close.
- ARIA: Ensure `#model-status` has `role="status"` and `aria-live="polite"`. Ensure `#error-message-area` has `role="alert"`.
- Focus management: After submitting an answer, focus returns to `#answer-input`.
- Mobile breakpoint sanity: At widths below 1024px, the single-column layout must not overflow. The right and left rails collapse; a toggle button exposes each.
- Visual regression review: Run a manual side-by-side comparison of the before/after screenshots recorded during D2-3.

Acceptance:

- Tab navigation reaches every interactive element without requiring a mouse.
- VoiceOver / NVDA announces the model status pill and error messages automatically.
- The mobile single-column layout passes a manual review at 375px viewport width.
- All 13 Playwright tests pass.

E2E expectation: green.

Delegation: needs human judgment for focus management decisions and mobile layout review.

---

## Shared scaffolding appendix

### `static/vendor/alpine.min.js` provenance

Download from the official Alpine.js GitHub releases (`https://github.com/alpinejs/alpine/releases`). Pin to a specific version tag (for example `v3.x.y`). Record the version and SHA-256 in `static/vendor/lockfile.json`:

```json
{
  "alpine.min.js": { "version": "3.x.y", "sha256": "<digest>" },
  "marked.min.js": { "version": "x.y.z", "sha256": "<digest>" }
}
```

### `static/js/store.js` skeleton

```js
document.addEventListener('alpine:init', () => {
  Alpine.store('app', {
    personas:  { list: [], selectedId: null },
    formats:   { list: [], selectedId: null },
    transcript:{ items: [], streamState: 'idle' },
    artifacts: { styleGuide: null, brief: null, draft: null, draftEvaluation: null, sourceSnapshot: null },
    history:   { articles: [], selectedArticleId: null },
    config:    { models: {}, storage: {}, customQuestions: [] },
    runtime:   { modelStatusPill: '', fallbackHealth: {} },
  });
});
```

### Expected directory tree under `static/js/`

```
static/js/
  store.js
  script.js          (retained; shrinks as cuts migrate handlers to components)
  components/
    historySidebar.js
    transcriptPane.js
    composer.js
    artifactRail.js
    settingsDrawer.js
    personaSwitcher.js
    personaForm.js
```

### Migration of `marked.min.js`

`marked.min.js` is currently loaded from a CDN reference in `static/index.html`. In Cut D2-1 or immediately after, download the pinned version to `static/vendor/marked.min.js`, update the `<script>` tag to `/static/vendor/marked.min.js`, and add its SHA-256 to `lockfile.json`. This ensures the Wails desktop build (Tier 2) and Docker container (Tier 3) have no runtime internet dependency for UI scripts.

---

## Validation baseline

Run after each cut before merging:

```
go test ./...
python3 -m pytest tests/e2e -q
./scripts/check-launcher.sh
git diff --check
```

---

## Delegation matrix

| Cut | Best owner | Reason |
|---|---|---|
| D2-1: Vendor Alpine.js | Cursor | Mechanical file addition; no logic |
| D2-2: Introduce store.js | Codex CLI | Store shape fully specified; handler wiring is repetitive |
| D2-3: Three-pane CSS Grid | Human judgment | Layout region assignment requires visual review |
| D2-4: Left-rail history sidebar | Codex CLI | API calls exist; component is HTML restructuring driven by store |
| D2-5: Right-rail artifact panel | Codex CLI | Established store subscription pattern; element relocation |
| D2-6: Centre transcript | Human judgment for layout | Composer placement and button relocation require visual decisions |
| D2-7: Settings drawer | Cursor (HTML) + Codex CLI (ARIA/CSS) | Clear spec; ARIA attributes are mechanical |
| D2-8: Polish and a11y | Human judgment | Focus management and mobile review require manual testing |
