#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/evo-x2-llama-swap.sh [inspect|plan|validate|start|swap]

Primary issue #90 path:

  validate    Network-free llama-swap adoption/template validation.
  inspect     Read-only API and legacy service diagnostic.
  plan        Print the legacy service diagnostic command plan.

Manual diagnostic only:

  start       Legacy systemd fallback service start.
  swap        Legacy systemd profile symlink swap + service restart.

The legacy start/swap path is not the llama-swap adoption path. Use it only
during an explicitly scheduled maintenance diagnostic.

Dry-run is the default. Mutating remote actions require:

  APPLY=1

or:

  EVO_X2_LLAMA_CPP_APPLY=1

Restarting the shared llama.cpp service for a profile swap also requires:

  ALLOW_RESTART=1

or:

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
  LLAMA_SWAP_CONFIG                   local llama-swap YAML template to validate
EOF
}

action="${1:-inspect}"
case "$action" in
  inspect|plan|validate|start|swap)
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
apply="${EVO_X2_LLAMA_CPP_APPLY:-${APPLY:-0}}"
allow_restart="${EVO_X2_LLAMA_CPP_ALLOW_RESTART:-${ALLOW_RESTART:-0}}"
llama_swap_config="${LLAMA_SWAP_CONFIG:-deploy/llama-swap/evo-x2.example.yaml}"
llama_swap_models=("gemma4:e2b" "qwen3.6:27b" "gemma4:31b")
llama_swap_optional_aliases=("gemma4:latest")

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
Recommended issue #90 strategy:
  Keep Evo X2 Ollama primary at ${ollama_base_url}.
  Put llama-swap in front of llama-server backends at ${llama_base_url}.
  Use llama-swap model aliases for the explicit fallback path and keep the app
  pointed directly at /llama/v1 only inside live-gated validation targets.

Legacy manual diagnostic:
  The old systemd profile path below is retained only for diagnosing the
  previous one-active-profile fallback service. It is not the adoption path for
  llama-swap and should not be used for routine model routing.

Selected legacy llama.cpp service profile:
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

validate_equal() {
  local label="$1"
  local got="$2"
  local want="$3"

  if [ "$got" = "$want" ]; then
    echo "ok - ${label}"
    return 0
  fi

  echo "not ok - ${label}" >&2
  echo "  got:  ${got}" >&2
  echo "  want: ${want}" >&2
  return 1
}

validate_contains() {
  local label="$1"
  local got="$2"
  local want="$3"

  case "$got" in
    *"$want"*)
      echo "ok - ${label}"
      return 0
      ;;
    *)
      echo "not ok - ${label}" >&2
      echo "  missing: ${want}" >&2
      echo "  in:      ${got}" >&2
      return 1
      ;;
  esac
}

validate_local() {
  local failures=0
  local start_command swap_command
  local expected_start expected_swap

  echo "== Network-free llama-swap validation"
  echo "This validation does not call curl, ssh, or systemctl."
  echo

  if [ "$ollama_base_url" != "$llama_base_url" ]; then
    echo "ok - primary and fallback URLs are distinct"
  else
    echo "not ok - primary and fallback URLs must be distinct (${ollama_base_url})" >&2
    failures=$((failures + 1))
  fi

  if [ "$apply" != "1" ]; then
    echo "ok - mutating actions are dry-run by default (APPLY=${apply})"
  else
    echo "not ok - validation must run with APPLY unset or 0" >&2
    failures=$((failures + 1))
  fi

  if [ "$allow_restart" != "1" ]; then
    echo "ok - profile swaps require an explicit restart gate (ALLOW_RESTART=${allow_restart})"
  else
    echo "not ok - validation must run with ALLOW_RESTART unset or 0" >&2
    failures=$((failures + 1))
  fi

  start_command="$(remote_command start)"
  swap_command="$(remote_command swap)"
  expected_start="sudo systemctl start $(remote_quote "$service")"
  expected_swap="sudo ln -sfn $(remote_quote "$profile_env") $(remote_quote "$active_env") && sudo systemctl restart $(remote_quote "$service")"

  validate_equal "start command is deterministic" "$start_command" "$expected_start" || failures=$((failures + 1))
  validate_equal "swap command is deterministic" "$swap_command" "$expected_swap" || failures=$((failures + 1))
  validate_contains "swap command updates active profile symlink" "$swap_command" "ln -sfn" || failures=$((failures + 1))
  validate_contains "swap command restarts only the llama.cpp service" "$swap_command" "systemctl restart $(remote_quote "$service")" || failures=$((failures + 1))
  validate_llama_swap_config || failures=$((failures + 1))

  echo
  echo "Validated command plan:"
  echo "  start: ${start_command}"
  echo "  swap:  ${swap_command}"

  if [ "$failures" -ne 0 ]; then
    echo
    echo "Network-free validation failed with ${failures} failure(s)." >&2
    return 1
  fi

  echo
  echo "Network-free validation passed."
}

