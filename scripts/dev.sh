#!/usr/bin/env bash
set -euo pipefail

if [ "${NOTE_MAKER_SKIP_ENV:-0}" != "1" ] && [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source ./.env
  set +a
fi

PORT="${PORT:-8080}"
LLM_RUNTIME="${LLM_RUNTIME:-local}"
LLAMACPP_HOST="${LLAMACPP_HOST:-127.0.0.1}"
LLAMACPP_PORT="${LLAMACPP_PORT:-8081}"
LLM_BASE_URL="${LLM_BASE_URL:-http://${LLAMACPP_HOST}:${LLAMACPP_PORT}/v1}"
LLM_MODEL="${LLM_MODEL:-${LLAMACPP_MODEL:-gemma4:31b}}"
STYLE_LLM_MODEL="${STYLE_LLM_MODEL:-${LLM_MODEL}}"
BRIEF_LLM_MODEL="${BRIEF_LLM_MODEL:-${LLM_MODEL}}"
ARTICLE_LLM_MODEL="${ARTICLE_LLM_MODEL:-${LLM_MODEL}}"
DRAFT_LLM_MODEL="${DRAFT_LLM_MODEL:-${LLM_MODEL}}"
LLM_FALLBACK_BASE_URLS="${LLM_FALLBACK_BASE_URLS:-}"
STYLE_LLM_FALLBACK_MODELS="${STYLE_LLM_FALLBACK_MODELS:-}"
BRIEF_LLM_FALLBACK_MODELS="${BRIEF_LLM_FALLBACK_MODELS:-}"
ARTICLE_LLM_FALLBACK_MODELS="${ARTICLE_LLM_FALLBACK_MODELS:-}"
DRAFT_LLM_FALLBACK_MODELS="${DRAFT_LLM_FALLBACK_MODELS:-}"
LLAMACPP_BASE_URL="${LLAMACPP_BASE_URL:-$LLM_BASE_URL}"
LLAMACPP_MODEL="${LLAMACPP_MODEL:-$LLM_MODEL}"
LLAMACPP_HF_REPO="${LLAMACPP_HF_REPO:-ggml-org/gemma-4-31B-it-GGUF}"
LLAMACPP_HF_FILE="${LLAMACPP_HF_FILE:-gemma-4-31B-it-Q4_K_M.gguf}"
LLAMA_SERVER="${LLAMA_SERVER:-llama-server}"

LLAMA_PID=""
APP_PID=""

cleanup() {
  if [ -n "$APP_PID" ] && kill -0 "$APP_PID" 2>/dev/null; then
    kill "$APP_PID" 2>/dev/null || true
  fi
  if [ -n "$LLAMA_PID" ] && kill -0 "$LLAMA_PID" 2>/dev/null; then
    kill "$LLAMA_PID" 2>/dev/null || true
  fi
}

trap cleanup EXIT INT TERM

if [ "$LLM_RUNTIME" = "local" ]; then
  if ! command -v "$LLAMA_SERVER" >/dev/null 2>&1; then
    echo "llama-server was not found. Set LLAMA_SERVER=/path/to/llama-server or install llama.cpp." >&2
    exit 127
  fi

  echo "Starting llama.cpp on ${LLAMACPP_HOST}:${LLAMACPP_PORT} with model alias ${LLAMACPP_MODEL}"
  "$LLAMA_SERVER" \
    --hf-repo "$LLAMACPP_HF_REPO" \
    --hf-file "$LLAMACPP_HF_FILE" \
    --alias "$LLAMACPP_MODEL" \
    --host "$LLAMACPP_HOST" \
    --port "$LLAMACPP_PORT" &
  LLAMA_PID="$!"
else
  echo "Using remote OpenAI-compatible LLM at ${LLM_BASE_URL} with model ${LLM_MODEL}"
fi

echo "Starting Note Maker on http://localhost:${PORT}"
PORT="$PORT" LLM_BASE_URL="$LLM_BASE_URL" LLM_MODEL="$LLM_MODEL" STYLE_LLM_MODEL="$STYLE_LLM_MODEL" BRIEF_LLM_MODEL="$BRIEF_LLM_MODEL" ARTICLE_LLM_MODEL="$ARTICLE_LLM_MODEL" DRAFT_LLM_MODEL="$DRAFT_LLM_MODEL" LLM_FALLBACK_BASE_URLS="$LLM_FALLBACK_BASE_URLS" STYLE_LLM_FALLBACK_MODELS="$STYLE_LLM_FALLBACK_MODELS" BRIEF_LLM_FALLBACK_MODELS="$BRIEF_LLM_FALLBACK_MODELS" ARTICLE_LLM_FALLBACK_MODELS="$ARTICLE_LLM_FALLBACK_MODELS" DRAFT_LLM_FALLBACK_MODELS="$DRAFT_LLM_FALLBACK_MODELS" LLAMACPP_BASE_URL="$LLAMACPP_BASE_URL" LLAMACPP_MODEL="$LLAMACPP_MODEL" go run ./cmd/server &
APP_PID="$!"

echo "Press Ctrl-C to stop both processes."

if [ -n "$LLAMA_PID" ]; then
  while kill -0 "$LLAMA_PID" 2>/dev/null && kill -0 "$APP_PID" 2>/dev/null; do
    sleep 2
  done
else
  while kill -0 "$APP_PID" 2>/dev/null; do
    sleep 2
  done
fi

echo "One process exited; stopping the rest."
