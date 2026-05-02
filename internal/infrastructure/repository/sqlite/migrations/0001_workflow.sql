CREATE TABLE IF NOT EXISTS schema_migrations (
	version INTEGER PRIMARY KEY,
	name TEXT NOT NULL,
	applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS articles (
	id TEXT PRIMARY KEY,
	project_id TEXT,
	persona_id TEXT NOT NULL,
	output_format_id TEXT NOT NULL,
	brief_session_id TEXT,
	current_draft_id TEXT,
	title TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	metadata_json TEXT NOT NULL DEFAULT '{}',
	FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_articles_project_id ON articles(project_id);
CREATE INDEX IF NOT EXISTS idx_articles_brief_session_id ON articles(brief_session_id);

CREATE TABLE IF NOT EXISTS author_style_results (
	id TEXT PRIMARY KEY,
	profile_id TEXT NOT NULL,
	guide_id TEXT NOT NULL,
	source_json TEXT NOT NULL,
	profile_json TEXT NOT NULL,
	guide_json TEXT NOT NULL,
	article_count INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_author_style_results_profile_id ON author_style_results(profile_id);
CREATE INDEX IF NOT EXISTS idx_author_style_results_guide_id ON author_style_results(guide_id);

CREATE TABLE IF NOT EXISTS author_source_articles (
	analysis_id TEXT NOT NULL,
	position INTEGER NOT NULL,
	article_id TEXT NOT NULL DEFAULT '',
	url TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	fetched_at TEXT NOT NULL,
	content_hash TEXT NOT NULL DEFAULT '',
	source_json TEXT NOT NULL,
	PRIMARY KEY (analysis_id, position),
	FOREIGN KEY (analysis_id) REFERENCES author_style_results(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_author_source_articles_hash ON author_source_articles(content_hash);

CREATE TABLE IF NOT EXISTS source_selector_snapshots (
	id TEXT PRIMARY KEY,
	scope_type TEXT NOT NULL,
	scope_id TEXT NOT NULL,
	selector_json TEXT NOT NULL,
	profile_json TEXT,
	article_json TEXT,
	content_hash TEXT NOT NULL DEFAULT '',
	fetched_at TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_source_selector_snapshots_scope ON source_selector_snapshots(scope_type, scope_id);
CREATE INDEX IF NOT EXISTS idx_source_selector_snapshots_hash ON source_selector_snapshots(content_hash);

CREATE TABLE IF NOT EXISTS brief_sessions (
	id TEXT PRIMARY KEY,
	style_profile_id TEXT NOT NULL,
	persona_id TEXT NOT NULL,
	output_format_id TEXT NOT NULL,
	parent_session_id TEXT NOT NULL DEFAULT '',
	phase TEXT NOT NULL,
	completed INTEGER NOT NULL,
	deep_dive_skipped INTEGER NOT NULL,
	question_template_version TEXT NOT NULL DEFAULT '',
	questions_json TEXT NOT NULL,
	answers_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_brief_sessions_style_profile_id ON brief_sessions(style_profile_id);
CREATE INDEX IF NOT EXISTS idx_brief_sessions_persona_format ON brief_sessions(persona_id, output_format_id);

CREATE TABLE IF NOT EXISTS brief_answers (
	session_id TEXT NOT NULL,
	position INTEGER NOT NULL,
	question_id TEXT NOT NULL,
	content TEXT NOT NULL,
	flow_type TEXT NOT NULL,
	target_question_id TEXT NOT NULL DEFAULT '',
	follow_up_index INTEGER NOT NULL DEFAULT 0,
	parent_answer_id TEXT NOT NULL DEFAULT '',
	answer_json TEXT NOT NULL,
	PRIMARY KEY (session_id, position),
	FOREIGN KEY (session_id) REFERENCES brief_sessions(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_brief_answers_question_id ON brief_answers(question_id);

CREATE TABLE IF NOT EXISTS briefs (
	session_id TEXT PRIMARY KEY,
	style_profile_id TEXT NOT NULL,
	persona_id TEXT NOT NULL DEFAULT '',
	output_format_id TEXT NOT NULL DEFAULT '',
	brief_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (session_id) REFERENCES brief_sessions(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS drafts (
	id TEXT PRIMARY KEY,
	article_id TEXT,
	session_id TEXT NOT NULL DEFAULT '',
	style_profile_id TEXT NOT NULL DEFAULT '',
	persona_id TEXT NOT NULL DEFAULT '',
	output_format_id TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL,
	markdown TEXT NOT NULL,
	content_hash TEXT NOT NULL,
	evaluation_json TEXT NOT NULL DEFAULT '{}',
	verification_json TEXT NOT NULL DEFAULT '{}',
	question_template_version TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	UNIQUE (article_id, version),
	FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_drafts_article_id ON drafts(article_id);
CREATE INDEX IF NOT EXISTS idx_drafts_session_id ON drafts(session_id);
CREATE INDEX IF NOT EXISTS idx_drafts_content_hash ON drafts(content_hash);

CREATE TABLE IF NOT EXISTS section_regenerations (
	id TEXT PRIMARY KEY,
	draft_id TEXT NOT NULL,
	article_id TEXT,
	section_anchor TEXT NOT NULL,
	section_heading TEXT NOT NULL DEFAULT '',
	base_version INTEGER NOT NULL,
	version INTEGER NOT NULL,
	replacement_markdown TEXT NOT NULL,
	updated_draft_markdown TEXT NOT NULL,
	updated_content_hash TEXT NOT NULL,
	verification_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	FOREIGN KEY (draft_id) REFERENCES drafts(id) ON DELETE CASCADE,
	FOREIGN KEY (article_id) REFERENCES articles(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_section_regenerations_draft_id ON section_regenerations(draft_id);
CREATE INDEX IF NOT EXISTS idx_section_regenerations_article_id ON section_regenerations(article_id);
