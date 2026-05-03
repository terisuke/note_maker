from __future__ import annotations

import os
import signal
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Mapping


HOST = "127.0.0.1"


@dataclass(frozen=True)
class E2EServer:
    base_url: str
    port: int
    log_path: Path
    _process: subprocess.Popen[bytes]

    @property
    def url(self) -> str:
        return self.base_url

    def read_log(self) -> str:
        return self.log_path.read_text(encoding="utf-8", errors="replace")

    def stop(self) -> None:
        if self._process.poll() is not None:
            return
        try:
            if hasattr(os, "killpg"):
                os.killpg(self._process.pid, signal.SIGTERM)
            else:
                self._process.terminate()
            self._process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            if hasattr(os, "killpg"):
                os.killpg(self._process.pid, signal.SIGKILL)
            else:
                self._process.kill()
            self._process.wait(timeout=5)
        except ProcessLookupError:
            pass


def find_free_port(host: str = HOST) -> int:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind((host, 0))
        return int(sock.getsockname()[1])


def start_server(
    repo_root: Path,
    tmp_dir: Path,
    *,
    host: str = HOST,
    timeout_seconds: float = 30.0,
    extra_env: Mapping[str, str] | None = None,
) -> E2EServer:
    port = find_free_port(host)
    base_url = f"http://{host}:{port}"
    log_file = tempfile.NamedTemporaryFile(
        prefix="note-maker-e2e-server-",
        suffix=".log",
        delete=False,
    )
    log_path = Path(log_file.name)

    env = _server_env(port, tmp_dir)
    if extra_env:
        env.update(extra_env)

    process = subprocess.Popen(
        ["go", "run", "./cmd/server"],
        cwd=repo_root,
        env=env,
        stdout=log_file,
        stderr=subprocess.STDOUT,
        start_new_session=True,
    )
    server = E2EServer(base_url=base_url, port=port, log_path=log_path, _process=process)

    try:
        _wait_until_ready(server, timeout_seconds)
    except Exception:
        server.stop()
        raise
    finally:
        log_file.close()

    return server


def _server_env(port: int, tmp_dir: Path) -> dict[str, str]:
    env = os.environ.copy()
    env.update(
        {
            "PORT": str(port),
            "NOTE_MAKER_CONFIG_PATH": str(tmp_dir / "app_config.json"),
            "WORKFLOW_STORE_DRIVER": "json",
            "WORKFLOW_STORE_PATH": str(tmp_dir / "workflow_store.json"),
            "LLM_BASE_URL": "http://127.0.0.1:1/v1",
            "LLAMACPP_BASE_URL": "http://127.0.0.1:1/v1",
            "LLM_MODEL": "e2e-stubbed-model",
            "STYLE_LLM_MODEL": "e2e-stubbed-style",
            "BRIEF_LLM_MODEL": "e2e-stubbed-brief",
            "ARTICLE_LLM_MODEL": "e2e-stubbed-article",
            "DRAFT_LLM_MODEL": "e2e-stubbed-draft",
            "VERIFY_LLM_MODEL": "e2e-stubbed-verify",
            "LLM_TIMEOUT_SECONDS": "1",
            "LLM_STREAM_FIRST_BYTE_TIMEOUT_SECONDS": "1",
            "LLM_STREAM_IDLE_TIMEOUT_SECONDS": "1",
        }
    )
    for name in (
        "LLM_FALLBACK_BASE_URLS",
        "FALLBACK_LLM_BASE_URLS",
        "FALLBACK_LLM_BASE_URL",
        "STYLE_LLM_FALLBACK_BASE_URLS",
        "STYLE_FALLBACK_LLM_BASE_URLS",
        "BRIEF_LLM_FALLBACK_BASE_URLS",
        "BRIEF_FALLBACK_LLM_BASE_URLS",
        "ARTICLE_LLM_FALLBACK_BASE_URLS",
        "ARTICLE_FALLBACK_LLM_BASE_URLS",
        "DRAFT_LLM_FALLBACK_BASE_URLS",
        "DRAFT_FALLBACK_LLM_BASE_URLS",
        "VERIFY_LLM_FALLBACK_BASE_URLS",
        "VERIFY_FALLBACK_LLM_BASE_URLS",
    ):
        env[name] = ""
    return env


def _wait_until_ready(server: E2EServer, timeout_seconds: float) -> None:
    deadline = time.monotonic() + timeout_seconds
    last_error: BaseException | None = None

    while time.monotonic() < deadline:
        if server._process.poll() is not None:
            raise RuntimeError(
                "E2E server exited before it became ready.\n"
                f"Exit code: {server._process.returncode}\n"
                f"Log:\n{server.read_log()}"
            )
        try:
            with urllib.request.urlopen(server.base_url + "/", timeout=0.5) as response:
                if response.status < 500:
                    return
        except (urllib.error.URLError, TimeoutError, OSError) as exc:
            last_error = exc
        time.sleep(0.1)

    raise TimeoutError(
        f"E2E server did not become ready at {server.base_url} within {timeout_seconds:.1f}s.\n"
        f"Last error: {last_error}\n"
        f"Log:\n{server.read_log()}"
    )
