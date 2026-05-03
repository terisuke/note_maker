# Next implementation cut: app baseline to fallback hardening

Date: 2026-05-03
Baseline branch: `develop`
Latest app cut: PR #87, merge commit `4af05cadaeacc5fa10bdd22aa4d5992200c7d8ba`

This document replaces the older Phase C polish plan. The app-like launcher,
queryable workflow memory, persona/history cards, browser E2E, and live
publishing-target matrix are now part of the `develop` baseline.

For operational handoff details, see
[Note Maker app handoff](../handoffs/app-handoff-2026-05-03.md).

## Current state

Implemented and merged:

- Phase A conversation UX:
  - [#17](https://github.com/terisuke/note_maker/issues/17) chat transcript and editable answers.
  - [#18](https://github.com/terisuke/note_maker/issues/18) SSE streaming, heartbeat, and cancellation.
  - [#19](https://github.com/terisuke/note_maker/issues/19) editable draft Markdown and section regeneration.
  - [#20](https://github.com/terisuke/note_maker/issues/20) deep-dive rationale in prompt and UI.
- Phase B persona / format:
  - [#21](https://github.com/terisuke/note_maker/issues/21) Persona and OutputFormat domain concepts.
  - [#22](https://github.com/terisuke/note_maker/issues/22) source fetching for note, Zenn, Qiita, RSS, HTML, and Cor GitHub Markdown.
  - [#23](https://github.com/terisuke/note_maker/issues/23) format prompt templates and validators.
  - [#24](https://github.com/terisuke/note_maker/issues/24) `terisuke` and `cloudia` persona seeds.
  - [#25](https://github.com/terisuke/note_maker/issues/25) persona/format-aware question templates and media matrix.
- Phase C persistence and product memory:
  - [#26](https://github.com/terisuke/note_maker/issues/26) SQLite workflow store.
  - [#27](https://github.com/terisuke/note_maker/issues/27) saved history and persona/session picker UI.
  - [#28](https://github.com/terisuke/note_maker/issues/28) human-readable cards for guides, briefs, projects, articles, drafts, and source snapshots.
  - [#14](https://github.com/terisuke/note_maker/issues/14) queryable product-memory umbrella: closed by PR #87 after custom persona update/delete and explicit brief versions landed.
- Quality and runtime:
  - [#11](https://github.com/terisuke/note_maker/issues/11) strict style thresholds.
  - [#13](https://github.com/terisuke/note_maker/issues/13) browser E2E.
  - [#29](https://github.com/terisuke/note_maker/issues/29) handler coverage.
  - [#40](https://github.com/terisuke/note_maker/issues/40), [#57](https://github.com/terisuke/note_maker/issues/57), [#70](https://github.com/terisuke/note_maker/issues/70)-[#74](https://github.com/terisuke/note_maker/issues/74) Evo X2 live media-matrix stabilization.
- App packaging baseline:
  - [#15](https://github.com/terisuke/note_maker/issues/15) pragmatic app-like launcher: closed by PR #87.

Open and active:

| Issue | Status | Next condition |
|---|---|---|
| [#36](https://github.com/terisuke/note_maker/issues/36) | Open | Start an approved workstation-local llama.cpp endpoint and record a live passing fallback report with `score >= 82`, `keyword_overlap >= 70`, and `runes >= 2800`. |
| [#45](https://github.com/terisuke/note_maker/issues/45) | Open | Improve Evo X2 llama.cpp `/llama/v1` first-token latency and keyword-overlap consistency while keeping Ollama primary healthy. |

## Current validation baseline

Fast local checks:

```bash
go test ./...
python3 -m pytest tests/e2e -q
./scripts/check-launcher.sh
git diff --check
```

Runtime checks:

```bash
make launcher-status
make scenario-local-llamacpp-fallback
RUN_EVO_X2_LLAMA_CPP_SCENARIO=1 make scenario-evo-x2-llama-brief-draft
make evo-x2-models
```

The #45 direct `/llama/v1` run on 2026-05-03 completed through the fallback
route with `scenario_passed=true`, score `88.2`, `3075` runes, and final
verification passed. It still missed promotion gates: `keyword_overlap=65/70`
and `first_chunk_ms=14105`.

The #36 local fallback command currently records plan-only output because no
loopback llama.cpp endpoint is listening on `127.0.0.1:8081`.

## Recommended next cut

Do not reopen broad Phase A/B/C/D scope. The next implementation should be a
small runtime hardening cut:

1. Pick either #36 or #45, not both, for the next PR.
2. Keep Evo X2 Ollama as primary.
3. Preserve explicit live gates for any LLM scenario.
4. Record base URL, model, elapsed seconds, first chunk, chunks, score, keyword overlap, runes, and verification result.
5. Update the corresponding validation doc before closing the issue.

Recommended order:

1. #36 if a safe workstation-local llama.cpp endpoint is available.
2. #45 if Evo X2 llama.cpp service/profile details and latency tuning are available.
3. Native packaging follow-up only after a user explicitly asks for a signed wrapper or installer.

## Main promotion readiness

The app baseline is ready for a develop-to-main PR when:

- `develop` contains PR #87 and this docs handoff update,
- open issues are expected follow-ups only (#36 and #45),
- fast local checks pass,
- GitHub checks on the docs PR pass.

Use the handoff checklist in
[Note Maker app handoff](../handoffs/app-handoff-2026-05-03.md#main-promotion-checklist)
for the main PR.
