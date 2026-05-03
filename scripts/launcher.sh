#!/usr/bin/env bash
set -euo pipefail

NM_APP_PID=""
NM_LLAMA_PID=""

nm_repo_root() {
  cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P
}

nm_log() {
  printf '[note-maker] %s\n' "$*" >&2
}

nm_warn() {
  printf '[note-maker] WARN: %s\n' "$*" >&2
}

nm_die() {
  printf '[note-maker] ERROR: %s\n' "$*" >&2
  exit 1
}

nm_usage() {
  cat <<'EOF'
Usage: scripts/launcher.sh [options]

Starts Note Maker as an app-like local launcher.

Options:
  --port PORT          Preferred app port. The launcher selects the next free
                       port unless --strict-port is also set.
  --strict-port        Fail instead of selecting the next free port.
  --data-dir PATH      User data directory for config, workflow store, and logs.
  --no-open            Do not open the app URL in the browser.
  --allow-degraded     Start the app even when Evo X2 Tailnet health fails.
  --local-llm          Explicitly start local llama.cpp and use it as primary.
  --status             Print launcher health/config status without starting.
  -h, --help           Show this help.

Environment:
  NOTE_MAKER_DATA_DIR             Overrides the default app data directory.
  NOTE_MAKER_OPEN_BROWSER=0       Same as --no-open.
  NOTE_MAKER_REQUIRE_EVO_X2=0     Same as --allow-degraded.
  NOTE_MAKER_START_LOCAL_LLM=1    Same as --local-llm.
EOF
}

nm_load_env() {
  local root="$1"
  if [ "${NOTE_MAKER_SKIP_ENV:-0}" != "1" ] && [ -f "${root}/.env" ]; then
    set -a
    # shellcheck disable=SC1091
    source "${root}/.env"
    set +a
  fi
}

nm_default_data_dir() {
  if [ -n "${NOTE_MAKER_DATA_DIR:-}" ]; then
    printf '%s\n' "$NOTE_MAKER_DATA_DIR"
    return 0
  fi

  local home="${HOME:-}"
  if [ -z "$home" ]; then
    printf '%s\n' "$(nm_repo_root)/data"
    return 0
  fi

  case "$(uname -s)" in
    Darwin)
      printf '%s\n' "${home}/Library/Application Support/Note Maker"
      ;;
    *)
      if [ -n "${XDG_DATA_HOME:-}" ]; then
        printf '%s\n' "${XDG_DATA_HOME}/note-maker"
      else
        printf '%s\n' "${home}/.local/share/note-maker"
      fi
      ;;
  esac
}

nm_port_available() {
  local port="$1"

  if command -v python3 >/dev/null 2>&1; then
    python3 - "$port" <<'PY' >/dev/null 2>&1
import socket
import sys

port = int(sys.argv[1])
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
try:
    sock.bind(("127.0.0.1", port))
except OSError:
    sys.exit(1)
finally:
    sock.close()
PY
    return $?
  fi

  if command -v lsof >/dev/null 2>&1; then
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      return 1
    fi
    return 0
  fi

  nm_warn "python3 and lsof are unavailable; assuming port ${port} is free"
  return 0
}

nm_find_available_port() {
  local requested="$1"
  local strict="$2"
  local port="$requested"
  local attempts=50

  if [ "$strict" = "1" ]; then
    if nm_port_available "$port"; then
      printf '%s\n' "$port"
      return 0
    fi
    return 1
  fi

  while [ "$attempts" -gt 0 ]; do
    if nm_port_available "$port"; then
      printf '%s\n' "$port"
      return 0
    fi
    port=$((port + 1))
    attempts=$((attempts - 1))
  done

  return 1
}

nm_models_url() {
  local base_url="$1"
  printf '%s/models\n' "${base_url%/}"
}

nm_http_ok() {
  local url="$1"
  local timeout="$2"

  if ! command -v curl >/dev/null 2>&1; then
    return 1
  fi

  curl -fsS --max-time "$timeout" "$url" >/dev/null 2>&1
}

nm_print_llm_health() {
  local label="$1"
  local base_url="$2"
  local timeout="$3"
  local models_url
  models_url="$(nm_models_url "$base_url")"

  if nm_http_ok "$models_url" "$timeout"; then
    nm_log "${label}: ready (${models_url})"
    return 0
  fi

  nm_warn "${label}: not reachable (${models_url})"
  return 1
}

nm_wait_http() {
  local label="$1"
  local url="$2"
  local timeout="$3"
  local start="$SECONDS"

  while [ $((SECONDS - start)) -lt "$timeout" ]; do
    if nm_http_ok "$url" 2; then
      nm_log "${label}: ready (${url})"
      return 0
    fi
    sleep 1
  done

  return 1
}

