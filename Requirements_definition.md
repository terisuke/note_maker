# Note Maker Requirements Definition

Version: 2.0
Date: 2026-05-01

## 1. Overview

This application generates Markdown drafts for note.com articles from a note article URL or a note username plus user writing instructions.

The backend no longer uses Google Gemini. Article generation is performed by a local `llama.cpp` `llama-server` process through its OpenAI-compatible API.

The next architecture revision is defined in [ADR 0001](docs/adrs/0001-three-phase-local-article-generation.md) and the [three-phase implementation plan](docs/implementation-plans/three-phase-local-article-generation.md). The product will evolve from a single generation request into:

1. author style analysis,
2. one-question-at-a-time article brief interview,
3. draft generation and style evaluation.

## 2. Runtime

- Backend: Go 1.23+
- Frontend: Static HTML/CSS/JavaScript
- Local LLM runtime: `llama.cpp` `llama-server`
- Model alias: `gemma4:31b`
- Model artifact: `ggml-org/gemma-4-31B-it-GGUF` `gemma-4-31B-it-Q4_K_M.gguf`
- Default LLM base URL: `http://127.0.0.1:8081/v1`

Expected local model startup:

```bash
llama-server \
  --hf-repo ggml-org/gemma-4-31B-it-GGUF \
  --hf-file gemma-4-31B-it-Q4_K_M.gguf \
  --alias gemma4:31b \
  --host 127.0.0.1 \
  --port 8081
```

## 3. API Contract

### `POST /api/generate`

Request JSON:

```json
{
  "note_url": "string",
  "username": "string",
  "keywords": ["string"],
  "theme": "string",
  "target_audience": "string",
  "exclusions": "string",
  "style_choice": "string",
  "tone_choice": "string",
  "word_count": 1500,
  "article_purpose": "string",
  "desired_content": "string",
  "introduction_points": "string",
  "main_points": "string",
  "conclusion_message": "string"
}
```

Validation:

- `note_url` or `username` is required.
- `theme` is required.
- `style_choice` defaults to `ですます調`.
- `tone_choice` defaults to `客観的`.
- `word_count` defaults to `1500`.

Success response:

```json
{
  "draft": "Markdown article draft"
}
```

The returned `draft` must be paste-ready Markdown:

- It starts with a single `# title` line.
- It contains no assistant preamble, reasoning text, or code fence wrapper.
- It uses `##` section headings and a natural introduction/body/conclusion flow.
- It reflects the user's theme, purpose, desired content, and reference material without copying the source article verbatim.

Error response:

```json
{
  "error": {
    "code": "ARTICLE_GENERATION_FAILED",
    "message": "Failed to generate article",
    "details": "optional details"
  }
}
```

### `GET /api/models`

Returns the model IDs exposed by the configured `llama-server` `/v1/models` endpoint.

Example:

```json
["gemma4:31b"]
```

## 4. Content Acquisition

The Note acquisition strategy is:

1. Fetch the public article page and extract title/body from public HTML.
2. For usernames, fetch the public RSS feed and then fetch linked public articles.
3. Use undocumented note.com API endpoints only as compatibility fallback.

The undocumented endpoints are not considered stable public contracts. They can change without notice and may be restricted by note.com policies or robots rules.

## 5. Architecture

The Go backend is split into:

- `internal/domain`: request and article domain types.
- `internal/domain`: draft normalization and paste-ready Markdown validation.
- `internal/application`: generation orchestration and prompt construction.
- `internal/infrastructure`: adapters for `llama.cpp` and Note acquisition.
- `internal/handlers`: HTTP JSON request/response handling.

Handlers must not construct prompts or call concrete remote APIs directly except through application services and infrastructure adapters.

The planned three-phase workflow introduces separate domain boundaries for author style, article brief interviews, and draft generation. The existing `POST /api/generate` endpoint remains a compatibility path while the new workflow is implemented.

## 6. Testing

Required test coverage:

- llama.cpp client request/response parsing using `httptest.Server`.
- application generation flow using fake source fetcher and fake text generator.
- Note article and RSS fetch behavior using mocked HTTP transport.
- `/api/generate` validation and success responses using injected fake application service.

Standard verification:

```bash
go test ./...
go build ./cmd/server
```

## 7. Operational Notes

- `GEMINI_API_KEY` is no longer used.
- `LLAMACPP_BASE_URL` and `LLAMACPP_MODEL` configure local generation.
- Gemma4 31B Q4_K_M is approximately 18.7GB and may require significant RAM/VRAM and startup time.
- Generated drafts must be reviewed by a human before publishing.