validate_llama_swap_config() {
  local failures=0

  echo
  echo "== llama-swap config template validation"
  if [ ! -f "$llama_swap_config" ]; then
    echo "not ok - llama-swap config template exists (${llama_swap_config})" >&2
    return 1
  fi
  echo "ok - llama-swap config template exists (${llama_swap_config})"

  for required in "healthCheckTimeout:" "models:" "cmd:"; do
    if grep -Fq -- "$required" "$llama_swap_config"; then
      echo "ok - config contains ${required}"
    else
      echo "not ok - config must contain ${required}" >&2
      failures=$((failures + 1))
    fi
  done

  for model in "${llama_swap_models[@]}"; do
    if grep -Fq -- "\"${model}\":" "$llama_swap_config" && grep -Fq -- "--alias ${model}" "$llama_swap_config"; then
      echo "ok - config maps ${model} with matching llama-server alias"
    else
      echo "not ok - config must map ${model} and use --alias ${model}" >&2
      failures=$((failures + 1))
    fi
  done

  for model in "${llama_swap_optional_aliases[@]}"; do
    if grep -Fq -- "\"${model}\":" "$llama_swap_config" || grep -Fq -- "--alias ${model}" "$llama_swap_config"; then
      if grep -Fq -- "\"${model}\":" "$llama_swap_config" && grep -Fq -- "--alias ${model}" "$llama_swap_config"; then
        echo "ok - optional alias ${model} is internally consistent"
      else
        echo "not ok - optional alias ${model} must include both model key and --alias" >&2
        failures=$((failures + 1))
      fi
    fi
  done

  if grep -Fq -- "llama-server" "$llama_swap_config"; then
    echo "ok - config launches llama-server backends"
  else
    echo "not ok - config must launch llama-server backends" >&2
    failures=$((failures + 1))
  fi

  if grep -Ev '^[[:space:]]*#' "$llama_swap_config" | grep -Eiq "ollama|systemctl|sudo|ssh "; then
    echo "not ok - llama-swap config must not call Ollama, systemctl, sudo, or ssh" >&2
    failures=$((failures + 1))
  else
    echo "ok - config does not call Ollama, systemctl, sudo, or ssh"
  fi

  local ports unique_ports host_count
  ports="$(grep -E -- "--port[[:space:]]+[0-9]+" "$llama_swap_config" | awk '{print $2}' | sort)"
  unique_ports="$(printf '%s\n' "$ports" | sed '/^$/d' | uniq)"
  if [ "$(printf '%s\n' "$ports" | sed '/^$/d' | wc -l | tr -d ' ')" -ge "${#llama_swap_models[@]}" ] && [ "$ports" = "$unique_ports" ]; then
    echo "ok - backend ports are present and unique"
  else
    echo "not ok - backend ports must be present and unique" >&2
    failures=$((failures + 1))
  fi

  host_count="$(grep -Fc -- "--host 127.0.0.1" "$llama_swap_config" || true)"
  if [ "$host_count" -ge "${#llama_swap_models[@]}" ]; then
    echo "ok - llama-server backends bind to localhost"
  else
    echo "not ok - llama-server backends should bind to 127.0.0.1" >&2
    failures=$((failures + 1))
  fi

  return "$failures"
}

if [ "$action" = "validate" ]; then
  validate_local
  exit 0
fi

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
  echo "Legacy manual diagnostic commands (not the llama-swap adoption path):"
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
echo
echo "Manual diagnostic only: this legacy systemd profile path is not the llama-swap adoption path."

if [ "$apply" != "1" ]; then
  echo
  echo "Dry-run only. Set APPLY=1 or EVO_X2_LLAMA_CPP_APPLY=1 to run this on ${ssh_host}."
  exit 0
fi

if [ "$action" = "swap" ] && [ "$allow_restart" != "1" ]; then
  echo "Refusing swap without ALLOW_RESTART=1 or EVO_X2_LLAMA_CPP_ALLOW_RESTART=1." >&2
  exit 2
fi

run_ssh "$command_text"

echo
echo "Waiting for llama.cpp health..."
sleep 2
curl_json "$(strip_v1 "$llama_base_url")${health_path}" || true
echo
print_models "llama.cpp fallback" "$llama_base_url"
