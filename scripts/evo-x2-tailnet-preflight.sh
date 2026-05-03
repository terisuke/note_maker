#!/usr/bin/env bash
set -euo pipefail

host="${EVO_X2_TAILNET_HOST:-evo-x2}"
timeout="${EVO_X2_PREFLIGHT_TIMEOUT_SECONDS:-5}"
base_url="${EVO_X2_LLM_BASE_URL:-http://${host}:11434/v1}"
models_url="${base_url%/}/models"

if curl -fsS --max-time "$timeout" "$models_url" >/dev/null; then
  echo "Evo X2 Tailnet LLM API is ready: ${models_url}"
  exit 0
fi

cat >&2 <<EOF
Evo X2 Tailnet LLM API is not reachable.

Expected primary endpoint:
  ${models_url}

This is the shared system path. It should work from any authorized Tailnet
device without per-device SSH forwarding.

Check on the client:
  tailscale status
  tailscale ping ${host}
  curl ${models_url}

Check on Evo X2:
  Ollama or llama.cpp must listen on the Tailnet-reachable interface.
  For Ollama, set OLLAMA_HOST to the Tailnet IP or 0.0.0.0 only if Tailscale
  ACLs/firewall rules restrict access appropriately.

The SSH tunnel path is intentionally not used by this default preflight.
Use make evo-x2-ssh-models only for a developer-specific diagnostic tunnel.
EOF

if command -v tailscale >/dev/null 2>&1; then
  echo >&2
  echo "Tailscale status excerpt:" >&2
  tailscale status 2>/dev/null | sed -n '1,12p' >&2 || true
fi

exit 1
