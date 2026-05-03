#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/evo-x2-llama-swap.sh [inspect|plan|start|swap]

Dry-run is the default. Mutating remote actions require:

  EVO_X2_LLAMA_CPP_APPLY=1

Restarting the shared llama.cpp service for a profile swap also requires:

  EVO_X2_LLAMA_CPP_ALLOW_RESTART=1

Environment:
  EVO_X2_TAILNET_HOST                 Tailnet host, default evo-x2.tailb30e58.ts.net
  EVO_X2_SSH_HOST                     SSH host for systemd operations, default evo-x2
  EVO_X2_OLLAMA_LLM_BASE_URL          Ollama primary OpenAI URL
  EVO_X2_LLAMA_CPP_LLM_BASE_URL       llama.cpp fallback OpenAI URL
  EVO_X2_LLAMA_CPP_SERVICE            systemd service, default note-maker-llama-cpp.service
  EVO_X2_LLAMA_CPP_PROFILE            profile name, default fallback-gemma-e2b
  EVO_X2_LLAMA_CPP_PROFILE_ENV        profile env file on Evo X2
  EVO_X2_LLAMA_CPP_ACTIVE_ENV         active env symlink on Evo X2
  EVO_X2_LLAMA_CPP_HEALTH_PATH        health path, default /health
EOF
}

action="${1:-inspect}"
case "$action" in
  inspect|plan|start|swap)
    ;;
  -h|--help|help)
    usage
    exit 0
    ;;
  *)
    echo "Unknown action: ${action}" >&2
    usage >&2
    exit 2
    ;;
esac

tailnet_host="${EVO_X2_TAILNET_HOST:-evo-x2.tailb30e58.ts.net}"
ssh_host="${EVO_X2_SSH_HOST:-evo-x2}"
ssh_bin="${EVO_X2_SSH_BIN:-ssh}"
timeout="${EVO_X2_PREFLIGHT_TIMEOUT_SECONDS:-5}"
ollama_base_url="${EVO_X2_OLLAMA_LLM_BASE_URL:-http://${tailnet_host}/v1}"
llama_base_url="${EVO_X2_LLAMA_CPP_LLM_BASE_URL:-http://${tailnet_host}/llama/v1}"
service="${EVO_X2_LLAMA_CPP_SERVICE:-note-maker-llama-cpp.service}"
profile="${EVO_X2_LLAMA_CPP_PROFILE:-fallback-gemma-e2b}"
profile_env="${EVO_X2_LLAMA_CPP_PROFILE_ENV:-/etc/note-maker/llama-cpp/${profile}.env}"
active_env="${EVO_X2_LLAMA_CPP_ACTIVE_ENV:-/etc/note-maker/llama-cpp/active.env}"
health_path="${EVO_X2_LLAMA_CPP_HEALTH_PATH:-/health}"
apply="${EVO_X2_LLAMA_CPP_APPLY:-0}"
allow_restart="${EVO_X2_LLAMA_CPP_ALLOW_RESTART:-0}"

strip_v1() {
  local value="$1"
  value="${value%/}"
  printf '%s\n' "${value%/v1}"
}

curl_json() {
  local url="$1"
  if command -v jq >/dev/null 2>&1; then
    curl -fsS --max-time "$timeout" "$url" | jq .
  else
    curl -fsS --max-time "$timeout" "$url"
    printf '\n'
  fi
}

print_models() {
  local label="$1"
  local base_url="$2"
  local models_url="${base_url%/}/models"
  echo "== ${label}: ${models_url}"
  if ! curl_json "$models_url"; then
    echo "${label} models endpoint is not reachable" >&2
    return 1
  fi
}

run_ssh() {
  if ! command -v "$ssh_bin" >/dev/null 2>&1; then
    echo "${ssh_bin} was not found; remote systemd operation cannot run" >&2
    return 127
  fi
  "$ssh_bin" -o BatchMode=yes -o ConnectTimeout="$timeout" "$ssh_host" "$@"
}

remote_quote() {
  printf "'%s'" "$(printf '%s' "$1" | sed "s/'/'\\\\''/g")"
}

remote_command() {
  local command_action="${1:-$action}"
  local quoted_profile_env quoted_active_env quoted_service
  quoted_profile_env="$(remote_quote "$profile_env")"
  quoted_active_env="$(remote_quote "$active_env")"
  quoted_service="$(remote_quote "$service")"

  case "$command_action" in
    start)
      printf 'sudo systemctl start %s\n' "$quoted_service"
      ;;
    swap)
      printf 'sudo ln -sfn %s %s && sudo systemctl restart %s\n' "$quoted_profile_env" "$quoted_active_env" "$quoted_service"
      ;;
    *)
      return 1
      ;;
  esac
}

print_strategy() {
  cat <<EOF
Recommended strategy:
  Keep Evo X2 Ollama primary at ${ollama_base_url}.
  Keep Evo X2 llama.cpp as an explicit fallback at ${llama_base_url}.
  Run exactly one shared llama.cpp fallback profile behind /llama/v1 unless live
  validation proves concurrent model services do not starve Ollama.

Selected llama.cpp service profile:
  service:     ${service}
  profile:     ${profile}
  profile env: ${profile_env}
  active env:  ${active_env}

The profile env should contain the llama-server model path/HF file, alias,
context size, host/port, and GPU/Vulkan/RADV settings. The systemd service
should read ${active_env}; swapping profiles is symlink + service restart.
EOF
}

inspect_remote_systemd() {
  echo "== Remote systemd status: ${ssh_host} ${service}"
  if ! run_ssh "systemctl is-active $(remote_quote "$service") 2>/dev/null || true; systemctl --no-pager --full status $(remote_quote "$service") 2>/dev/null | sed -n '1,18p' || true"; then
    echo "Remote systemd status unavailable; API inspection above is still authoritative for clients." >&2
  fi
}

if [ "$ollama_base_url" = "$llama_base_url" ]; then
  echo "Refusing to operate: Ollama and llama.cpp URLs are identical (${ollama_base_url})." >&2
  exit 2
fi

if [ "$action" = "inspect" ]; then
  print_models "Ollama primary" "$ollama_base_url" || true
  echo
  print_models "llama.cpp fallback" "$llama_base_url" || true
  echo
  inspect_remote_systemd
  exit 0
fi

print_strategy

if [ "$action" = "plan" ]; then
  echo
  echo "Dry-run remote commands:"
  echo "  start: $(remote_command start)"
  echo "  swap:  $(remote_command swap)"
  echo
  echo "Post-checks:"
  echo "  curl -fsS $(strip_v1 "$llama_base_url")${health_path}"
  echo "  curl -fsS ${llama_base_url%/}/models"
  exit 0
fi

command_text="$(remote_command)"
echo
echo "Remote command:"
echo "  ${command_text}"

if [ "$apply" != "1" ]; then
  echo
  echo "Dry-run only. Set EVO_X2_LLAMA_CPP_APPLY=1 to run this on ${ssh_host}."
  exit 0
fi

if [ "$action" = "swap" ] && [ "$allow_restart" != "1" ]; then
  echo "Refusing swap without EVO_X2_LLAMA_CPP_ALLOW_RESTART=1." >&2
  exit 2
fi

run_ssh "$command_text"

echo
echo "Waiting for llama.cpp health..."
sleep 2
curl_json "$(strip_v1 "$llama_base_url")${health_path}" || true
echo
print_models "llama.cpp fallback" "$llama_base_url"
