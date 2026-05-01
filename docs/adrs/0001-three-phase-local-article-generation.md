# ADR 0001: Three-phase local article generation workflow

Date: 2026-05-01

## Status

Accepted

## Context

The first local LLM implementation replaced Gemini with an OpenAI-compatible local endpoint and `gemma4:31b`. It can generate paste-ready Markdown, but empirical testing with Terisuke's real note.com articles showed two important limits:

- Passing multiple full articles directly into one generation prompt is too slow for a 31B local model and can time out.
- A single request that fetches articles, infers writing style, collects the new article requirements, and generates the draft mixes several domain concerns.

The scenario test improved from `style_score=56.2` to `style_score=79.8` only after reducing the prompt size and passing a derived style guide. This indicates that stable output requires a persistent author-style asset instead of repeatedly sending raw source articles.

The `/Users/teradakousuke/Developer/polls` repository provides a useful conversation pattern:

- a conversation entity owns phase and message history,
- question-answer pairs are saved explicitly,
- phase detection and question selection are application services,
- prompt construction is isolated from HTTP handlers.

Note Maker should adopt that pattern in a smaller Go implementation.

## Decision

Note Maker will move from a single `POST /api/generate` flow to a three-phase workflow:

1. **Author Style Analysis**
   - Fetch public note.com articles through the Note acquisition adapter.
   - Build a durable `AuthorStyleProfile` and `WritingStyleGuide`.
   - Store source metadata, objective metrics, extracted themes, voice guidance, and representative excerpts.

2. **Article Brief Interview**
   - Ask the user one question at a time about the article to write.
   - Store `ArticleBriefSession`, `ArticleQuestion`, and `BriefAnswer` pairs.
   - Ask deterministic fixed questions first.
   - Then ask deep-dive follow-up questions based on selected fixed-question answers.
   - Detect completion only when the minimum article conditions and required deep dives are satisfied.

3. **Draft Generation**
   - Generate a draft from `WritingStyleGuide + ArticleBrief`.
   - Do not fetch Note articles during draft generation.
   - Validate the draft as paste-ready Markdown and compare it against the author style profile.

These phases are orchestrated by application services, not autonomous background agents. The word "agent" may be used in product language, but the implementation should use deterministic workflow boundaries first.

## Domain Model

New domain concepts:

- `AuthorSource`
  - note username, note article IDs, source URLs, fetched timestamps.
- `AuthorStyleProfile`
  - objective metrics such as paragraph length, sentence length, quote density, first-person usage, keyword frequencies, and heading structure.
- `WritingStyleGuide`
  - stable natural-language guidance derived from the profile: preferred first person, recurring themes, paragraph rhythm, opening patterns, conclusion patterns, and warnings.
- `ArticleBriefSession`
  - interview state, current phase, answered questions, deep-dive state, completion state.
- `ArticleQuestion`
  - fixed or generated question, flow type, required flag, target article field, deep-dive count.
- `BriefAnswer`
  - answer content, target question ID, flow type, follow-up index.
- `ArticleBrief`
  - final structured article requirements: theme, thesis, target reader, purpose, must-include points, exclusions, tone, structure, target length, expected reader action.
- `Draft`
  - paste-ready Markdown plus validation and style-comparison metadata.

## Application Services

Planned services:

- `AnalyzeAuthorStyleService`
  - input: note username or explicit article URLs.
  - output: `AuthorStyleProfile`, `WritingStyleGuide`.

- `ArticleBriefInterviewService`
  - input: current session and latest user answer.
  - output: next question or completed `ArticleBrief`.
  - includes fixed-question selection, deep-dive target selection, and follow-up generation.

- `GenerateDraftService`
  - input: `WritingStyleGuide`, `ArticleBrief`.
  - output: validated `Draft`, comparison report.

- `ArticleWorkflowService`
  - optional facade for UI/API flows that need to coordinate the three services.

## Infrastructure

Adapters remain outside the domain:

- Note acquisition:
  - public page and RSS remain preferred for the product path.
  - note.com JSON APIs may be used in local scenario tests and compatibility adapters where the user explicitly requests them.

- Local LLM:
  - `llama.cpp` `llama-server` remains the documented target.
  - Any OpenAI-compatible local endpoint, including Ollama, can be used for local verification if it exposes the required model alias.

- Storage:
  - initial implementation can use in-memory repositories and JSON file fixtures.
  - durable storage can be added later without changing the domain model.

## API Direction

The existing `POST /api/generate` can remain as a compatibility endpoint, but new flows should use phase-specific endpoints:

- `POST /api/author-style/analyze`
- `GET /api/author-style/{id}`
- `POST /api/brief-sessions`
- `POST /api/brief-sessions/{id}/answers`
- `GET /api/brief-sessions/{id}`
- `POST /api/drafts`

The UI should expose the phases explicitly:

1. collect and summarize author style,
2. interview the user for this article with fixed questions and targeted deep dives,
3. generate and evaluate the draft.

## Interview Flow

The interview phase follows the pattern proven in the `polls` repository:

1. Ask fixed questions in a deterministic order.
2. Save each fixed question and answer as a QA pair.
3. Select deep-dive targets from important answers.
4. Ask one or more follow-up questions tied to the original fixed question.
5. Save follow-up answers separately with `follow_up_index`.
6. Assemble the final `ArticleBrief` from both fixed answers and deep-dive answers.

The first implementation should not allow the LLM to choose arbitrary new topics. Follow-ups must remain anchored to a fixed question and the user's answer. This prevents the interview from drifting and makes the final brief auditable.

## Testing Strategy

The workflow is only accepted when these local tests exist:

- Author style scenario:
  - fetch real Terisuke articles from note.com,
  - build a profile and guide,
  - save a report that can be inspected locally.

- Interview scenario:
  - start a brief session,
  - answer questions one by one,
  - verify fixed-question phase transitions,
  - verify deep-dive follow-up generation,
  - verify final `ArticleBrief`.

- Draft scenario:
  - use a saved `WritingStyleGuide` and `ArticleBrief`,
  - generate with local `gemma4:31b`,
  - validate paste-ready Markdown,
  - compare the draft against the author style profile with stricter thresholds than the first implementation.

Unit tests must not depend on external network or a running local LLM. Integration and scenario tests may require explicit environment variables.

## Consequences

Positive:

- The product becomes easier to reason about in DDD terms.
- Local LLM calls become smaller and more stable.
- The user can inspect and correct the style guide and article brief before generation.
- Tests can verify each phase independently.

Tradeoffs:

- More domain objects and endpoints are needed.
- The UI becomes a workflow rather than a single form.
- Some persistence is required for a useful interview experience.

## Rejected Alternatives

- Keep a single generation endpoint and make the prompt more complex.
  - Rejected because it already showed latency and stability problems.

- Let three autonomous agents communicate freely.
  - Rejected for the first implementation because ownership of state and failure recovery would be ambiguous.

- Always pass all source articles into generation.
  - Rejected because it is slow for local 31B inference and makes output less predictable.
