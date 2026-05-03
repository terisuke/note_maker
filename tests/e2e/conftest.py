from __future__ import annotations

import json
import os
from fnmatch import fnmatch
from pathlib import Path
from typing import Any
from urllib.parse import urlparse

import pytest

from e2e_server import E2EServer, start_server


MARKED_CDN_PATTERN = "**/cdnjs.cloudflare.com/ajax/libs/marked/4.3.0/marked.min.js"
MARKED_STUB = r"""
(() => {
  const escapeHTML = (value) => String(value ?? '')
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;');
  const inline = (value) => escapeHTML(value)
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/`([^`]+)`/g, '<code>$1</code>');
  window.marked = {
    parse(markdown) {
      return String(markdown ?? '')
        .split(/\n{2,}/)
        .filter((block) => block.trim())
        .map((block) => {
          const text = block.trim();
          const heading = /^(#{1,6})\s+(.+)$/.exec(text);
          if (heading) {
            const level = heading[1].length;
            return `<h${level}>${inline(heading[2])}</h${level}>`;
          }
          return `<p>${inline(text).replace(/\n/g, '<br>')}</p>`;
        })
        .join('\n');
    },
  };
})();
""".strip()


class ApiStubber:
    def __init__(self, context: Any) -> None:
        self._context = context
        self._stubs: list[dict[str, Any]] = []
        self.requests: list[dict[str, Any]] = []
        self.install_default_stubs()
        context.route("**/api/**", self._handle)

    def install_default_stubs(self) -> None:
        self.stub_json("GET", "/api/models", ["gemma4:31b", "gemma4:e2b", "gemma4:latest"])
        self.stub_json("GET", "/api/personas", [default_persona()])
        self.stub_json("GET", "/api/formats", [default_format()])
        self.stub_json("GET", "/api/config/storage", default_storage_config())
        self.stub_json("PATCH", "/api/config/storage", default_storage_config())
        self.stub_json("GET", "/api/brief-sessions/templates", default_question_template())
        self.stub_json("GET", "/api/workflow/artifacts", default_workflow_artifacts())
        self.stub_json("GET", "/api/history", default_workflow_artifacts())
        self.stub_json("GET", "/api/author-style", {"style_guides": []})
        self.stub_json("GET", "/api/brief-sessions", {"sessions": []})
        self.stub_json("GET", "/api/briefs", {"briefs": []})
        self.stub_json("GET", "/api/projects", {"projects": []})

    def stub_json(
        self,
        method: str,
        path: str,
        payload: Any,
        *,
        status: int = 200,
        headers: dict[str, str] | None = None,
    ) -> None:
        self._add_stub(
            method,
            path,
            status=status,
            headers={"Content-Type": "application/json", **(headers or {})},
            body=json.dumps(payload),
        )

    def stub_error(
        self,
        method: str,
        path: str,
        *,
        status: int = 500,
        message: str = "stubbed API error",
        code: str = "E2E_STUB_ERROR",
    ) -> None:
        self.stub_json(method, path, {"error": {"code": code, "message": message}}, status=status)

    def stub_sse(
        self,
        method: str,
        path: str,
        events: list[dict[str, Any]],
        *,
        status: int = 200,
    ) -> None:
        body = "".join(_format_sse_event(event) for event in events)
        self._add_stub(
            method,
            path,
            status=status,
            headers={"Content-Type": "text/event-stream"},
            body=body,
        )

    def clear(self) -> None:
        self._stubs.clear()

    def reset(self) -> None:
        self.clear()
        self.install_default_stubs()

    def _add_stub(
        self,
        method: str,
        path: str,
        *,
        status: int,
        headers: dict[str, str],
        body: str,
    ) -> None:
        self._stubs.insert(
            0,
            {
                "method": method.upper(),
                "path": path,
                "status": status,
                "headers": headers,
                "body": body,
            },
        )

    def _handle(self, route: Any) -> None:
        request = route.request
        parsed = urlparse(request.url)
        path_with_query = parsed.path + (f"?{parsed.query}" if parsed.query else "")
        record = {
            "method": request.method,
            "path": parsed.path,
            "query": parsed.query,
            "url": request.url,
            "post_data": request.post_data,
        }
        self.requests.append(record)

        for stub in self._stubs:
            if request.method.upper() != stub["method"]:
                continue
            if _path_matches(stub["path"], path_with_query, parsed.path):
                route.fulfill(status=stub["status"], headers=stub["headers"], body=stub["body"])
                return

        route.fulfill(
            status=501,
            headers={"Content-Type": "application/json"},
            body=json.dumps(
                {
                    "error": {
                        "code": "E2E_API_STUB_MISSING",
                        "message": f"No E2E stub registered for {request.method} {path_with_query}",
                    }
                }
            ),
        )


