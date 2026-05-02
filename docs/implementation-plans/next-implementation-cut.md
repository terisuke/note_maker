# Next implementation cut

Date: 2026-05-02

This document translates the current open issue set into the next executable implementation sequence after the Tailnet Evo X2 runtime fixes.

## Current issue state

Closed and incorporated:

- [#11](https://github.com/terisuke/note_maker/issues/11) — strict Terisuke style tuning.
- [#21](https://github.com/terisuke/note_maker/issues/21) — Persona and OutputFormat domain concepts.
- [#38](https://github.com/terisuke/note_maker/issues/38) — Evo X2 Tailnet OpenAI-compatible API as primary runtime.

Open and active:

- Phase A: [#17](https://github.com/terisuke/note_maker/issues/17), [#18](https://github.com/terisuke/note_maker/issues/18), [#19](https://github.com/terisuke/note_maker/issues/19), [#20](https://github.com/terisuke/note_maker/issues/20).
- Phase B remaining: [#22](https://github.com/terisuke/note_maker/issues/22), [#23](https://github.com/terisuke/note_maker/issues/23), [#24](https://github.com/terisuke/note_maker/issues/24), [#25](https://github.com/terisuke/note_maker/issues/25).
- Phase C: [#26](https://github.com/terisuke/note_maker/issues/26), [#27](https://github.com/terisuke/note_maker/issues/27), [#28](https://github.com/terisuke/note_maker/issues/28), extending [#14](https://github.com/terisuke/note_maker/issues/14).
- Quality and packaging: [#13](https://github.com/terisuke/note_maker/issues/13), [#15](https://github.com/terisuke/note_maker/issues/15), [#29](https://github.com/terisuke/note_maker/issues/29), [#36](https://github.com/terisuke/note_maker/issues/36).
- Runtime stabilization: [#40](https://github.com/terisuke/note_maker/issues/40) for primary Tailnet Evo X2 quality/metrics.

## Next target

Start with [#18](https://github.com/terisuke/note_maker/issues/18), not [#17](https://github.com/terisuke/note_maker/issues/17).

Reason: a real Tailnet Evo X2 scenario reached the correct endpoint but took `1396.80s` and still missed quality gates. A spinner-only UI is not usable at that latency. Streaming, heartbeat, cancellation, and partial-result retention are the highest-leverage improvement before reshaping the transcript.

## Implementation sequence

1. **[#18] SSE streaming and cancellation**
   - Add streaming to `internal/infrastructure/llamacpp`.
   - Add status, heartbeat, token, done, and error event types.
   - Support cancellation from browser disconnect and explicit Cancel.
   - Keep non-streaming generation for tests and compatibility.

2. **[#17] Chat transcript and editable answers**
   - Replace the bounded log with a transcript surface.
   - Render existing fixed and deep-dive answers as bubbles.
   - Add in-memory fork-on-edit endpoint first; persistence follows in Phase C.

3. **[#20] Deep-dive rationale in transcript**
   - Add parent-answer excerpts to prompt and UI.
   - Ensure LLM and rule-based fallback paths produce the same contextual prefix.

4. **[#19] Editable draft and section regenerate**
   - Make the Markdown draft editable after streaming lands.
   - Add section anchor parsing and replacement tests.
   - Regenerate one heading subtree at a time.

5. **[#26] SQLite persistence**
   - Persist answer forks, draft versions, guide versions, and project/article history.

## Additional gap

[#40](https://github.com/terisuke/note_maker/issues/40) tracks primary-runtime quality and runtime metrics. That work is separate from [#36](https://github.com/terisuke/note_maker/issues/36), which is only for local llama.cpp fallback quality. Do not block [#18](https://github.com/terisuke/note_maker/issues/18) on #40; streaming is needed precisely because these long primary-runtime runs can fail late and still need to preserve partial output.
