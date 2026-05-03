import json
import re
from urllib.parse import parse_qs, urlparse

from playwright.sync_api import Page, expect


CONFIG_KEY = "note-maker-config-v1"

MODELS = [
    "gemma4:e2b",
    "qwen3.6:27b",
    "gemma4:31b",
    "gemma4:latest",
    "llama3.2:8b",
]

PERSONAS = [
    {
        "id": "terisuke",
        "display_name": "Terisuke",
        "description": "Practical local-first engineering notes.",
        "default_format": "note_article",
        "voice_notes": {"first_person": ["僕"]},
        "sources": [
            {"kind": "note", "ref": "cor_instrument"},
            {"kind": "github", "ref": "Cor-Incorporated/corsweb2024/src/content/blog/ja"},
        ],
    },
    {
        "id": "cloudia",
        "display_name": "Cloudia",
        "description": "Technical cloud and platform writing.",
        "default_format": "zenn_article",
        "voice_notes": {"first_person": ["私たち"]},
        "sources": [
            {"kind": "zenn", "ref": "cloudia"},
            {"kind": "qiita", "ref": "Cloudia_Cor_Inc"},
        ],
    },
]

FORMATS = [
    {"id": "note_article", "display_name": "note"},
    {"id": "markdown_blog", "display_name": "Company Blog"},
    {"id": "zenn_article", "display_name": "Zenn"},
    {"id": "qiita_article", "display_name": "Qiita"},
]


def question(question_id, text, required=True):
    return {
        "id": question_id,
        "text": text,
        "flow_type": "main",
        "target_field": question_id,
        "required": required,
    }


TEMPLATES = {
    ("terisuke", "note_article"): [
        question("theme", "何について書きますか？"),
        question("opening_episode", "冒頭で使う具体的な出来事は？"),
    ],
    ("terisuke", "markdown_blog"): [
        question("theme", "ブログ記事のテーマは？"),
        question("cor_blog_purpose", "会社ブログで達成したい目的は？"),
    ],
    ("cloudia", "zenn_article"): [
        question("theme", "技術記事のテーマは？"),
        question("target_stack", "対象スタックは？"),
        question("cloudia_viewpoint", "Cloudiaとして強調する視点は？", required=False),
    ],
    ("cloudia", "qiita_article"): [
        question("theme", "Qiita記事のテーマは？"),
        question("code_examples", "含めたいコード例は？"),
        question("cloudia_viewpoint", "Cloudiaとして強調する視点は？", required=False),
    ],
}


def test_model_config_persists_and_restores_from_local_storage(page: Page, base_url: str):
    install_app_routes(page)

    page.goto(app_url(base_url))
    wait_for_config_loaded(page)

    page.select_option("#style-model", "llama3.2:8b")
    page.select_option("#brief-model", "gemma4:31b")
    page.select_option("#draft-model", "qwen3.6:27b")
    page.select_option("#verify-model", "gemma4:e2b")

    saved = read_config(page)
    assert saved["models"] == {
        "style": "llama3.2:8b",
        "brief": "gemma4:31b",
        "draft": "qwen3.6:27b",
        "verify": "gemma4:e2b",
    }

    page.reload()
    wait_for_config_loaded(page)

    expect(page.locator("#style-model")).to_have_value("llama3.2:8b")
    expect(page.locator("#brief-model")).to_have_value("gemma4:31b")
    expect(page.locator("#draft-model")).to_have_value("qwen3.6:27b")
    expect(page.locator("#verify-model")).to_have_value("gemma4:e2b")


