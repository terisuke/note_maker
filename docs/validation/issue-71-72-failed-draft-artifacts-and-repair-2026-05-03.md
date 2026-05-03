# Issue #71/#72 failed draft artifacts and format repair

Date: 2026-05-03

## Scope

Implemented two related runtime-stabilization changes:

- [#71](https://github.com/terisuke/note_maker/issues/71): preserve failed draft artifacts and runtime metrics.
- [#72](https://github.com/terisuke/note_maker/issues/72): add one bounded format-repair retry for recoverable format failures.

## Behavior

- `draft.GenerateResult` now includes raw `GenerationAttempt` records for accepted generations and style revisions.
- Format validation failures return `draft.UnusableDraftError`, preserving raw model output and validation errors.
- `cmd/scenario/draft_generation` writes:
  - `raw_attempt_N_generation_M_KIND.txt`
  - `failure_attempt_N.json`
- `failure_attempt_N.json` records:
  - validation error,
  - elapsed seconds,
  - timeout seconds,
  - streaming/first-chunk/chunk metrics,
  - endpoint/model context,
  - persona and output format,
  - raw output artifact paths.
- `cmd/scenario/live_media_matrix` reads `failure_attempt_N.json` and restores elapsed time, model context, failure path, and raw output paths into aggregate rows.
- Recoverable format failures get one repair prompt. Validators remain strict.

## Validator fixes from live evidence

The Zenn staged live rerun showed two overly broad validator behaviors:

- Zenn/Qiita cross-format markers inside explanatory code fences or inline examples should not be treated as actual platform syntax leakage.
- Frontmatter formats should not run the legacy note-only preamble heuristic against body text after YAML frontmatter.

Both are now covered by domain tests.

## Verification

Commands:

```sh
go test ./cmd/scenario/draft_generation ./cmd/scenario/live_media_matrix ./internal/application/draft ./internal/domain/article ./internal/domain/format
go test ./...
```

Results:

- Targeted tests passed.
- Full test suite passed.

## Live evidence

The first Zenn live rerun after #71/#72 wrote failure diagnostics instead of returning an opaque `0.00s` row:

- Case: `cloudia_zenn_tutorial`
- Endpoint: `http://evo-x2.tailb30e58.ts.net/v1`
- Draft model: `gemma4:31b`
- Verify model: `gemma4:latest`
- Elapsed: `723.51s`
- First chunk: `143343ms`
- Chunks: `1874`
- Failure path: `tmp/media_matrix/live/cloudia_zenn_tutorial/failure_attempt_1.json`
- Raw outputs:
  - `tmp/media_matrix/live/cloudia_zenn_tutorial/raw_attempt_1_generation_1_initial.txt`
  - `tmp/media_matrix/live/cloudia_zenn_tutorial/raw_attempt_1_generation_2_format_repair.txt`

This run exposed the validator overreach fixed above.
