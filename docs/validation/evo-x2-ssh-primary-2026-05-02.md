# Evo X2 SSH primary validation - 2026-05-02

Tracks Issue [#35](https://github.com/terisuke/note_maker/issues/35).

## Summary

Evo X2 over the Tailscale SSH tunnel is the correct primary inference path. It passes the full workflow scenario with the intended `gemma4:31b` draft model.

Local llama.cpp is available as a fallback runtime, but the local fallback model set did not meet the draft quality threshold in this run. Follow-up hardening is tracked by Issue [#36](https://github.com/terisuke/note_maker/issues/36).

## Evo X2 primary

Command:

```bash
/usr/bin/time -p make scenario-evo-x2
```

Runtime:

- Base URL: `http://127.0.0.1:21434/v1`
- Transport: Tailscale SSH tunnel to `evo-x2:127.0.0.1:11434`
- Draft model: `gemma4:31b`
- Style model: `gemma4:latest`
- Brief model: `gemma4:e2b`

Result:

- Passed: `true`
- Style score: `89.2`
- Draft length: `2813` runes
- Elapsed: `749.13s`

The preflight confirmed the SSH tunnel before the scenario:

```text
Evo X2 SSH LLM tunnel is ready: http://127.0.0.1:21434/v1/models
```

## Local llama.cpp fallback

The local fallback test used the local `llama-server` binary:

```text
/Users/teradakousuke/.docker/bin/inference/llama-server
```

`gemma4:31b` could not be validated through local llama.cpp using the existing Ollama blob. The available `llama-server` failed to load it:

```text
wrong number of tensors; expected 1189, got 833
```

`qwen3:30b-a3b` did load through local llama.cpp with Metal offload and `--reasoning off`.

Command:

```bash
/usr/bin/time -p env RUN_NOTE_SCENARIO=1 RUN_LOCAL_LLM_SCENARIO=1 LLM_BASE_URL=http://127.0.0.1:8081/v1 LLM_MODEL=qwen3:30b-a3b STYLE_LLM_MODEL=qwen3:30b-a3b BRIEF_LLM_MODEL=qwen3:30b-a3b ARTICLE_LLM_MODEL=qwen3:30b-a3b DRAFT_LLM_MODEL=qwen3:30b-a3b LLM_TIMEOUT_SECONDS=900 SCENARIO_MIN_STYLE_SCORE=80 SCENARIO_MIN_DRAFT_RUNES=2800 DRAFT_MAX_ATTEMPTS=2 go run ./cmd/scenario/full_workflow
```

Result:

- Passed: `false`
- Style score: `76.6`
- Candidate length: `932` chars in evaluation, `952` chars on disk
- Elapsed: `582.02s`
- Main failures: `total_style_score=76.6 below 82.0`, `keyword_overlap=54 below 70`

`qwen3.5:27b` also could not be validated through the available local `llama-server`:

```text
qwen35.rope.dimension_sections has wrong array length; expected 4, got 3
```

## Decision

- Keep Evo X2 over Tailscale SSH as the default primary path.
- Keep local llama.cpp as fallback only.
- Do not treat local fallback as production-quality until Issue [#36](https://github.com/terisuke/note_maker/issues/36) finds a local llama.cpp-compatible model and server flag set that passes the strict draft thresholds.
