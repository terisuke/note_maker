#!/usr/bin/env bash
set -euo pipefail

ssh_host="${EVO_X2_SSH_HOST:-evo-x2}"
ssh_bin="${EVO_X2_SSH_BIN:-ssh}"
local_host="${EVO_X2_SSH_LOCAL_HOST:-127.0.0.1}"
local_port="${EVO_X2_SSH_LOCAL_PORT:-21434}"
remote_host="${EVO_X2_REMOTE_LLM_HOST:-127.0.0.1}"
remote_port="${EVO_X2_REMOTE_LLM_PORT:-11434}"
timeout="${EVO_X2_PREFLIGHT_TIMEOUT_SECONDS:-5}"

base_url="${EVO_X2_LLM_BASE_URL:-http://${local_host}:${local_port}/v1}"
models_url="${base_url%/}/models"
remote_models_url="http://${remote_host}:${remote_port}/v1/models"

check_models() {
  curl -fsS --max-time "$timeout" "$1" >/dev/null
}

if check_models "$models_url"; then
  echo "Evo X2 SSH diagnostic tunnel is ready: ${models_url}"
  exit 0
fi

if ! command -v "$ssh_bin" >/dev/null 2>&1; then
  echo "${ssh_bin} was not found. Install/configure SSH before using Evo X2 as the primary LLM." >&2
  exit 127
fi

echo "Checking Evo X2 over SSH: ${ssh_host} -> ${remote_models_url}" >&2
if "$ssh_bin" -o BatchMode=yes -o ConnectTimeout="$timeout" "$ssh_host" "curl -fsS --max-time ${timeout} '${remote_models_url}' >/dev/null"; then
  if check_models "$models_url"; then
    echo "Evo X2 SSH LLM tunnel is ready: ${models_url}"
    exit 0
  fi
else
  echo "Evo X2 SSH command failed. Verify 'ssh ${ssh_host}' works before running the app." >&2
fi

echo "Opening explicit SSH tunnel: ${local_host}:${local_port} -> ${ssh_host}:${remote_host}:${remote_port}" >&2
if "$ssh_bin" \
  -fN \
  -o BatchMode=yes \
  -o ExitOnForwardFailure=yes \
  -o ConnectTimeout="$timeout" \
  -L "${local_host}:${local_port}:${remote_host}:${remote_port}" \
  "$ssh_host"; then
  if check_models "$models_url"; then
    echo "Evo X2 SSH LLM tunnel is ready: ${models_url}"
    exit 0
  fi
fi

cat >&2 <<EOF
Evo X2 SSH diagnostic tunnel is not reachable.

Expected primary endpoint:
  ${models_url}

Expected remote endpoint over SSH:
  ${remote_models_url}

This SSH tunnel is not the default system path. The default path is the
Tailnet OpenAI-compatible API, normally http://evo-x2:11434/v1.
EOF
exit 1