def test_persona_format_template_switching_and_custom_question_crud_migration(
    page: Page,
    base_url: str,
):
    captured = install_app_routes(page)
    page.add_init_script(
        """
        localStorage.setItem("note-maker-config-v1", JSON.stringify({
          mode: { persona: "cloudia", format: "qiita_article" },
          models: { style: "gemma4:e2b", brief: "qwen3.6:27b", draft: "gemma4:31b", verify: "gemma4:latest" },
          questions: [
            { id: "theme", text: "legacy template should not survive" },
            { id: "legacy_custom", text: "Migrated custom question", flow_type: "main", target_field: "custom" }
          ]
        }));
        """
    )

    page.goto(app_url(base_url))
    wait_for_config_loaded(page)
    expect(page.locator("#question-config-list")).to_contain_text("Qiita記事のテーマは？")
    expect(page.locator("#question-config-list")).to_contain_text("含めたいコード例は？")
    expect(page.locator('input[aria-label="追加質問"]')).to_have_value("Migrated custom question")
    expect(page.locator("#question-config-list")).not_to_contain_text("legacy template should not survive")

    migrated = read_config(page)
    assert "questions" not in migrated
    assert migrated["customQuestions"] == [
        {
            "id": "legacy_custom",
            "text": "Migrated custom question",
            "flow_type": "main",
            "target_field": "custom",
        }
    ]

    page.click("#add-question-btn")
    custom_inputs = page.locator('input[aria-label="追加質問"]')
    expect(custom_inputs).to_have_count(2)
    custom_inputs.nth(1).fill("What operational risk should the article cover?")

    saved_after_add = read_config(page)
    assert [item["text"] for item in saved_after_add["customQuestions"]] == [
        "Migrated custom question",
        "What operational risk should the article cover?",
    ]

    page.locator("#question-config-list .question-config-row.custom").nth(0).get_by_role(
        "button",
        name="削除",
    ).click()
    expect(page.locator('input[aria-label="追加質問"]')).to_have_value(
        "What operational risk should the article cover?"
    )
    assert [item["text"] for item in read_config(page)["customQuestions"]] == [
        "What operational risk should the article cover?"
    ]

    page.select_option("#persona-select", "terisuke")
    expect(page.locator("#format-select")).to_have_value("note_article")
    expect(page.locator("#mode-summary")).to_contain_text("Terisuke × note")
    expect(page.locator("#question-config-list")).to_contain_text("何について書きますか？")

    page.select_option("#format-select", "markdown_blog")
    expect(page.locator("#mode-summary")).to_contain_text("Terisuke × Company Blog")
    expect(page.locator("#question-config-list")).to_contain_text("会社ブログで達成したい目的は？")

    page.click("#reset-questions-btn")
    expect(page.locator('input[aria-label="追加質問"]')).to_have_count(0)
    expect(page.locator("#question-config-list")).to_contain_text("会社ブログで達成したい目的は？")
    assert read_config(page)["customQuestions"] == []
    assert ("terisuke", "markdown_blog") in captured["template_requests"]


def test_start_interview_sends_custom_questions_in_payload(page: Page, base_url: str):
    captured = install_app_routes(page)

    page.goto(app_url(base_url))
    wait_for_config_loaded(page)

    page.select_option("#persona-select", "cloudia")
    expect(page.locator("#format-select")).to_have_value("zenn_article")
    expect(page.locator("#question-config-list")).to_contain_text("対象スタックは？")

    page.select_option("#brief-model", "qwen3.6:27b")
    page.click("#add-question-btn")
    page.locator('input[aria-label="追加質問"]').last.fill("Which migration trap should be explained?")
    page.click("#add-question-btn")
    page.locator('input[aria-label="追加質問"]').last.fill("What rollback signal should readers monitor?")

    page.click("#use-preset-style-btn")
    expect(page.locator("#start-interview-btn")).to_be_enabled()
    page.click("#start-interview-btn")

    expect(page.locator("#interview-area")).to_be_visible()
    payload = captured["session_requests"][0]
    assert payload["style_profile_id"] == "profile-cloudia-zenn_article"
    assert payload["persona_id"] == "cloudia"
    assert payload["output_format_id"] == "zenn_article"
    assert payload["brief_model"] == "qwen3.6:27b"
    assert [item["text"] for item in payload["questions"]] == [
        "Which migration trap should be explained?",
        "What rollback signal should readers monitor?",
    ]
    assert {item["target_field"] for item in payload["questions"]} == {"custom"}
    assert "target_stack" not in {item["id"] for item in payload["questions"]}

    expect(page.locator("#question-log")).to_contain_text("技術記事のテーマは？")

    page.fill("#answer-input", "A deterministic browser route stub captured this answer.")
    page.click("#submit-answer-btn")
    expect(page.locator("#question-log")).to_contain_text("対象スタックは？")
    assert captured["answer_requests"][0]["brief_model"] == "qwen3.6:27b"


