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

At this stage, Issue #74 remained open. The blocker was specifically **Cloudia/Zenn style calibration**, not runtime, format, artifact capture, repair, length, or final verification.

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

## Full publishing-target matrix rerun

After the bounded Cloudia/Zenn and Cloudia/Qiita proofs passed, the full publishing-target matrix was rerun against the same Evo X2 Tailnet OpenAI-compatible API.

Command:

```sh
RUN_LIVE_MEDIA_MATRIX=1 \
SCENARIO_STREAM_DRAFT=1 \
SCENARIO_OUTPUT_DIR=tmp/media_matrix \
LIVE_MEDIA_MATRIX_OUTPUT_DIR=tmp/media_matrix/live \
LIVE_MEDIA_MATRIX_CASES=terisuke_note_essay,cor_blog_technical_report,cor_blog_vision_sharing,cloudia_zenn_tutorial,cloudia_qiita_how_to \
LLM_BASE_URL=http://evo-x2.tailb30e58.ts.net/v1 \
DRAFT_LLM_MODEL=gemma4:31b \
VERIFY_LLM_MODEL=gemma4:latest \
LLM_FALLBACK_BASE_URLS=http://evo-x2.tailb30e58.ts.net/llama/v1 \
go run ./cmd/scenario/live_media_matrix
```

Runtime:

- Endpoint: `http://evo-x2.tailb30e58.ts.net/v1`
- Draft model: `gemma4:31b`
- Verify model: `gemma4:latest`
- Fallback chain configured: Evo X2 llama.cpp `/llama/v1`, then workstation-local fallback when configured
- Transport: Tailscale VPN / OpenAI-compatible API

Result:

| Case | Status | Attempt | Seconds | First chunk | Chunks | Score / min | Runes / min | Verification |
|---|---|---:|---:|---:|---:|---:|---:|---|
| `terisuke_note_essay` | passed | 1 | `107.86` | `67818ms` | `1610` | `90.7 / 82.0` | `2849 / 2800` | passed |
| `cor_blog_technical_report` | passed | 1 | `152.34` | `67984ms` | `1696` | `81.4 / 80.0` | `3329 / 2200` | passed |
| `cor_blog_vision_sharing` | passed | 1 | `113.98` | `68986ms` | `1657` | `89.5 / 80.0` | `3156 / 1600` | passed |
| `cloudia_zenn_tutorial` | passed | 1 | `121.14` | `65568ms` | `2686` | `86.4 / 82.0` | `5641 / 1800` | passed |
| `cloudia_qiita_how_to` | passed | 2 | `114.75` | `68857ms` | `1898` | `82.2 / 82.0` | `3737 / 1400` | passed |

Aggregate:

- Cases: 5
- Passed: 5
- Failed: 0
- Average seconds: `122.01`
- Average score: `86.0`
- Average runes: `3742`
- Aggregate: `tmp/media_matrix/live/aggregate.json`
- Report: `tmp/media_matrix/live/aggregate.md`

Interpretation:

- The Tailnet Evo X2 primary path is usable for all current publishing targets.
- The fallback chain is configured but was not needed for this passing aggregate.
- All long-form structural gates passed.
- All format validators accepted the generated output.
- All lightweight final verification checks passed.
- The Qiita case required a second attempt; the retry succeeded after the scenario made `## 参考リンク` an explicit required structure.

Issue #74 can close based on this run. Issue #40 can also close for the current note/Qiita/Zenn/Cor blog publishing-target acceptance scope. The homepage section remains a separate short-format check and is intentionally outside this closure gate.

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

The two bounded Cloudia technical proofs both passed. The next Evo X2 spend was the full publishing-target matrix, recorded above as a `5/5` pass.

## #40 closure condition

Issue #40 can close because the full live matrix now records endpoint, per-phase models, elapsed seconds, generated runes, style score, final verification result, and artifact paths for every publishing target, with:

- no runtime endpoint failure,
- no output-format validation failure,
- no final-verification failure,
- no strict style-gate failure,
- no structural-signal failure,
- no style profile/guide/brief mismatch,
- no missing failed-or-successful draft artifacts.

The homepage section case can remain as a separate format check, but it is not part of the #40 publishing-target closure gate.