nm_open_browser() {
  local url="$1"

  case "$(uname -s)" in
    Darwin)
      open "$url" >/dev/null 2>&1 || nm_warn "Could not open browser; visit ${url}"
      ;;
    Linux)
      if command -v xdg-open >/dev/null 2>&1; then
        xdg-open "$url" >/dev/null 2>&1 || nm_warn "Could not open browser; visit ${url}"
      else
        nm_log "Open ${url}"
      fi
      ;;
    *)
      nm_log "Open ${url}"
      ;;
  esac
}

nm_stop_pid() {
  local pid="$1"
  local label="$2"
  local attempts=20

  if [ -z "$pid" ] || ! kill -0 "$pid" 2>/dev/null; then
    return 0
  fi

  nm_log "Stopping ${label} (pid ${pid})"
  kill "$pid" 2>/dev/null || true

  while [ "$attempts" -gt 0 ]; do
    if ! kill -0 "$pid" 2>/dev/null; then
      wait "$pid" 2>/dev/null || true
      return 0
    fi
    sleep 1
    attempts=$((attempts - 1))
  done

  nm_warn "${label} did not stop after SIGTERM; sending SIGKILL"
  kill -9 "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
}

nm_cleanup() {
  nm_stop_pid "${NM_APP_PID:-}" "Note Maker server"
  nm_stop_pid "${NM_LLAMA_PID:-}" "local llama.cpp"
  NM_APP_PID=""
  NM_LLAMA_PID=""
}

nm_start_local_llm() {
  local log_file="$1"

  if ! command -v "$LLAMA_SERVER" >/dev/null 2>&1; then
    nm_die "llama-server was not found. Set LLAMA_SERVER=/path/to/llama-server or install llama.cpp."
  fi

  nm_log "Starting local llama.cpp on ${LLAMACPP_HOST}:${LLAMACPP_PORT}; log: ${log_file}"
  "$LLAMA_SERVER" \
    --hf-repo "$LLAMACPP_HF_REPO" \
    --hf-file "$LLAMACPP_HF_FILE" \
    --alias "$LLAMACPP_MODEL" \
    --host "$LLAMACPP_HOST" \
    --port "$LLAMACPP_PORT" \
    >"$log_file" 2>&1 &
  printf '%s\n' "$!"
}

nm_build_server() {
  local root="$1"
  local bin_dir="$2"
  local bin_path="${bin_dir}/note-maker-server"

  mkdir -p "$bin_dir"
  nm_log "Building server binary: ${bin_path}"
  (cd "$root" && go build -o "$bin_path" ./cmd/server)
  printf '%s\n' "$bin_path"
}

nm_start_server() {
  local root="$1"
  local bin_path="$2"
  local log_file="$3"

  nm_log "Starting Note Maker server; log: ${log_file}"
  (
    cd "$root"
    exec "$bin_path"
  ) >"$log_file" 2>&1 &
  printf '%s\n' "$!"
}

