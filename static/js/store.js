// Phase D2 cut D2-2: introduce Alpine.store('app') with 7 slices.
// See docs/adrs/0003-conversation-first-workspace-ui.md.
//
// Slice shapes:
//   personas:   list of persona records and the currently selected id.
//   formats:    list of output-format records and the currently selected id.
//   transcript: ordered interview messages and the SSE stream state.
//   artifacts:  rendered cards (style guide, brief, draft, evaluation, source).
//   history:    saved articles list and the currently opened article id.
//   config:     model defaults, storage config payload, custom-question defs.
//   runtime:    transient runtime status (model-status pill text, fallback health).
//
// Subsequent cuts migrate handlers from script.js to read/write through here.
// In D2-2 we only register the store and mirror five proof-of-life sites.

document.addEventListener('alpine:init', () => {
  if (!window.Alpine) return;
  window.Alpine.store('app', {
    personas:   { list: [], selectedId: null },
    formats:    { list: [], selectedId: null },
    transcript: { items: [], streamState: 'idle' },
    artifacts:  {
      styleGuide: null,
      brief: null,
      draft: null,
      draftEvaluation: null,
      sourceSnapshot: null,
    },
    history:    { articles: [], selectedArticleId: null },
    config:     { models: {}, storage: {}, customQuestions: [] },
    runtime:    { modelStatusPill: 'checking', fallbackHealth: null },
  });
});
