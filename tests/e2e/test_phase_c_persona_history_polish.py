import re
from typing import Any

from playwright.sync_api import Page, Route, Request, expect

from test_history_stream_regenerate import (
    BRIEF,
    HISTORY_INDEX,
    PERSONAS,
    QUESTIONS,
    SESSION_DETAIL,
    STYLE_DETAIL,
    StubState,
    fulfill_json,
    install_routes,
    open_app,
    open_history_session,
    select_when_enabled,
)


PHASE_C_PERSONA = {
    "id": "phase-c-writer",
    "display_name": "Phase C Writer",
    "description": "Persona added from browser E2E",
    "default_format": "note_article",
    "voice_notes": {"first_person": ["私"], "tone": "direct"},
    "sources": [{"kind": "note", "ref": "phase-c-feed"}],
}

PHASE_C_STYLE = {
    **STYLE_DETAIL,
    "profile_id": "phase-c-style",
    "guide_id": "phase-c-guide",
    "title": "Phase C 文体ガイド",
    "persona_id": "phase-c-writer",
    "guide_markdown": "# Phase C 文体ガイド\n\n- 追加ペルソナの履歴です",
}

PHASE_C_SESSION = {
    **SESSION_DETAIL,
    "session_id": "phase-c-session",
    "title": "Phase C 取材セッション",
    "style_profile_id": "phase-c-style",
    "persona_id": "phase-c-writer",
    "brief": {
        **BRIEF,
        "theme": "追加ペルソナの履歴を開く",
        "persona_id": "phase-c-writer",
        "style_profile_id": "phase-c-style",
    },
    "questions": QUESTIONS,
}

PHASE_C_HISTORY_INDEX = {
    "style_guides": [
        {
            "profile_id": "phase-c-style",
            "title": "Phase C 文体ガイド",
            "persona_id": "phase-c-writer",
            "output_format_id": "note_article",
            "updated_at": "2026-05-03T08:00:00Z",
        }
    ],
    "sessions": [
        {
            "session_id": "phase-c-session",
            "title": "Phase C 取材セッション",
            "style_profile_id": "phase-c-style",
            "persona_id": "phase-c-writer",
            "output_format_id": "note_article",
            "completed": True,
            "updated_at": "2026-05-03T08:00:00Z",
        }
    ],
    "projects": [],
    "articles": [],
    "drafts": [],
}


def phase_c_locator(page: Page, *selectors: str):
    for selector in selectors:
        if page.locator(selector).count():
            return page.locator(selector)
    return page.locator(selectors[0])