nm_split_and_report_fallbacks() {
  local fallbacks="$1"
  local timeout="$2"
  local item
  local old_ifs="$IFS"

  if [ -z "$fallbacks" ]; then
    return 0
  fi

  IFS=','
  for item in $fallbacks; do
    item="${item#"${item%%[![:space:]]*}"}"
    item="${item%"${item##*[![:space:]]}"}"
    if [ -n "$item" ]; then
      nm_print_llm_health "Fallback LLM" "$item" "$timeout" || true
    fi
  done
  IFS="$old_ifs"
}

nm_main() {
  local root
  root="$(nm_repo_root)"

  local requested_port=""
  local strict_port="${NOTE_MAKER_STRICT_PORT:-0}"
  local open_browser="${NOTE_MAKER_OPEN_BROWSER:-1}"
  local require_evo_x2="${NOTE_MAKER_REQUIRE_EVO_X2:-1}"
  local local_llm="${NOTE_MAKER_START_LOCAL_LLM:-0}"
  local status_only=0
  local data_dir_arg=""

  while [ "$#" -gt 0 ]; do
    case "$1" in
      --port)
        [ "$#" -ge 2 ] || nm_die "--port requires a value"
        requested_port="$2"
        shift 2
        ;;
      --strict-port)
        strict_port=1
        shift
        ;;
      --data-dir)
        [ "$#" -ge 2 ] || nm_die "--data-dir requires a value"
        data_dir_arg="$2"
        shift 2
        ;;
      --no-open)
        open_browser=0
        shift
        ;;
      --allow-degraded)
        require_evo_x2=0
        shift
        ;;
      --local-llm)
        local_llm=1
        shift
        ;;
      --status)
        status_only=1
        open_browser=0
        require_evo_x2=0
        shift
        ;;
      -h|--help)
        nm_usage
        return 0
        ;;
      *)
        nm_die "unknown option: $1"
        ;;
    esac
  done

  nm_load_env "$root"

  if [ -n "$data_dir_arg" ]; then
    NOTE_MAKER_DATA_DIR="$data_dir_arg"
  fi

  local data_dir
  data_dir="$(nm_default_data_dir)"
  mkdir -p "$data_dir" "${data_dir}/logs"

  PORT="${requested_port:-${PORT:-8080}}"
  if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
    nm_die "PORT must be a number from 1 to 65535; got ${PORT}"
  fi

  local selected_port
  if ! selected_port="$(nm_find_available_port "$PORT" "$strict_port")"; then
    nm_die "port ${PORT} is not available"
  fi
  if [ "$selected_port" != "$PORT" ]; then
    nm_warn "Port ${PORT} is busy; selected ${selected_port}"
  fi
  PORT="$selected_port"

  EVO_X2_TAILNET_HOST="${EVO_X2_TAILNET_HOST:-evo-x2.tailb30e58.ts.net}"
  EVO_X2_OLLAMA_LLM_BASE_URL="${EVO_X2_OLLAMA_LLM_BASE_URL:-http://${EVO_X2_TAILNET_HOST}/v1}"
  EVO_X2_LLAMA_CPP_LLM_BASE_URL="${EVO_X2_LLAMA_CPP_LLM_BASE_URL:-http://${EVO_X2_TAILNET_HOST}/llama/v1}"
  EVO_X2_LLM_BASE_URL="${EVO_X2_LLM_BASE_URL:-${EVO_X2_OLLAMA_LLM_BASE_URL}}"
  LLAMACPP_HOST="${LLAMACPP_HOST:-127.0.0.1}"
  LLAMACPP_PORT="${LLAMACPP_PORT:-8081}"
  LLAMACPP_MODEL="${LLAMACPP_MODEL:-gemma4:31b}"
  LLAMACPP_HF_REPO="${LLAMACPP_HF_REPO:-ggml-org/gemma-4-31B-it-GGUF}"
  LLAMACPP_HF_FILE="${LLAMACPP_HF_FILE:-gemma-4-31B-it-Q4_K_M.gguf}"
  LLAMA_SERVER="${LLAMA_SERVER:-llama-server}"

  if [ "$local_llm" = "1" ]; then
    LLM_RUNTIME="local"
    LLM_BASE_URL="${NOTE_MAKER_LOCAL_LLM_BASE_URL:-http://${LLAMACPP_HOST}:${LLAMACPP_PORT}/v1}"
  else
    if [ "${LLM_RUNTIME:-}" = "local" ]; then
      nm_warn "LLM_RUNTIME=local is set, but local llama.cpp starts only with --local-llm or NOTE_MAKER_START_LOCAL_LLM=1"
    fi
    LLM_RUNTIME="remote"
    LLM_BASE_URL="${LLM_BASE_URL:-${EVO_X2_LLM_BASE_URL}}"
  fi

  LLM_MODEL="${LLM_MODEL:-${LLAMACPP_MODEL}}"
  STYLE_LLM_MODEL="${STYLE_LLM_MODEL:-gemma4:e2b}"
  BRIEF_LLM_MODEL="${BRIEF_LLM_MODEL:-qwen3.6:27b}"
  ARTICLE_LLM_MODEL="${ARTICLE_LLM_MODEL:-gemma4:e2b}"
  DRAFT_LLM_MODEL="${DRAFT_LLM_MODEL:-${LLM_MODEL}}"
  VERIFY_LLM_MODEL="${VERIFY_LLM_MODEL:-gemma4:latest}"
  LLM_FALLBACK_BASE_URLS="${LLM_FALLBACK_BASE_URLS:-${EVO_X2_LLAMA_CPP_LLM_BASE_URL},http://${LLAMACPP_HOST}:${LLAMACPP_PORT}/v1}"
  STYLE_LLM_FALLBACK_MODELS="${STYLE_LLM_FALLBACK_MODELS:-}"
  BRIEF_LLM_FALLBACK_MODELS="${BRIEF_LLM_FALLBACK_MODELS:-}"
  ARTICLE_LLM_FALLBACK_MODELS="${ARTICLE_LLM_FALLBACK_MODELS:-}"
  DRAFT_LLM_FALLBACK_MODELS="${DRAFT_LLM_FALLBACK_MODELS:-}"
  VERIFY_LLM_FALLBACK_MODELS="${VERIFY_LLM_FALLBACK_MODELS:-}"
  LLAMACPP_BASE_URL="${LLAMACPP_BASE_URL:-${LLM_BASE_URL}}"

  NOTE_MAKER_CONFIG_PATH="${NOTE_MAKER_CONFIG_PATH:-${data_dir}/app_config.json}"
  WORKFLOW_STORE_PATH="${WORKFLOW_STORE_PATH:-${data_dir}/workflow_store.json}"

  export PORT LLM_RUNTIME LLM_BASE_URL LLM_MODEL
  export STYLE_LLM_MODEL BRIEF_LLM_MODEL ARTICLE_LLM_MODEL DRAFT_LLM_MODEL VERIFY_LLM_MODEL
  export LLM_FALLBACK_BASE_URLS STYLE_LLM_FALLBACK_MODELS BRIEF_LLM_FALLBACK_MODELS
  export ARTICLE_LLM_FALLBACK_MODELS DRAFT_LLM_FALLBACK_MODELS VERIFY_LLM_FALLBACK_MODELS
  export LLAMACPP_BASE_URL LLAMACPP_MODEL NOTE_MAKER_CONFIG_PATH WORKFLOW_STORE_PATH

  nm_log "Data directory: ${data_dir}"
  nm_log "Config path: ${NOTE_MAKER_CONFIG_PATH}"
  nm_log "Workflow store path: ${WORKFLOW_STORE_PATH}"
  nm_log "App port: ${PORT}"
  nm_log "Primary LLM runtime: ${LLM_RUNTIME} (${LLM_BASE_URL}, model ${LLM_MODEL})"

  local health_timeout="${NOTE_MAKER_HEALTH_TIMEOUT_SECONDS:-5}"
  local evo_ready=0
  if nm_print_llm_health "Evo X2 Tailnet primary" "$EVO_X2_LLM_BASE_URL" "$health_timeout"; then
    evo_ready=1
  fi
  nm_split_and_report_fallbacks "$LLM_FALLBACK_BASE_URLS" "$health_timeout"

  if [ "$local_llm" != "1" ] && [ "$require_evo_x2" = "1" ] && [ "$evo_ready" != "1" ]; then
    cat >&2 <<EOF

