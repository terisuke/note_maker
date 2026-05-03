CREATE TABLE IF NOT EXISTS brief_versions (
	session_id TEXT NOT NULL,
	version INTEGER NOT NULL,
	style_profile_id TEXT NOT NULL,
	persona_id TEXT NOT NULL DEFAULT '',
	output_format_id TEXT NOT NULL DEFAULT '',
	brief_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (session_id, version),
	FOREIGN KEY (session_id) REFERENCES briefs(session_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_brief_versions_session ON brief_versions(session_id, version);

INSERT INTO brief_versions (
	session_id, version, style_profile_id, persona_id, output_format_id, brief_json, created_at
)
SELECT session_id, 1, style_profile_id, persona_id, output_format_id, brief_json, created_at
FROM briefs
WHERE NOT EXISTS (
	SELECT 1
	FROM brief_versions
	WHERE brief_versions.session_id = briefs.session_id
);