def pytest_collection_modifyitems(items: list[pytest.Item]) -> None:
    for item in items:
        if f"{os.sep}tests{os.sep}e2e{os.sep}" in str(item.path):
            item.add_marker(pytest.mark.e2e)


@pytest.fixture(scope="session")
def repo_root() -> Path:
    return Path(__file__).resolve().parents[2]


@pytest.fixture(scope="session")
def e2e_server(repo_root: Path, tmp_path_factory: pytest.TempPathFactory) -> E2EServer:
    server = start_server(repo_root, tmp_path_factory.mktemp("note-maker-e2e-server"))
    try:
        yield server
    finally:
        server.stop()


@pytest.fixture(scope="session")
def base_url(e2e_server: E2EServer) -> str:
    return e2e_server.base_url


@pytest.fixture(scope="session")
def playwright_instance() -> Any:
    try:
        from playwright.sync_api import sync_playwright
    except ImportError as exc:
        pytest.fail(
            f"Playwright is required for browser E2E tests but is not installed: {exc}",
            pytrace=False,
        )

    with sync_playwright() as playwright:
        yield playwright


@pytest.fixture(scope="session")
def browser(playwright_instance: Any) -> Any:
    from playwright.sync_api import Error as PlaywrightError

    browser_name = os.environ.get("E2E_BROWSER", "chromium")
    launcher = getattr(playwright_instance, browser_name, None)
    if launcher is None:
        pytest.fail(f"Unsupported Playwright browser: {browser_name}", pytrace=False)

    headless = os.environ.get("E2E_HEADLESS", "1").lower() not in {"0", "false", "no"}
    try:
        browser = launcher.launch(headless=headless)
    except PlaywrightError as exc:
        pytest.fail(
            f"Playwright {browser_name} is required for browser E2E tests but cannot start: {exc}",
            pytrace=False,
        )

    try:
        yield browser
    finally:
        browser.close()


@pytest.fixture()
def context(browser: Any, base_url: str) -> Any:
    context = browser.new_context(base_url=base_url)
    context.route(MARKED_CDN_PATTERN, _fulfill_marked_stub)
    try:
        yield context
    finally:
        context.close()


@pytest.fixture()
def api_stub(context: Any) -> ApiStubber:
    return ApiStubber(context)


@pytest.fixture()
def api_stubs(api_stub: ApiStubber) -> ApiStubber:
    return api_stub


@pytest.fixture()
def page(context: Any, api_stub: ApiStubber) -> Any:
    return context.new_page()


def default_persona() -> dict[str, Any]:
    return {
        "id": "terisuke",
        "display_name": "E2E Terisuke",
        "description": "Browser E2E persona",
        "default_format": "note_article",
        "sources": [{"kind": "note", "ref": "e2e_writer", "url": "https://example.test/rss"}],
        "voice_notes": {"first_person": ["私"], "tone": "calm", "title_patterns": []},
    }


def default_format() -> dict[str, Any]:
    return {
        "id": "note_article",
        "display_name": "note",
        "description": "E2E note format",
        "extension": ".md",
    }


def default_storage_config() -> dict[str, Any]:
    return {
        "active_driver": "json",
        "active_path": "tmp/e2e-workflow-store.json",
        "configured_driver": "json",
        "configured_path": "tmp/e2e-workflow-store.json",
        "config_path": "tmp/e2e-app-config.json",
        "env_locked": False,
        "restart_required": False,
        "restart_message": "現在の保存方式で動作中です。",
        "effective_next_boot": True,
    }


def default_question_template() -> dict[str, Any]:
    return {
        "persona_id": "terisuke",
        "output_format_id": "note_article",
        "questions": [
            {
                "id": "theme",
                "text": "この記事で伝えたいことは何ですか？",
                "flow_type": "main",
                "target_field": "theme",
                "required": True,
            }
        ],
    }


def default_workflow_artifacts() -> dict[str, Any]:
    return {
        "style_guides": [],
        "sessions": [],
        "briefs": [],
        "projects": [],
        "articles": [],
        "drafts": [],
    }


def _path_matches(pattern: str, path_with_query: str, path_only: str) -> bool:
    return (
        pattern == path_with_query
        or pattern == path_only
        or fnmatch(path_with_query, pattern)
        or fnmatch(path_only, pattern)
    )


def _format_sse_event(event: dict[str, Any]) -> str:
    event_name = event.get("event", "message")
    data = event.get("data", {})
    encoded = data if isinstance(data, str) else json.dumps(data)
    return f"event: {event_name}\ndata: {encoded}\n\n"


def _fulfill_marked_stub(route: Any) -> None:
    route.fulfill(
        status=200,
        headers={"Content-Type": "application/javascript; charset=utf-8"},
        body=MARKED_STUB,
    )