Evo X2 Tailnet is not reachable, so the launcher did not start the app.
Use --allow-degraded to start the UI without a healthy primary LLM, or
use --local-llm / NOTE_MAKER_START_LOCAL_LLM=1 to explicitly start local llama.cpp.
EOF
    return 1
  fi

  if [ "$status_only" = "1" ]; then
    return 0
  fi

  local timestamp
  timestamp="$(date +%Y%m%d-%H%M%S)"
  local app_log="${data_dir}/logs/server-${timestamp}.log"
  local llama_log="${data_dir}/logs/llama-${timestamp}.log"
  local bin_path="${data_dir}/note-maker-server"

  trap nm_cleanup EXIT
  trap 'nm_cleanup; exit 130' INT
  trap 'nm_cleanup; exit 143' TERM

  if [ "$local_llm" = "1" ]; then
    NM_LLAMA_PID="$(nm_start_local_llm "$llama_log")"
    if ! nm_wait_http "Local llama.cpp" "$(nm_models_url "$LLM_BASE_URL")" "${NOTE_MAKER_LOCAL_LLM_WAIT_SECONDS:-120}"; then
      nm_die "local llama.cpp did not become ready; see ${llama_log}"
    fi
  fi

  bin_path="$(nm_build_server "$root" "$data_dir")"
  NM_APP_PID="$(nm_start_server "$root" "$bin_path" "$app_log")"

  local app_url="http://127.0.0.1:${PORT}"
  if ! nm_wait_http "Note Maker" "$app_url" "${NOTE_MAKER_SERVER_WAIT_SECONDS:-20}"; then
    nm_die "server did not become ready; see ${app_log}"
  fi

  nm_log "Launcher ready: ${app_url}"
  if [ "$open_browser" = "1" ]; then
    nm_open_browser "$app_url"
  fi

  nm_log "Press Ctrl-C to stop Note Maker."
  while kill -0 "$NM_APP_PID" 2>/dev/null; do
    if [ -n "$NM_LLAMA_PID" ] && ! kill -0 "$NM_LLAMA_PID" 2>/dev/null; then
      nm_warn "local llama.cpp exited; stopping app"
      break
    fi
    sleep 2
  done
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  nm_main "$@"
fi
