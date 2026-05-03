import json
import re
from dataclasses import dataclass, field
from typing import Any
from urllib.parse import parse_qs, urlparse

import pytest
from playwright.sync_api import Page, Route, Request, expect


MARKED_STUB = """
window.marked = {
  parse(markdown) {
    return String(markdown || '')
      .split(/\\n{2,}/)
      .map((block) => {
        const escaped = block
          .replaceAll('&', '&amp;')
          .replaceAll('<', '&lt;')
          .replaceAll('>', '&gt;');
        if (escaped.startsWith('# ')) {
          return `<h1>${escaped.slice(2)}</h1>`;
        }
        if (escaped.startsWith('## ')) {
          return `<h2>${escaped.slice(3)}</h2>`;
        }
        return `<p>${escaped.replaceAll('\\n', '<br>')}</p>`;
      })
      .join('');
  },
};
"""


PERSONAS = [
    {
        "id": "terisuke",
        "display_name": "Terisuke",
        "description": "Local writing persona",
        "default_format": "note_article",
        "voice_notes": {"first_person": ["私"]},
        "sources": [{"kind": "note", "ref": "terisuke"}],
    }
]

FORMATS = [{"id": "note_article", "display_name": "note"}]
MODELS = ["style-e2e", "brief-e2e", "draft-e2e", "verify-e2e"]
QUESTIONS = [
    {
        "id": "theme",
        "text": "この記事で一番伝えたいことは？",
        "flow_type": "main",
        "target_field": "theme",
        "required": True,
    },
    {
        "id": "reader",
        "text": "想定読者は誰ですか？",
        "flow_type": "main",
        "target_field": "reader",
        "required": True,
    },
]

STYLE_DETAIL = {
    "profile_id": "style-history",
    "guide_id": "guide-history",
    "title": "履歴文体ガイド",
    "persona_id": "terisuke",
    "output_format_id": "note_article",
    "article_count": 3,
    "guide_markdown": "# 履歴文体ガイド\n\n- 具体例から始める\n- 読者の不安を短く受け止める",
}

BRIEF = {
    "theme": "履歴から再開する記事",
    "reader": "保存済みプロジェクトを確認する編集者",
    "opening_episode": "朝のレビューで履歴を開く",
    "expected_reader_action": "途中から安全に再開する",
    "must_include": "文体、取材、下書きの状態",
    "persona_id": "terisuke",
    "output_format_id": "note_article",
    "style_profile_id": "style-history",
}

SESSION_DETAIL = {
    "session_id": "session-history",
    "title": "履歴取材セッション",
    "style_profile_id": "style-history",
    "persona_id": "terisuke",
    "output_format_id": "note_article",
    "completed": True,
    "brief": BRIEF,
    "questions": QUESTIONS,
    "answers": [
        {
            "question_id": "theme",
            "content": "Original history answer",
            "flow_type": "main",
        }
    ],
}

DRAFT_MARKDOWN = """# 履歴下書き

## 背景

古い背景です。

## Target Section

古い対象セクションです。

## まとめ

古いまとめです。
"""

DRAFT_DETAIL = {
    "draft_id": "draft-history",
    "article_id": "article-history",
    "session_id": "session-history",
    "style_profile_id": "style-history",
    "persona_id": "terisuke",
    "output_format_id": "note_article",
    "version": 2,
    "score": 4.2,
    "status": "passed",
    "markdown": DRAFT_MARKDOWN,
}

ARTICLE_DETAIL = {
    "article_id": "article-history",
    "project_id": "project-history",
    "title": "履歴記事",
    "session_id": "session-history",
    "style_profile_id": "style-history",
    "persona_id": "terisuke",
    "output_format_id": "note_article",
    "status": "drafted",
    "brief": BRIEF,
    "current_draft": DRAFT_DETAIL,
    "draft_versions": [DRAFT_DETAIL],
    "source_snapshot": {
        "title": "参照ソース",
        "fetched_at": "2026-05-01T08:00:00Z",
        "articles": [
            {
                "title": "参考記事A",
                "url": "https://example.test/reference-a",
                "fetched_at": "2026-05-01T08:00:00Z",
            }
        ],
    },
}

PROJECT_DETAIL = {
    "project_id": "project-history",
    "title": "履歴プロジェクト",
    "persona_id": "terisuke",
    "output_format_id": "note_article",
    "status": "active",
    "article_count": 1,
    "articles": [ARTICLE_DETAIL],
}

