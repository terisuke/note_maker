#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
# shellcheck source=scripts/launcher.sh
source "${ROOT}/scripts/launcher.sh"

fail() {
  printf 'launcher_test: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  local got="$1"
  local want="$2"
  local label="$3"
  if [ "$got" != "$want" ]; then
    fail "${label}: got ${got}, want ${want}"
  fi
}

tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
  if [ -n "${busy_pid:-}" ]; then
    kill "$busy_pid" 2>/dev/null || true
    wait "$busy_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

NOTE_MAKER_DATA_DIR="${tmp_dir}/custom-data"
assert_eq "$(nm_default_data_dir)" "${tmp_dir}/custom-data" "NOTE_MAKER_DATA_DIR override"
unset NOTE_MAKER_DATA_DIR

HOME="${tmp_dir}/home"
mkdir -p "$HOME"
unset XDG_DATA_HOME
case "$(uname -s)" in
  Darwin)
    assert_eq "$(nm_default_data_dir)" "${HOME}/Library/Application Support/Note Maker" "macOS default data dir"
    ;;
  *)
    assert_eq "$(nm_default_data_dir)" "${HOME}/.local/share/note-maker" "XDG fallback data dir"
    XDG_DATA_HOME="${tmp_dir}/xdg"
    assert_eq "$(nm_default_data_dir)" "${XDG_DATA_HOME}/note-maker" "XDG_DATA_HOME data dir"
    ;;
esac

assert_eq "$(nm_models_url "http://example.test/v1/")" "http://example.test/v1/models" "models URL normalization"

if command -v python3 >/dev/null 2>&1; then
  port_file="${tmp_dir}/busy-port"
  python3 - "$port_file" <<'PY' &
import socket
import sys
import time

sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.bind(("127.0.0.1", 0))
sock.listen()
with open(sys.argv[1], "w", encoding="utf-8") as handle:
    handle.write(str(sock.getsockname()[1]))
    handle.flush()
time.sleep(30)
PY
  busy_pid="$!"
  for _ in 1 2 3 4 5; do
    [ -s "$port_file" ] && break
    sleep 1
  done
  [ -s "$port_file" ] || fail "busy port helper did not start"
  busy_port="$(cat "$port_file")"
  selected_port="$(nm_find_available_port "$busy_port" 0)"
  if [ "$selected_port" = "$busy_port" ]; then
    fail "port selection reused busy port ${busy_port}"
  fi
  if nm_find_available_port "$busy_port" 1 >/dev/null 2>&1; then
    fail "strict port mode accepted busy port ${busy_port}"
  fi
fi

printf 'launcher_test: ok\n'