def test_add_persona_updates_current_and_history_selectors_after_reload(page: Page, base_url: str) -> None:
    personas = [*PERSONAS]

    def personas_handler(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        if call["method"] == "GET":
            fulfill_json(route, personas)
            return None
        assert call["method"] == "POST"
        payload = call["payload"]
        assert payload["id"] == PHASE_C_PERSONA["id"]
        assert payload["display_name"] == PHASE_C_PERSONA["display_name"]
        assert payload["default_format"] == "note_article"
        assert payload["sources"][0]["ref"] == "phase-c-feed"
        personas.append(PHASE_C_PERSONA)
        fulfill_json(route, PHASE_C_PERSONA, status=201)
        return None

    def history_handler(route: Route, request: Request, call: dict[str, Any], state: StubState) -> dict[str, Any]:
        persona_id = call["query"].get("persona_id", [""])[0]
        return PHASE_C_HISTORY_INDEX if persona_id == "phase-c-writer" else HISTORY_INDEX

    state = install_routes(
        page,
        {
            "/api/personas": personas_handler,
            "/api/workflow/artifacts": history_handler,
            "/api/author-style/phase-c-style": lambda *_: PHASE_C_STYLE,
            "/api/brief-sessions/phase-c-session": lambda *_: PHASE_C_SESSION,
        },
        clear_storage=False,
    )
    open_app(page, base_url)

    phase_c_locator(page, "#add-persona-toggle-btn", "#add-persona-btn").click()

    expect(page.locator("#add-persona-form")).to_be_visible()
    page.locator("#persona-id-input").fill("phase-c-writer")
    phase_c_locator(page, "#persona-name-input", "#persona-display-name-input").fill("Phase C Writer")
    page.locator("#persona-description-input").fill("Persona added from browser E2E")
    page.locator("#persona-default-format-select").select_option("note_article")
    phase_c_locator(page, "#persona-voice-input", "#persona-first-person-input").fill("私")
    page.locator("#persona-source-kind-input").fill("note")
    page.locator("#persona-source-ref-input").fill("phase-c-feed")
    page.locator("#save-persona-btn").click()

    expect(page.locator("#persona-select")).to_have_value("phase-c-writer")
    expect(page.locator("#history-persona-select")).to_have_value("phase-c-writer")
    expect(page.locator("#history-status")).to_contain_text("1件の文体ガイド")
    select_when_enabled(page, "#history-style-select", "phase-c-style")
    select_when_enabled(page, "#history-session-select", "phase-c-session")
    expect(page.locator("#brief-card")).to_contain_text("追加ペルソナの履歴を開く")

    page.reload()

    expect(page.locator('#persona-select option[value="phase-c-writer"]')).to_have_text("Phase C Writer")
    expect(page.locator('#history-persona-select option[value="phase-c-writer"]')).to_have_text("Phase C Writer")
    page.locator("#persona-select").select_option("phase-c-writer")
    expect(page.locator("#history-persona-select")).to_have_value("phase-c-writer")
    expect(page.locator("#history-status")).to_contain_text("1件の文体ガイド")
    assert state.calls_to("/api/personas")[0]["method"] == "GET"
    assert any(call["method"] == "POST" for call in state.calls_to("/api/personas"))


def test_custom_persona_edit_and_delete_controls_call_product_memory_api(page: Page, base_url: str) -> None:
    personas = [*PERSONAS, PHASE_C_PERSONA]
    updated_persona = {
        **PHASE_C_PERSONA,
        "display_name": "Phase C Writer Updated",
        "description": "Edited from browser E2E",
        "voice_notes": {"first_person": ["僕"]},
        "sources": [{"kind": "zenn", "ref": "phase-c-edited-feed"}],
    }

    def personas_handler(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        assert call["method"] == "GET"
        fulfill_json(route, personas)
        return None

    def persona_detail_handler(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        nonlocal personas
        if call["method"] == "PATCH":
            payload = call["payload"]
            assert payload["id"] == PHASE_C_PERSONA["id"]
            assert payload["display_name"] == updated_persona["display_name"]
            assert payload["description"] == updated_persona["description"]
            assert payload["default_format"] == "note_article"
            assert payload["voice_notes"]["first_person"] == ["僕"]
            assert payload["sources"][0]["kind"] == "zenn"
            assert payload["sources"][0]["ref"] == "phase-c-edited-feed"
            personas = [updated_persona if item["id"] == PHASE_C_PERSONA["id"] else item for item in personas]
            fulfill_json(route, updated_persona)
            return None
        assert call["method"] == "DELETE"
        personas = [item for item in personas if item["id"] != PHASE_C_PERSONA["id"]]
        route.fulfill(status=204, body="")
        return None

    state = install_routes(
        page,
        {
            "/api/personas": personas_handler,
            "/api/personas/phase-c-writer": persona_detail_handler,
        },
    )
    open_app(page, base_url)

    page.locator("#persona-select").select_option("phase-c-writer")
    expect(page.locator("#history-persona-select")).to_have_value("phase-c-writer")
    page.locator("#edit-persona-btn").click()

    expect(page.locator("#add-persona-form")).to_be_visible()
    expect(page.locator("#persona-form-title")).to_have_text("書き手を編集")
    expect(page.locator("#persona-id-input")).to_be_disabled()
    expect(page.locator("#persona-id-input")).to_have_value("phase-c-writer")
    phase_c_locator(page, "#persona-name-input", "#persona-display-name-input").fill(updated_persona["display_name"])
    page.locator("#persona-description-input").fill(updated_persona["description"])
    phase_c_locator(page, "#persona-voice-input", "#persona-first-person-input").fill("僕")
    page.locator("#persona-source-kind-input").fill("zenn")
    page.locator("#persona-source-ref-input").fill("phase-c-edited-feed")
    page.locator("#save-persona-btn").click()

    expect(page.locator("#persona-status")).to_contain_text("更新しました")
    expect(page.locator('#persona-select option[value="phase-c-writer"]')).to_have_text(updated_persona["display_name"])
    expect(page.locator('#history-persona-select option[value="phase-c-writer"]')).to_have_text(updated_persona["display_name"])

    page.once("dialog", lambda dialog: dialog.accept())
    page.locator("#delete-persona-btn").click()

    expect(page.locator("#persona-status")).to_contain_text("削除しました")
    expect(page.locator("#persona-select")).to_have_value("terisuke")
    expect(page.locator("#history-persona-select")).to_have_value("terisuke")
    expect(page.locator('#persona-select option[value="phase-c-writer"]')).to_have_count(0)
    assert [call["method"] for call in state.calls_to("/api/personas/phase-c-writer")] == ["PATCH", "DELETE"]


def test_saved_history_brief_card_edit_save_cancel_and_error(page: Page, base_url: str) -> None:
    patch_calls = 0
    updated_brief = {
        **BRIEF,
        "theme": "保存後のブリーフテーマ",
        "reader": "保存状態を確認する編集者",
    }

    def patch_brief(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        nonlocal patch_calls
        assert call["method"] == "PATCH"
        patch_calls += 1
        if patch_calls == 1:
            assert call["payload"]["fields"]["theme"] == updated_brief["theme"]
            fulfill_json(route, updated_brief)
            return None
        fulfill_json(route, {"error": {"message": "brief save failed"}}, status=500)
        return None

    state = install_routes(page, {"/api/briefs/session-history": patch_brief})
    open_app(page, base_url)
    open_history_session(page)

    page.locator("#edit-brief-btn").click()
    expect(page.locator("#brief-edit-form")).to_be_visible()
    page.locator("#brief-theme-input").fill("キャンセルされるテーマ")
    page.locator("#cancel-brief-edit-btn").click()
    expect(page.locator("#brief-card")).to_contain_text("履歴から再開する記事")
    expect(page.locator("#brief-card")).not_to_contain_text("キャンセルされるテーマ")

    page.locator("#edit-brief-btn").click()
    page.locator("#brief-theme-input").fill(updated_brief["theme"])
    page.locator("#brief-reader-input").fill(updated_brief["reader"])
    page.locator("#save-brief-edit-btn").click()
    expect(page.locator("#brief-card")).to_contain_text(updated_brief["theme"])
    expect(page.locator("#brief-card")).to_contain_text(updated_brief["reader"])
    expect(page.locator("#brief-edit-status")).to_contain_text(re.compile("保存"))

    page.locator("#edit-brief-btn").click()
    page.locator("#brief-theme-input").fill("失敗するブリーフテーマ")
    page.locator("#save-brief-edit-btn").click()
    expect(page.locator("#brief-edit-status")).to_contain_text(re.compile("失敗|failed|error", re.I))
    expect(page.locator("#brief-card")).not_to_contain_text("失敗するブリーフテーマ")
    assert len(state.calls_to("/api/briefs/session-history")) == 2


def test_saved_history_brief_card_shows_version_history(page: Page, base_url: str) -> None:
    versions = {
        "versions": [
            {
                "session_id": "session-history",
                "version": 1,
                "created_at": "2026-05-01T08:00:00Z",
                "brief": {**BRIEF, "theme": "編集前のブリーフテーマ", "reader": "初稿を確認する編集者"},
            },
            {
                "session_id": "session-history",
                "version": 2,
                "created_at": "2026-05-03T09:00:00Z",
                "brief": {**BRIEF, "theme": "保存後のブリーフテーマ", "reader": "保存状態を確認する編集者"},
            },
        ]
    }

    state = install_routes(page, {"/api/briefs/session-history/versions": lambda *_: versions})
    open_app(page, base_url)
    open_history_session(page)

    page.locator("#show-brief-versions-btn").click()

    expect(page.locator("#brief-version-history")).to_be_visible()
    expect(page.locator("#brief-version-history")).to_contain_text("v2")
    expect(page.locator("#brief-version-history")).to_contain_text("保存後のブリーフテーマ")
    expect(page.locator("#brief-version-history")).to_contain_text("v1")
    expect(page.locator("#brief-version-history")).to_contain_text("編集前のブリーフテーマ")
    assert [call["method"] for call in state.calls_to("/api/briefs/session-history/versions")] == ["GET"]


def test_saved_history_style_card_edit_save_cancel_and_error(page: Page, base_url: str) -> None:
    patch_calls = 0
    updated_style = {
        **STYLE_DETAIL,
        "guide_markdown": "# 保存後の文体ガイド\n\n- 保存された編集内容です",
    }

    def style_detail(route: Route, request: Request, call: dict[str, Any], state: StubState) -> None:
        nonlocal patch_calls
        if call["method"] == "GET":
            fulfill_json(route, STYLE_DETAIL)
            return None
        assert call["method"] == "PATCH"
        patch_calls += 1
        if patch_calls == 1:
            assert "保存された編集内容" in call["payload"]["guide_markdown"]
            fulfill_json(route, updated_style)
            return None
        fulfill_json(route, {"error": {"message": "style save failed"}}, status=500)
        return None

    state = install_routes(page, {"/api/author-style/style-history": style_detail})
    open_app(page, base_url)
    select_when_enabled(page, "#history-style-select", "style-history")
    expect(page.locator("#style-guide-card")).to_contain_text("具体例から始める")

    page.locator("#edit-style-guide-btn").click()
    expect(page.locator("#style-guide-edit-form")).to_be_visible()
    page.locator("#style-guide-markdown-input").fill("# キャンセルされる文体ガイド\n\n- 保存しない")
    page.locator("#cancel-style-guide-edit-btn").click()
    expect(page.locator("#style-guide-card")).to_contain_text("履歴文体ガイド")
    expect(page.locator("#style-guide-card")).not_to_contain_text("キャンセルされる文体ガイド")

    page.locator("#edit-style-guide-btn").click()
    page.locator("#style-guide-markdown-input").fill(updated_style["guide_markdown"])
    page.locator("#save-style-guide-edit-btn").click()
    expect(page.locator("#style-guide-card")).to_contain_text("保存された編集内容")
    expect(page.locator("#style-guide-edit-status")).to_contain_text(re.compile("保存"))

    page.locator("#edit-style-guide-btn").click()
    page.locator("#style-guide-markdown-input").fill("# 失敗する文体ガイド\n\n- 保存しない")
    page.locator("#save-style-guide-edit-btn").click()
    expect(page.locator("#style-guide-edit-status")).to_contain_text(re.compile("失敗|failed|error", re.I))
    expect(page.locator("#style-guide-card")).not_to_contain_text("失敗する文体ガイド")
    assert len([call for call in state.calls_to("/api/author-style/style-history") if call["method"] == "PATCH"]) == 2