HISTORY_INDEX = {
    "style_guides": [
        {
            "profile_id": "style-history",
            "title": "履歴文体ガイド",
            "persona_id": "terisuke",
            "output_format_id": "note_article",
            "updated_at": "2026-05-01T08:00:00Z",
        }
    ],
    "sessions": [
        {
            "session_id": "session-history",
            "title": "履歴取材セッション",
            "style_profile_id": "style-history",
            "persona_id": "terisuke",
            "output_format_id": "note_article",
            "completed": True,
            "updated_at": "2026-05-01T08:00:00Z",
        }
    ],
    "projects": [
        {
            "project_id": "project-history",
            "title": "履歴プロジェクト",
            "persona_id": "terisuke",
            "output_format_id": "note_article",
            "article_count": 1,
            "updated_at": "2026-05-01T08:00:00Z",
        }
    ],
    "articles": [
        {
            "article_id": "article-history",
            "project_id": "project-history",
            "title": "履歴記事",
            "persona_id": "terisuke",
            "output_format_id": "note_article",
            "updated_at": "2026-05-01T08:00:00Z",
        }
    ],
    "drafts": [
        {
            "draft_id": "draft-history",
            "article_id": "article-history",
            "session_id": "session-history",
            "style_profile_id": "style-history",
            "title": "履歴下書き",
            "persona_id": "terisuke",
            "output_format_id": "note_article",
            "version": 2,
            "updated_at": "2026-05-01T08:00:00Z",
        }
    ],
}


@dataclass
class StubState:
    calls: list[dict[str, Any]] = field(default_factory=list)
    pending_routes: dict[str, list[Route]] = field(default_factory=dict)

    def record(self, request: Request, path: str) -> dict[str, Any]:
        payload = request.post_data_json if request.post_data else None
        call = {
            "method": request.method,
            "path": path,
            "query": parse_qs(urlparse(request.url).query),
            "headers": {key.lower(): value for key, value in request.headers.items()},
            "payload": payload,
        }
        self.calls.append(call)
        return call

    def calls_to(self, path: str) -> list[dict[str, Any]]:
        return [call for call in self.calls if call["path"] == path]


def install_routes(page: Page, handlers: dict[str, Any] | None = None) -> StubState:
    state = StubState()
    handlers = handlers or {}

    page.add_init_script("localStorage.clear();")
    page.route(
        "https://cdnjs.cloudflare.com/ajax/libs/marked/4.3.0/marked.min.js",
        lambda route: route.fulfill(
            status=200,
            content_type="application/javascript",
            body=MARKED_STUB,
        ),
    )

    def api_handler(route: Route, request: Request) -> None:
        parsed = urlparse(request.url)
        path = parsed.path
        call = state.record(request, path)

        if path in handlers:
            response = handlers[path](route, request, call, state)
            if response is None:
                return
            fulfill_json(route, response)
            return

        if path == "/api/personas":
            fulfill_json(route, PERSONAS)
        elif path == "/api/formats":
            fulfill_json(route, FORMATS)
        elif path == "/api/models":
            fulfill_json(route, MODELS)
        elif path == "/api/config/storage":
            fulfill_json(
                route,
                {
                    "active_driver": "json",
                    "active_path": "data/workflow_store.json",
                    "configured_driver": "json",
                    "configured_path": "data/workflow_store.json",
                    "env_locked": False,
                    "restart_required": False,
                },
            )
        elif path == "/api/brief-sessions/templates":
            fulfill_json(route, {"questions": QUESTIONS})
        elif path == "/api/workflow/artifacts":
            fulfill_json(route, HISTORY_INDEX)
        elif path == "/api/author-style/style-history":
            fulfill_json(route, STYLE_DETAIL)
        elif path == "/api/brief-sessions/session-history":
            fulfill_json(route, SESSION_DETAIL)
        elif path == "/api/projects/project-history":
            fulfill_json(route, PROJECT_DETAIL)
        elif path == "/api/articles/article-history":
            fulfill_json(route, ARTICLE_DETAIL)
        elif path == "/api/drafts/draft-history":
            fulfill_json(route, DRAFT_DETAIL)
        else:
            pytest.fail(f"Unexpected unstubbed API request: {request.method} {path}")

    page.route("**/api/**", api_handler)
    return state


def fulfill_json(route: Route, data: Any, status: int = 200) -> None:
    route.fulfill(
        status=status,
        content_type="application/json",
        body=json.dumps(data, ensure_ascii=False),
    )


def fulfill_sse(route: Route, events: list[tuple[str, dict[str, Any]]]) -> None:
    body = "".join(
        f"event: {event}\ndata: {json.dumps(data, ensure_ascii=False)}\n\n"
        for event, data in events
    )
    route.fulfill(status=200, content_type="text/event-stream", body=body)


