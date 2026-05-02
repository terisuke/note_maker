# Issue 66 plain brief questions validation - 2026-05-03

## Goal

Before live Evo X2 measurement, reduce the user's answering burden in the brief interview. The previous fixed questions were too broad and editorial, which made the one-question-at-a-time flow feel mentally heavy.

## Implementation

- Rewrote the base fixed questions in plain Japanese.
- Split broad prompts into smaller prompts:
  - reader problem,
  - key takeaway,
  - concrete example,
  - evidence,
  - title or heading keywords.
- Expanded medium-specific prompts:
  - note: emotional/story arc,
  - Zenn/Qiita: stack, prerequisite knowledge, reproduction proof, code examples, references,
  - Cor company blog: technical report vs vision-sharing purpose and desired reader reaction.
- Softened fallback follow-up wording so it asks for one concrete scene, step, number, feeling, or reason.
- Updated the LLM follow-up prompt to request shorter, easier questions and avoid difficult editorial terms.
- The UI now wraps template question text, labels required/optional questions, and lets optional questions advance as `未定` when the answer box is empty.
- Additional question answers now keep their question label in the final draft prompt instead of being appended as unlabeled fragments.

## Verification

Commands:

```bash
go test ./internal/domain/brief ./internal/application/brief ./internal/application/draft ./internal/handlers ./cmd/scenario/media_matrix
node --check static/js/script.js
go run ./cmd/scenario/media_matrix
```

Result:

- All checks passed.
- Offline media matrix completed with `question_template_ids=14`, `composed_templates=10`, and `cases=6`.

## Remaining measurement

This change intentionally does not run the live Evo X2 media matrix. The next step is to run one bounded #40 case after confirming the new interview flow is usable in the browser, then proceed to the full Note/Qiita/Zenn/Cor blog comparison.
