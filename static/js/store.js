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
//
// Boot ordering: the deferred Alpine CDN/vendor script in <head> can dispatch
// 'alpine:init' before this module executes, so we register via two paths:
//   Path A: listen for 'alpine:init' (Alpine boots after this module).
//   Path B: register synchronously when window.Alpine is already present at
//           module-load time (Alpine has already booted before this module).

function registerAppStore(Alpine) {
  Alpine.store('app', {
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
}

// Path A: store.js runs before Alpine has booted — register on alpine:init.
document.addEventListener('alpine:init', () => {
  if (window.Alpine) registerAppStore(window.Alpine);
});

// Path B: store.js runs after Alpine has already booted (possible because the
// defer'd Alpine in <head> may dispatch alpine:init before this module
// executes). Register synchronously if Alpine is already present and the
// store is not yet defined.
if (
  window.Alpine &&
  typeof window.Alpine.store === 'function' &&
  window.Alpine.store('app') === undefined
) {
  registerAppStore(window.Alpine);
}