def install_app_routes(page: Page):
    captured = {
        "template_requests": [],
        "seed_requests": [],
        "session_requests": [],
        "answer_requests": [],
    }

    page.route(
        "https://cdnjs.cloudflare.com/ajax/libs/marked/4.3.0/marked.min.js",
        lambda route: route.fulfill(
            status=200,
            content_type="application/javascript",
            body="window.marked = { parse: (value) => String(value || '') };",
        ),
    )
    page.route("**/api/**", lambda route: dispatch_api(route, page, captured))
    return captured


def dispatch_api(route, page: Page, captured):
    request = route.request
    parsed = urlparse(request.url)
    path = parsed.path
    method = request.method

    if method == "GET" and path == "/api/models":
        fulfill_json(route, MODELS)
        return

    if method == "GET" and path == "/api/personas":
        fulfill_json(route, PERSONAS)
        return

    if method == "GET" and path == "/api/formats":
        fulfill_json(route, FORMATS)
        return

    if method == "GET" and path == "/api/config/storage":
        fulfill_json(
            route,
            {
                "active_driver": "json",
                "active_path": "tmp/e2e-workflow-store.json",
                "configured_driver": "json",
                "configured_path": "tmp/e2e-workflow-store.json",
                "env_locked": False,
                "restart_required": False,
                "restart_message": "",
            },
        )
        return

    if method == "GET" and path == "/api/workflow/artifacts":
        fulfill_json(
            route,
            {
                "projects": [],
                "articles": [],
                "drafts": [],
                "author_styles": [],
                "brief_sessions": [],
            },
        )
        return

    if method == "GET" and path == "/api/brief-sessions/templates":
        query = parse_qs(parsed.query)
        persona_id = query.get("persona_id", ["terisuke"])[0]
        format_id = query.get("format_id", ["note_article"])[0]
        captured["template_requests"].append((persona_id, format_id))
        fulfill_json(route, {"questions": TEMPLATES.get((persona_id, format_id), [])})
        return

    if method == "POST" and path == "/api/author-style/seed":
        body = request_json(request)
        captured["seed_requests"].append(body)
        persona_id = body.get("persona_id", "terisuke")
        format_id = body.get("output_format_id", "note_article")
        fulfill_json(
            route,
            {
                "profile_id": f"profile-{persona_id}-{format_id}",
                "guide_id": f"guide-{persona_id}-{format_id}",
                "article_count": 3,
                "guide_markdown": "# Style Guide\n- Keep claims concrete.\n- Match selected persona.",
            },
        )
        return

    if method == "POST" and path == "/api/brief-sessions":
        body = request_json(request)
        captured["session_requests"].append(body)
        questions = TEMPLATES.get((body.get("persona_id"), body.get("output_format_id")), [])
        fulfill_json(
            route,
            {
                "session_id": "session-browser-e2e",
                "parent_session_id": "",
                "next_question": questions[0],
                "answers": [],
            },
        )
        return

    if method == "POST" and re.fullmatch(r"/api/brief-sessions/[^/]+/answers", path):
        body = request_json(request)
        captured["answer_requests"].append(body)
        payload = {
            "session_id": "session-browser-e2e",
            "parent_session_id": "",
            "completed": False,
            "answers": [
                {
                    "question_id": "theme",
                    "content": body.get("content", ""),
                    "flow_type": "main",
                }
            ],
            "next_question": question("target_stack", "対象スタックは？"),
        }
        route.fulfill(
            status=200,
            content_type="text/event-stream",
            body=f"event: result\ndata: {json.dumps(payload)}\n\n",
        )
        return

    route.fulfill(
        status=404,
        content_type="application/json",
        body=json.dumps({"error": {"message": f"unexpected {method} {path}"}}),
    )


def fulfill_json(route, value, status=200):
    route.fulfill(
        status=status,
        content_type="application/json",
        body=json.dumps(value),
    )


def request_json(request):
    return json.loads(request.post_data or "{}")


def app_url(base_url: str) -> str:
    return base_url.rstrip("/") + "/"


def wait_for_config_loaded(page: Page):
    page.wait_for_function(
        """
        () => document.querySelector("#style-model")?.options.length > 0
          && document.querySelector("#persona-select")?.options.length > 0
          && document.querySelector("#format-select")?.options.length > 0
          && !document.querySelector("#question-config-list")?.textContent.includes("読み込んでいます")
        """
    )


def read_config(page: Page):
    return page.evaluate(f"JSON.parse(localStorage.getItem({json.dumps(CONFIG_KEY)}))")
