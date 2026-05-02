# Issue 40 epic decomposition

Date: 2026-05-03

## Context

The full Tailnet Evo X2 media-matrix run completed against the primary OpenAI-compatible API path, but only `terisuke_note_essay` passed. The failures were useful, but they also showed that the scenario plan needed one more layer before implementation:

- The draft-only live matrix starts from completed `ArticleBrief` fixtures, so it cannot prove that the revised fixed questions are actually easier to answer.
- Early unusable drafts were discarded before enough diagnostic artifacts were written.
- Some failures were recoverable format errors: assistant preamble leakage and Zenn/Qiita notation leakage.
- The homepage section was judged with long-form article assumptions.

## Epic

[#40](https://github.com/terisuke/note_maker/issues/40) remains open as the runtime stabilization epic. It should not close until staged validation and consecutive-run acceptance criteria are met.

## Sub-issues

| Order | Issue | Scope | Why it exists |
|---:|---|---|---|
| 1 | [#70](https://github.com/terisuke/note_maker/issues/70) | Interview-template scenario | Proves question-template usability and medium-specific brief output before Evo X2 draft generation. |
| 2 | [#71](https://github.com/terisuke/note_maker/issues/71) | Failed draft artifacts | Preserves raw output and runtime metrics when validation fails before a draft is accepted. |
| 3 | [#72](https://github.com/terisuke/note_maker/issues/72) | Bounded format repair | Gives recoverable format mistakes one strict retry without weakening validators. |
| 4 | [#73](https://github.com/terisuke/note_maker/issues/73) | Output-format-specific gates | Separates long-form article gates from homepage short HTML gates. |
| 5 | [#74](https://github.com/terisuke/note_maker/issues/74) | Staged Evo X2 rerun | Runs template/offline/live validation in the correct order and records results back to #40. |

## Implementation order

1. Implement #70.
2. Implement #71/#72/#73 in parallel only if write scopes stay disjoint.
3. Run one live Evo X2 case from a previously failing medium.
4. Run the full note/Qiita/Zenn/Cor blog matrix only after the scenario and diagnostic gaps are closed.

## Docs updated

- [ADR 0002](../adrs/0002-multi-persona-multi-format-extension.md)
- [Issue and ADR guardrails](../implementation-plans/issue-adr-guardrails.md)
- [Next implementation cut](../implementation-plans/next-implementation-cut.md)
