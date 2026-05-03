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

Issue #74 should remain open. The next implementation work should tune Cloudia/Zenn style scoring or prompt calibration before the full note/Qiita/Zenn/Cor blog live matrix is rerun.
