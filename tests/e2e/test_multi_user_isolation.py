from __future__ import annotations

import sqlite3
from pathlib import Path
from typing import Any

from e2e_server import E2EServer, start_server


def test_multi_user_header_isolation_hides_other_users_drafts(
    repo_root: Path,
    tmp_path: Path,
    playwright_instance: Any,
) -> None:
    server = start_server(
        repo_root,
        tmp_path,
        extra_env={
            "MULTI_USER": "1",
            "WORKFLOW_STORE_DRIVER": "sqlite",
            "WORKFLOW_STORE_PATH": str(tmp_path / "workflow_store.db"),
        },
    )
    try:
        _seed_user_draft(tmp_path / "workflow_store.db", "alice", "alice-project", "alice-article", "alice-draft")
        _seed_user_draft(tmp_path / "workflow_store.db", "bob", "bob-project", "bob-article", "bob-draft")

        alice = _request_context(playwright_instance, server, "alice")
        bob = _request_context(playwright_instance, server, "bob")
        anonymous = playwright_instance.request.new_context(base_url=server.base_url)
        try:
            shared_persona = _persona_payload("shared-writer")
            assert alice.post("/api/personas", data=shared_persona).status == 201
            assert bob.post("/api/personas", data=shared_persona).status == 409

            alice_response = alice.get("/api/workflow/artifacts")
            assert alice_response.ok
            alice_payload = alice_response.json()
            assert [draft["id"] for draft in alice_payload["drafts"]] == ["alice-draft"]
            assert "bob-draft" not in str(alice_payload)

            bob_response = bob.get("/api/workflow/artifacts")
            assert bob_response.ok
            bob_payload = bob_response.json()
            assert [draft["id"] for draft in bob_payload["drafts"]] == ["bob-draft"]
            assert "alice-draft" not in str(bob_payload)

            assert anonymous.get("/api/workflow/artifacts").status == 401
            assert anonymous.get("/healthz").status == 204
        finally:
            alice.dispose()
            bob.dispose()
            anonymous.dispose()
    finally:
        server.stop()


def _request_context(playwright_instance: Any, server: E2EServer, user_id: str) -> Any:
    return playwright_instance.request.new_context(
        base_url=server.base_url,
        extra_http_headers={"X-Forwarded-User": user_id},
    )


def _seed_user_draft(db_path: Path, user_id: str, project_id: str, article_id: str, draft_id: str) -> None:
    timestamp = "2026-05-04T09:00:00Z"
    with sqlite3.connect(db_path) as db:
        db.execute(
            """
            INSERT INTO projects (id, name, created_at, updated_at, metadata_json, user_id)
            VALUES (?, ?, ?, ?, '{}', ?)
            """,
            (project_id, f"{user_id.title()} project", timestamp, timestamp, user_id),
        )
        db.execute(
            """
            INSERT INTO articles (
                id, project_id, persona_id, output_format_id, brief_session_id,
                current_draft_id, title, created_at, updated_at, metadata_json, user_id
            )
            VALUES (?, ?, 'terisuke', 'note_article', '', ?, ?, ?, ?, '{}', ?)
            """,
            (article_id, project_id, draft_id, f"{user_id.title()} article", timestamp, timestamp, user_id),
        )
        db.execute(
            """
            INSERT INTO drafts (
                id, article_id, session_id, style_profile_id, persona_id, output_format_id,
                version, markdown, content_hash, evaluation_json, verification_json,
                question_template_version, created_at, user_id
            )
            VALUES (?, ?, '', '', 'terisuke', 'note_article', 1, ?, ?, '{}', '{}', 'brief-template/v1', ?, ?)
            """,
            (
                draft_id,
                article_id,
                f"# {user_id.title()} draft\n\nThis draft belongs to {user_id}.",
                f"{draft_id}-hash",
                timestamp,
                user_id,
            ),
        )


def _persona_payload(persona_id: str) -> dict[str, Any]:
    return {
        "id": persona_id,
        "display_name": "Shared Writer",
        "description": "Same natural ID conflict smoke.",
        "default_format": "note_article",
        "sources": [{"kind": "note", "ref": "shared-writer"}],
        "voice_notes": {"tone": "Calm", "first_person": ["私"], "title_patterns": []},
    }
