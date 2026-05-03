#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
scripts=(
  "scripts/dev.sh"
  "scripts/evo-x2-tailnet-preflight.sh"
  "scripts/evo-x2-ssh-preflight.sh"
  "scripts/launcher.sh"
  "scripts/launcher_test.sh"
  "scripts/check-launcher.sh"
)

for script in "${scripts[@]}"; do
  bash -n "${ROOT}/${script}"
done

if command -v shellcheck >/dev/null 2>&1; then
  shellcheck "${scripts[@]/#/${ROOT}/}"
else
  printf 'shellcheck not found; skipped\n'
fi

"${ROOT}/scripts/launcher_test.sh"
make -C "$ROOT" -n launcher >/dev/null
make -C "$ROOT" -n launcher-status >/dev/null

printf 'launcher checks passed\n'