def hold_request(key: str):
    def handler(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        state.pending_routes.setdefault(key, []).append(route)
        return None

    return handler


def abort_pending(state: StubState, key: str) -> None:
    for route in state.pending_routes.pop(key, []):
        try:
            route.abort("aborted")
        except Exception:
            pass


def open_app(page: Page, base_url: str) -> None:
    page.goto(base_url.rstrip("/") + "/")
    expect(page.locator("#persona-select")).to_have_value("terisuke")
    expect(page.locator("#history-status")).to_contain_text("1件のプロジェクト")


def select_when_enabled(page: Page, selector: str, value: str) -> None:
    locator = page.locator(selector)
    expect(locator).to_be_enabled()
    locator.select_option(value)


def open_history_session(page: Page) -> None:
    select_when_enabled(page, "#history-style-select", "style-history")
    expect(page.locator("#style-guide-card")).to_contain_text("具体例から始める")
    select_when_enabled(page, "#history-session-select", "session-history")
    expect(page.locator("#brief-card")).to_contain_text("履歴から再開する記事")
    page.locator("#open-history-btn").click()
    expect(page.locator("#history-status")).to_contain_text("選択した履歴を現在の作業状態に反映しました")


def open_history_draft(page: Page) -> None:
    open_history_session(page)
    select_when_enabled(page, "#history-draft-select", "draft-history")
    expect(page.locator("#history-current-draft-card")).to_contain_text("履歴下書き")
    page.locator("#open-history-btn").click()
    expect(page.locator("#markdown-output")).to_have_value(re.compile("Target Section"))


def test_history_opening_populates_all_selects_and_readable_cards(page: Page, base_url: str) -> None:
    install_routes(page)
    open_app(page, base_url)

    select_when_enabled(page, "#history-style-select", "style-history")
    expect(page.locator("#style-guide-card")).to_contain_text("履歴文体ガイド")
    expect(page.locator("#style-guide-card")).to_contain_text("具体例から始める")

    select_when_enabled(page, "#history-session-select", "session-history")
    expect(page.locator("#brief-card")).to_contain_text("履歴から再開する記事")
    expect(page.locator("#brief-card")).to_contain_text("保存済みプロジェクトを確認する編集者")

    select_when_enabled(page, "#history-project-select", "project-history")
    expect(page.locator("#history-project-card")).to_contain_text("履歴プロジェクト")
    expect(page.locator("#history-project-card")).to_contain_text("1件")

    select_when_enabled(page, "#history-article-select", "article-history")
    expect(page.locator("#history-article-card")).to_contain_text("履歴記事")
    expect(page.locator("#history-article-brief-card")).to_contain_text("朝のレビューで履歴を開く")
    expect(page.locator("#history-source-snapshot-card")).to_contain_text("参考記事A")

    select_when_enabled(page, "#history-draft-select", "draft-history")
    expect(page.locator("#history-current-draft-card")).to_contain_text("Target Section")
    expect(page.locator("#history-draft-versions-card")).to_contain_text("score 4.2")

    page.locator("#open-history-btn").click()

    expect(page.locator("#history-status")).to_contain_text("選択した履歴を現在の作業状態に反映しました")
    expect(page.locator("#profile-id")).to_have_text("style-history")
    expect(page.locator("#question-log")).to_contain_text("Original history answer")
    expect(page.locator("#brief-card")).to_contain_text("途中から安全に再開する")
    expect(page.locator("#markdown-output")).to_have_value(re.compile("古い対象セクションです。"))
    expect(page.locator("#preview-content")).to_contain_text("履歴下書き")


def test_answer_streaming_follow_up_and_cancel_recovery(page: Page, base_url: str) -> None:
    def seed_style(route: Route, request: Request, call: dict[str, Any], state: StubState) -> dict[str, Any]:
        return STYLE_DETAIL

    def create_session(route: Route, request: Request, call: dict[str, Any], state: StubState) -> dict[str, Any]:
        return {
            "session_id": "session-stream",
            "style_profile_id": "style-history",
            "completed": False,
            "questions": QUESTIONS,
            "answers": [],
            "next_question": QUESTIONS[0],
        }

    def answer_stream(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        assert call["headers"]["accept"] == "text/event-stream"
        assert call["payload"]["content"] == "最初の回答です"
        fulfill_sse(
            route,
            [
                ("status", {"status": "stream_opened", "elapsed_ms": 0}),
                ("chunk", {"text": "どの判断を", "elapsed_ms": 10}),
                ("chunk", {"text": "最初に説明しますか？", "elapsed_ms": 20}),
                (
                    "result",
                    {
                        "session_id": "session-stream",
                        "completed": False,
                        "answers": [
                            {
                                "question_id": "theme",
                                "content": "最初の回答です",
                                "flow_type": "main",
                            }
                        ],
                        "next_question": {
                            "id": "theme_follow_1",
                            "text": "どの判断を最初に説明しますか？",
                            "flow_type": "deep_dive_follow_up",
                            "target_question_id": "theme",
                            "follow_up_index": 1,
                        },
                    },
                ),
                ("done", {"status": "completed", "elapsed_ms": 30}),
            ],
        )

    state = install_routes(
        page,
        {
            "/api/author-style/seed": seed_style,
            "/api/brief-sessions": create_session,
            "/api/brief-sessions/session-stream/answers": answer_stream,
        },
    )
    open_app(page, base_url)

    page.locator("#use-preset-style-btn").click()
    expect(page.locator("#profile-id")).to_have_text("style-history")
    page.locator("#start-interview-btn").click()
    expect(page.locator("#question-log")).to_contain_text("この記事で一番伝えたいことは？")

    page.locator("#answer-input").fill("最初の回答です")
    page.locator("#submit-answer-btn").click()

    expect(page.locator("#question-log")).to_contain_text("どの判断を最初に説明しますか？")
    expect(page.locator("#question-log")).to_contain_text("最初の回答です")
    expect(page.locator("#cancel-answer-btn")).to_be_hidden()
    expect(page.locator("#submit-answer-btn")).to_be_enabled()

    page.unroute("**/api/**")
    state = install_routes(
        page,
        {
            "/api/author-style/seed": seed_style,
            "/api/brief-sessions": create_session,
            "/api/brief-sessions/session-stream/answers": hold_request("answer"),
        },
    )
    page.reload()
    expect(page.locator("#history-status")).to_contain_text("1件のプロジェクト")
    page.locator("#use-preset-style-btn").click()
    page.locator("#start-interview-btn").click()
    expect(page.locator("#question-log")).to_contain_text("この記事で一番伝えたいことは？")

    page.locator("#answer-input").fill("キャンセルする回答")
    page.locator("#submit-answer-btn").evaluate("button => button.click()")
    expect(page.locator("#cancel-answer-btn")).to_be_visible()
    page.locator("#cancel-answer-btn").click()
    abort_pending(state, "answer")

    expect(page.locator("#cancel-answer-btn")).to_be_hidden()
    expect(page.locator("#submit-answer-btn")).to_be_enabled()
    expect(page.locator("#question-log")).to_contain_text("処理を停止しました")


def test_draft_streaming_and_cancel_recovery(page: Page, base_url: str) -> None:
    stream_markdown = "# Streaming Draft\n\n## Body\n\n途中まで生成してから完成します。\n"

    def draft_stream(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        assert call["headers"]["accept"] == "text/event-stream"
        assert call["payload"]["style_profile_id"] == "style-history"
        assert call["payload"]["session_id"] == "session-history"
        fulfill_sse(
            route,
            [
                ("status", {"status": "runtime_connected", "endpoint": "stubbed", "model": "draft-e2e", "elapsed_ms": 0}),
                ("chunk", {"text": "# Streaming Draft\n\n", "elapsed_ms": 5}),
                ("chunk", {"text": "## Body\n\n途中まで生成してから完成します。\n", "elapsed_ms": 15}),
                (
                    "result",
                    {
                        "draft": stream_markdown,
                        "evaluation": {"passed": True, "comparison": {"score": 4.6}, "failures": []},
                        "verification": {"performed": True, "passed": True, "summary": "stub verified", "failures": []},
                        "quality_gate": {"score": 4.6, "runes": 28, "failed_metrics": []},
                    },
                ),
                ("done", {"status": "completed", "elapsed_ms": 40, "runes": 28, "score": 4.6}),
            ],
        )

    install_routes(page, {"/api/drafts": draft_stream})
    open_app(page, base_url)
    open_history_session(page)

    page.locator("#generate-draft-btn").click()

    expect(page.locator("#markdown-output")).to_have_value(re.compile("途中まで生成してから完成します。"))
    expect(page.locator("#evaluation-summary")).to_contain_text("PASS")
    expect(page.locator("#verification-summary")).to_contain_text("stub verified")
    expect(page.locator("#draft-status")).to_contain_text("score 4.6")
    expect(page.locator("#cancel-draft-btn")).to_be_hidden()
    expect(page.locator("#generate-draft-btn")).to_be_enabled()

    page.unroute("**/api/**")
    state = install_routes(page, {"/api/drafts": hold_request("draft")})
    page.reload()
    expect(page.locator("#history-status")).to_contain_text("1件のプロジェクト")
    open_history_session(page)

    page.locator("#generate-draft-btn").evaluate("button => button.click()")
    expect(page.locator("#cancel-draft-btn")).to_be_visible()
    page.locator("#cancel-draft-btn").click()
    abort_pending(state, "draft")

    expect(page.locator("#cancel-draft-btn")).to_be_hidden()
    expect(page.locator("#generate-draft-btn")).to_be_enabled()
    expect(page.locator("#draft-status")).to_contain_text("停止しました")


def test_edit_answer_calls_fork_endpoint_and_updates_transcript(page: Page, base_url: str) -> None:
    def fork_answer(route: Route, request: Request, call: dict[str, Any], state: StubState) -> dict[str, Any]:
        assert call["method"] == "POST"
        assert call["payload"]["content"] == "Forked answer with sharper direction"
        return {
            "session_id": "session-fork",
            "parent_session_id": "session-history",
            "completed": False,
            "questions": QUESTIONS,
            "answers": [
                {
                    "question_id": "theme",
                    "content": "Forked answer with sharper direction",
                    "flow_type": "main",
                }
            ],
            "next_question": QUESTIONS[1],
        }

    state = install_routes(
        page,
        {
            "/api/brief-sessions/session-history/answers/theme/edit": fork_answer,
        },
    )
    open_app(page, base_url)
    open_history_session(page)

    page.locator(".answer-edit-btn").first.click()
    page.locator(".answer-edit-input").fill("Forked answer with sharper direction")
    page.get_by_role("button", name="保存して分岐").click()

    expect(page.locator("#question-log")).to_contain_text("Forked answer with sharper direction")
    expect(page.locator("#question-log")).to_contain_text("想定読者は誰ですか？")
    expect(page.locator("#question-log")).not_to_contain_text("Original history answer")
    assert state.calls_to("/api/brief-sessions/session-history/answers/theme/edit")


def test_section_regeneration_candidate_reject_and_accept_flow(page: Page, base_url: str) -> None:
    replacements = [
        "## Target Section\n\n破棄する候補です。\n",
        "## Target Section\n\n採用した再生成候補です。\n",
    ]

    def regenerate(route: Route, request: Request, call: dict[str, Any], state: StubState) -> dict[str, Any]:
        assert call["method"] == "POST"
        assert call["payload"]["style_profile_id"] == "style-history"
        assert call["payload"]["session_id"] == "session-history"
        assert call["payload"]["section_anchor"] == "target-section"
        assert "古い対象セクションです。" in call["payload"]["draft_markdown"]
        replacement = replacements.pop(0)
        return {
            "draft_id": "draft-regenerated",
            "section": {"anchor": "target-section", "heading": "Target Section"},
            "replacement_markdown": replacement,
        }

    state = install_routes(
        page,
        {
            "/api/drafts/session-history/regenerate-section": regenerate,
        },
    )
    open_app(page, base_url)
    open_history_draft(page)

    page.get_by_role("button", name="Markdown").click()
    markdown = page.locator("#markdown-output")
    markdown.focus()
    target_offset = DRAFT_MARKDOWN.index("古い対象セクション")
    markdown.evaluate(
        "(textarea, offset) => { textarea.setSelectionRange(offset, offset); textarea.dispatchEvent(new Event('click', { bubbles: true })); }",
        target_offset,
    )
    expect(page.locator("#section-status")).to_contain_text("Target Section")
    expect(page.locator("#regenerate-section-btn")).to_be_enabled()

    page.locator("#regenerate-section-btn").click()
    expect(page.locator("#section-candidate")).to_be_visible()
    expect(page.locator("#section-candidate-output")).to_have_value("## Target Section\n\n破棄する候補です。\n")
    page.locator("#reject-section-btn").click()
    expect(page.locator("#section-candidate")).to_be_hidden()
    expect(markdown).to_have_value(re.compile("古い対象セクションです。"))

    page.locator("#regenerate-section-btn").click()
    expect(page.locator("#section-candidate-output")).to_have_value("## Target Section\n\n採用した再生成候補です。\n")
    page.locator("#accept-section-btn").click()

    expect(page.locator("#section-candidate")).to_be_hidden()
    expect(markdown).to_have_value(re.compile("採用した再生成候補です。"))
    expect(markdown).not_to_have_value(re.compile("古い対象セクションです。"))
    expect(page.locator("#preview-content")).to_contain_text("採用した再生成候補です。")
    assert len(state.calls_to("/api/drafts/session-history/regenerate-section")) == 2
