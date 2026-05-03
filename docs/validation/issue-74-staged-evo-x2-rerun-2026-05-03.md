# Issue #74 staged Evo X2 rerun

Date: 2026-05-03

## Scope

Ran the first staged Tailnet Evo X2 validation slice after implementing #70-#73.

This was intentionally limited to one previously failing medium:

- Case: `cloudia_zenn_tutorial`
- Medium: Zenn
- Previous failure class: format validation failed before draft/evaluation artifacts were available.

## Command

```sh
LIVE_MEDIA_MATRIX_CASES=cloudia_zenn_tutorial make scenario-media-matrix-live
```

Runtime:

- Endpoint: `http://evo-x2.tailb30e58.ts.net/v1`
- Draft model: `gemma4:31b`
- Verify model: `gemma4:latest`
- Transport: Tailscale VPN / OpenAI-compatible API

## Result

The final rerun did not pass the strict gate, but it progressed past the previous format-failure layer:

| Case | Status | Seconds | First chunk | Chunks | Score / min | Runes / min | Verification |
|---|---|---:|---:|---:|---:|---:|---|
| `cloudia_zenn_tutorial` | failed | `702.17` | `169491ms` | `1435` | `73.6 / 82.0` | `3905 / 1800` | passed |

Artifacts:

- Aggregate: `tmp/media_matrix/live/aggregate.json`
- Report: `tmp/media_matrix/live/aggregate.md`
- Draft: `tmp/media_matrix/live/cloudia_zenn_tutorial/draft.md`
- Evaluation: `tmp/media_matrix/live/cloudia_zenn_tutorial/evaluation.json`
- Verification: `tmp/media_matrix/live/cloudia_zenn_tutorial/verification.json`

## Interpretation

This is a useful staged failure:

- The Tailnet path worked.
- The Zenn format validator accepted the generated article.
- The rune gate passed.
- Lightweight final verification passed.
- The remaining failure is strict style score: `73.6`, below the Zenn gate `82.0`.

Issue #74 should remain open. The blocker is now specifically **Cloudia/Zenn style calibration**, not runtime, format, artifact capture, repair, length, or final verification.

The next implementation work should use the preserved draft and evaluation artifacts to tune Cloudia/Zenn style guidance or scoring calibration before spending a full Evo X2 matrix run. The calibration must keep the Zenn validator strict: no assistant preamble, valid Zenn frontmatter, Zenn-only notation where applicable, and no Qiita notation leakage.

## Calibration rerun

The current cut added the missing reliability layer before rerunning the same Zenn case:

- each media-matrix case now has its own style `profile.json` and `guide.json`,
- live media-matrix runs pass those case-specific artifacts to `draft_generation`,
- draft generation rejects profile/guide/brief style-profile mismatches,
- final verification failure now blocks `scenario_passed`,
- structural signals are enforced by the live aggregate runner,
- the web response exposes `quality_gate` details so low-score drafts remain visible.

Command:

```sh
LIVE_MEDIA_MATRIX_CASES=cloudia_zenn_tutorial make scenario-media-matrix-live
```

Result:

| Case | Status | Seconds | First chunk | Chunks | Score / min | Runes / min | Verification |
|---|---|---:|---:|---:|---:|---:|---|
| `cloudia_zenn_tutorial` | passed | `741.93` | `86456ms` | `2205` | `88.1 / 82.0` | `5040 / 1800` | passed |

Runtime:

- Endpoint: `http://evo-x2.tailb30e58.ts.net/v1`
- Draft model: `gemma4:31b`
- Verify model: `gemma4:latest`
- Transport: Tailscale VPN / OpenAI-compatible API

Artifacts:

- Aggregate: `tmp/media_matrix/live/aggregate.json`
- Report: `tmp/media_matrix/live/aggregate.md`
- Draft: `tmp/media_matrix/live/cloudia_zenn_tutorial/draft.md`
- Evaluation: `tmp/media_matrix/live/cloudia_zenn_tutorial/evaluation.json`
- Verification: `tmp/media_matrix/live/cloudia_zenn_tutorial/verification.json`
- Style profile: `tmp/media_matrix/styles/cloudia_zenn_tutorial/profile.json`
- Style guide: `tmp/media_matrix/styles/cloudia_zenn_tutorial/guide.json`

Interpretation:

- The Tailnet path still works.
- The Zenn format validator accepted the article.
- Required Zenn structural signals were present.
- The rune gate passed.
- Lightweight final verification passed.
- The strict style gate now passes: `88.1`, above the Zenn gate `82.0`.

## Next proof sequence

1. Run the full live publishing-target matrix for note, Qiita, Zenn, and Cor blog.

## Adjacent Qiita proof

After the Zenn proof passed, the next bounded run checked the adjacent Cloudia technical target: Qiita.

Purpose:

- Verify that the Cloudia style calibration generalizes beyond Zenn.
- Confirm that Qiita-specific format and structural gates remain strict.
- Catch cross-format regression, especially Zenn-only notation leaking into Qiita as an actual block.

Command:

```sh
LIVE_MEDIA_MATRIX_CASES=cloudia_qiita_how_to make scenario-media-matrix-live
```

Runtime:

- Endpoint: `http://evo-x2.tailb30e58.ts.net/v1`
- Draft model: `gemma4:31b`
- Verify model: `gemma4:latest`
- Transport: Tailscale VPN / OpenAI-compatible API

Result:

| Case | Status | Seconds | First chunk | Chunks | Score / min | Runes / min | Verification |
|---|---|---:|---:|---:|---:|---:|---|
| `cloudia_zenn_tutorial` | passed | `741.93` | `86456ms` | `2205` | `88.1 / 82.0` | `5040 / 1800` | passed |
| `cloudia_qiita_how_to` | passed | `598.62` | `111645ms` | `1336` | `83.7 / 82.0` | `3318 / 1400` | passed |

Artifacts:

- Aggregate: `tmp/media_matrix/live/aggregate.json`
- Report: `tmp/media_matrix/live/aggregate.md`
- Draft: `tmp/media_matrix/live/cloudia_qiita_how_to/draft.md`
- Evaluation: `tmp/media_matrix/live/cloudia_qiita_how_to/evaluation.json`
- Verification: `tmp/media_matrix/live/cloudia_qiita_how_to/verification.json`
- Style profile: `tmp/media_matrix/styles/cloudia_qiita_how_to/profile.json`
- Style guide: `tmp/media_matrix/styles/cloudia_qiita_how_to/guide.json`

Interpretation:

- The Tailnet path still works for the adjacent Cloudia technical case.
- The Qiita format validator accepted the article.
- Required Qiita structural signals were present: frontmatter `title:`, `:::note`, `diff_` code fence, and `## ` headings.
- The draft used Qiita's `diff_go` style rather than Zenn's `diff go` style.
- Zenn `:::message` appeared only as explanatory inline/table text, not as an actual block.
- The rune gate passed.
- Lightweight final verification passed.
- The strict style gate passed: `83.7`, above the Qiita gate `82.0`.

The two bounded Cloudia technical proofs now both pass. The next useful Evo X2 spend is the full publishing-target matrix.

## #40 closure condition

Issue #40 can close only after the full live matrix records endpoint, per-phase models, elapsed seconds, generated runes, style score, final verification result, and artifact paths for every publishing target, with:

- no runtime endpoint failure,
- no output-format validation failure,
- no final-verification failure,
- no strict style-gate failure,
- no structural-signal failure,
- no style profile/guide/brief mismatch,
- no missing failed-or-successful draft artifacts.

The homepage section case can remain as a separate format check, but it is not part of the #40 publishing-target closure gate.
