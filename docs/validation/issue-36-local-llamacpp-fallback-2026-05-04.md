# Issue #36 local llama.cpp fallback follow-up

Date: 2026-05-04
Branch: `codex/close-runtime-desktop-deployment`

## Source fix

The local fallback scenario previously fed scripted brief answers as an ordered
slice. The fixed interview template has more than the required answer fields, so
the scripted values shifted after `reader`; `must_include`, `exclusions`,
`target_length_structure`, and `tone_stance` could contain the wrong answers.

This cut maps scripted answers by fixed question ID, adds coverage for the
mapping, forwards the strict keyword gate into draft generation, and adds local
fallback generation controls for max tokens, temperature, and top-p.

## Validation commands

```sh
go test ./cmd/scenario/brief_interview ./cmd/scenario/draft_generation ./cmd/scenario/local_llamacpp_fallback ./internal/application/draft ./internal/infrastructure/llamacpp
go test ./...
```

Result: passed.

## Corrected brief evidence

After the fix, `tmp/local_llamacpp_fallback_retry/brief_interview/brief.json`
contained the intended fields:

```text
MustInclude=Note APIで記事を集めること、文体ガイドと一問一答を分けること、深掘り質問で記事の核を作ること。音楽家からエンジニア、起業、LT登壇、AI駆動開発という自分の文脈も自然に接続したい
Exclusions=ローカルLLMを万能だと断言すること、根拠のない性能比較、Gemini依存
TargetLengthStructure=3000字前後、最低2800字。導入、違和感、設計変更、Evo X2でのモデル使い分け、実装と検証、読者への提案、結論で構成する
ToneStance=内省的だが技術検証の具体性もある。僕という一人称で、音楽やLTの経験も比喩として使いながら、読者に問いかける調子にする
```

## Live local model attempts

Local server:

```sh
/opt/homebrew/bin/llama-server \
  -m /Users/teradakousuke/.lmstudio/models/lmstudio-community/gemma-3-27b-it-GGUF/gemma-3-27b-it-Q8_0.gguf \
  --host 127.0.0.1 --port 8081 --alias local-gemma3:27b \
  --ctx-size 8192 --n-gpu-layers 999 --jinja --flash-attn auto
```

Corrected streaming run:

```sh
RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 \
LOCAL_LLAMACPP_FALLBACK_BASE_URL=http://127.0.0.1:8081/v1 \
LOCAL_LLAMACPP_FALLBACK_MODEL=local-gemma3:27b \
LOCAL_LLAMACPP_FALLBACK_VERIFY_MODEL=local-gemma3:27b \
LOCAL_LLAMACPP_FALLBACK_OUTPUT_DIR=tmp/local_llamacpp_fallback_retry \
LOCAL_LLAMACPP_FALLBACK_TIMEOUT_SECONDS=1200 \
make scenario-local-llamacpp-fallback
```

Result: failed in draft generation with `read llama.cpp stream: unexpected EOF`
after about 72 seconds.

Non-streaming quality check:

```sh
RUN_LOCAL_LLAMACPP_FALLBACK_SCENARIO=1 \
LOCAL_LLAMACPP_FALLBACK_BASE_URL=http://127.0.0.1:8081/v1 \
LOCAL_LLAMACPP_FALLBACK_MODEL=local-gemma3:27b \
LOCAL_LLAMACPP_FALLBACK_VERIFY_MODEL=local-gemma3:27b \
LOCAL_LLAMACPP_FALLBACK_OUTPUT_DIR=tmp/local_llamacpp_fallback_nostream \
LOCAL_LLAMACPP_FALLBACK_MAX_ATTEMPTS=1 \
LOCAL_LLAMACPP_FALLBACK_STREAM_DRAFT=0 \
make scenario-local-llamacpp-fallback
```

Result: failed in draft generation with `call llama.cpp: ... EOF` after about
218 seconds. The server process was stopped after the run.

## Close assessment

Do not close #36 yet. The scenario inputs are now corrected and the local
fallback runner records stricter gates, but the currently available Gemma 3 27B
GGUF local server still does not complete a passing draft run. The issue still
needs a local llama.cpp-compatible model/config that records `status=passed`,
`score >= 82`, `keyword_overlap >= 70`, and `runes >= 2800` without Ollama local
API or Evo X2.
