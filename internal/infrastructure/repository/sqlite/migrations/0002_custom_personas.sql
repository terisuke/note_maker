CREATE TABLE IF NOT EXISTS custom_personas (
	id TEXT PRIMARY KEY,
	display_name TEXT NOT NULL,
	default_format TEXT NOT NULL,
	persona_json TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_custom_personas_default_format ON custom_personas(default_format);
