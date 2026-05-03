# Issue 20 deep-dive rationale validation - 2026-05-02

Tracks Issue [#20](https://github.com/terisuke/note_maker/issues/20).

## Code checks

```bash
go test ./...
node --check static/js/script.js
git diff --check
```

Result: pass.

## Scenario variant

This run intentionally differs from the prior note-oriented validation:

- Persona: `terisuke`
- Output format: `markdown_blog`
- Brief variant: `cor_blog`
- Topic: Evo X2 local LLM inference foundation as company technical knowledge and internal vision sharing
- Primary runtime: `http://evo-x2.tailb30e58.ts.net/v1`
- Draft model: `gemma4:31b`
- Verification model: `gemma4:latest`

The goal was not to re-run the exact same note essay. It was to start building per-phase averages across different media and writing modes.

## Full workflow attempt

```bash
env SCENARIO_BRIEF_VARIANT=cor_blog SCENARIO_OUTPUT_FORMAT_ID=markdown_blog SCENARIO_MIN_DRAFT_RUNES=2200 make scenario-evo-x2
```

Preflight passed and confirmed the Tailnet OpenAI-compatible API:

```text
Evo X2 Tailnet LLM API is ready: http://evo-x2.tailb30e58.ts.net/v1/models
```

Brief interview completed:

```text
variant=cor_blog
persona_id=terisuke
output_format_id=markdown_blog
answers=13
deep_dives=4
elapsed_seconds=240.01
```

The draft stage failed the strict style gate:

```text
style score 63.1 below scenario minimum 80.0
```

Artifacts still showed a usable draft length:

- Draft file: `tmp/draft_generation/draft.md`
- Characters: `3314`
- Style score: `63.1`
- Main failures: `sentence_length=56 below 75`, `first_person=48 below 60`
- Verification: failed because the old scenario timeout let the verifier reach an expired context and report the workstation-local fallback endpoint.

This exposed two scenario-harness problems fixed in the same PR:

- `cmd/scenario/draft_generation` now prints metrics before exiting non-zero.
- `llamacpp.Client` no longer falls through to fallback clients after the caller context has expired.

## Draft-only rerun

The existing company-blog brief was reused and local workstation fallback was intentionally removed from the chain:

```bash
env \
  RUN_LOCAL_LLM_SCENARIO=1 \
  SCENARIO_STREAM_DRAFT=1 \
  SCENARIO_OUTPUT_DIR=tmp/draft_generation_issue20 \
  AUTHOR_PROFILE_PATH=tmp/author_style/profile.json \
  WRITING_GUIDE_PATH=tmp/author_style/guide.json \
  ARTICLE_BRIEF_PATH=tmp/brief_interview/brief.json \
  LLM_BASE_URL=http://evo-x2.tailb30e58.ts.net/v1 \
  LLM_MODEL=gemma4:31b \
  DRAFT_LLM_MODEL=gemma4:31b \
  VERIFY_LLM_MODEL=gemma4:latest \
  LLM_TIMEOUT_SECONDS=1200 \
  SCENARIO_DRAFT_TIMEOUT_SECONDS=1200 \
  LLM_FALLBACK_BASE_URLS=http://evo-x2.tailb30e58.ts.net/llama/v1 \
  DRAFT_LLM_FALLBACK_MODELS=gemma-4-E2B-it-Q8_0.gguf \
  VERIFY_LLM_FALLBACK_MODELS=gemma-4-E2B-it-Q8_0.gguf \
  SCENARIO_MIN_STYLE_SCORE=60 \
  SCENARIO_MIN_DRAFT_RUNES=2800 \
  DRAFT_MAX_ATTEMPTS=1 \
  go run ./cmd/scenario/draft_generation
```

Result:

```text
scenario_passed=true
attempt=1
passed=false
score=72.3
min_style_score=60.0
runes=3675
min_draft_runes=2800
verification_performed=true
verification_passed=true
elapsed_seconds=439.86
streaming=true
first_chunk_ms=31573
chunks=1778
llm_base_url=http://evo-x2.tailb30e58.ts.net/v1
llm_model=gemma4:31b
verify_model=gemma4:latest
```

Interpretation:

- Tailnet primary path was reachable and used for the scenario.
- The draft exceeded the 2800-character gate.
- Lightweight final verification passed.
- Strict Terisuke-note style score remained below 82 because this scenario deliberately used a company-blog format with the existing note-derived profile. This is useful signal: cross-medium validation should track a separate baseline rather than treating note-style thresholds as universal.

