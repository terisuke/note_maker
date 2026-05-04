package sqlite

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	authorstyleapp "github.com/teradakousuke/note_maker/internal/application/authorstyle"
	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

var ErrScopedIDConflict = errors.New("record id belongs to another user")

//go:embed migrations/*.sql
var migrationFiles embed.FS

const (
	driverName             = "sqlite3"
	defaultTemplateVersion = "brief-template/v1"
)

// WorkflowStore persists workflow state in SQLite while keeping the memory
// store's public behavior for current callers.
type WorkflowStore struct {
	db     *sql.DB
	userID string
}

// ProjectRecord is the persisted project aggregate prepared for history UI.
type ProjectRecord struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Metadata  map[string]any
}

// ArticleRecord is one target deliverable inside a project.
type ArticleRecord struct {
	ID             string
	ProjectID      string
	PersonaID      string
	OutputFormatID string
	BriefSessionID string
	CurrentDraftID string
	Title          string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Metadata       map[string]any
}

// SourceSnapshotRecord stores a source selector and the normalized fetch result
// available at the time it was used.
type SourceSnapshotRecord struct {
	ID          string
	ScopeType   string
	ScopeID     string
	Selector    sourcedomain.Ref
	Profile     *sourcedomain.ProfileSnapshot
	Article     *sourcedomain.ArticleSnapshot
	ContentHash string
	FetchedAt   time.Time
	CreatedAt   time.Time
}

// DraftRecord is one persisted generated draft version.
type DraftRecord struct {
	ID                      string
	ArticleID               string
	SessionID               string
	StyleProfileID          string
	PersonaID               string
	OutputFormatID          string
	Version                 int
	Markdown                string
	ContentHash             string
	Evaluation              draftapp.StyleEvaluation
	Verification            draftapp.FinalVerification
	QuestionTemplateVersion string
	CreatedAt               time.Time
}

// SectionRegenerationRecord stores the result of regenerating one section from
// an existing draft version.
type SectionRegenerationRecord struct {
	ID                   string
	DraftID              string
	ArticleID            string
	SectionAnchor        string
	SectionHeading       string
	BaseVersion          int
	Version              int
	ReplacementMarkdown  string
	UpdatedDraftMarkdown string
	UpdatedContentHash   string
	Verification         draftapp.FinalVerification
	CreatedAt            time.Time
}

// NewWorkflowStore opens or creates a SQLite workflow store and applies migrations.
func NewWorkflowStore(path string) (*WorkflowStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("sqlite workflow store path is required")
	}
	if path != ":memory:" {
		path = filepath.Clean(path)
	}
	db, err := sql.Open(driverName, path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite workflow store: %w", err)
	}
	store := &WorkflowStore{db: db, userID: defaultUserID}
	if err := store.configure(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.Migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// NewWorkflowStoreDB wraps an existing database handle and applies migrations.
func NewWorkflowStoreDB(db *sql.DB) (*WorkflowStore, error) {
	if db == nil {
		return nil, fmt.Errorf("sqlite db is required")
	}
	store := &WorkflowStore{db: db, userID: defaultUserID}
	if err := store.configure(); err != nil {
		return nil, err
	}
	if err := store.Migrate(); err != nil {
		return nil, err
	}
	return store, nil
}

// DB returns the underlying database handle for advanced queries in focused tests/tools.
func (s *WorkflowStore) DB() *sql.DB {
	return s.db
}

const defaultUserID = "local"

// ForUser returns a shallow store view scoped to one authenticated principal.
func (s *WorkflowStore) ForUser(userID string) *WorkflowStore {
	if s == nil {
		return nil
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		userID = defaultUserID
	}
	return &WorkflowStore{db: s.db, userID: userID}
}

func (s *WorkflowStore) currentUserID() string {
	if s == nil || strings.TrimSpace(s.userID) == "" {
		return defaultUserID
	}
	return strings.TrimSpace(s.userID)
}

func ensureScopedWrite(result sql.Result, resourceType, resourceID, userID string) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return nil
	}
	if rows == 0 {
		return fmt.Errorf("%w: %s %q for user %q", ErrScopedIDConflict, resourceType, resourceID, userID)
	}
	return nil
}

// Close releases the underlying SQLite connection pool.
func (s *WorkflowStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *WorkflowStore) configure() error {
	if _, err := s.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	return nil
}

// Migrate applies embedded schema migrations once.
func (s *WorkflowStore) Migrate() error {
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("prepare schema migrations table: %w", err)
	}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := migrationVersion(entry.Name())
		if err != nil {
			return err
		}
		var exists int
		err = s.db.QueryRow(`SELECT 1 FROM schema_migrations WHERE version = ?`, version).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", entry.Name(), err)
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.Exec(string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, version, entry.Name(), formatTime(nowUTC())); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", entry.Name(), err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}

// SaveAuthorStyle stores an author style analysis result.
func (s *WorkflowStore) SaveAuthorStyle(result authorstyleapp.AnalyzeResult) error {
	if result.ID == "" {
		return fmt.Errorf("author style result id is required")
	}
	if err := result.Profile.Validate(); err != nil {
		return err
	}
	if err := result.Guide.Validate(); err != nil {
		return err
	}
	createdAt := result.CreatedAt
	if createdAt.IsZero() {
		createdAt = nowUTC()
	}
	sourceJSON, err := marshalString(result.Source)
	if err != nil {
		return fmt.Errorf("encode author source: %w", err)
	}
	profileJSON, err := marshalString(result.Profile)
	if err != nil {
		return fmt.Errorf("encode author profile: %w", err)
	}
	guideJSON, err := marshalString(result.Guide)
	if err != nil {
		return fmt.Errorf("encode writing style guide: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save author style: %w", err)
	}
	defer rollbackUnlessDone(tx)
	resultExec, err := tx.Exec(`
INSERT INTO author_style_results (id, profile_id, guide_id, source_json, profile_json, guide_json, article_count, created_at, user_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	profile_id = excluded.profile_id,
	guide_id = excluded.guide_id,
	source_json = excluded.source_json,
	profile_json = excluded.profile_json,
	guide_json = excluded.guide_json,
	article_count = excluded.article_count,
	created_at = excluded.created_at
	WHERE author_style_results.user_id = excluded.user_id`,
		result.ID, result.Profile.ID, result.Guide.ID, sourceJSON, profileJSON, guideJSON, result.ArticleCount, formatTime(createdAt), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save author style: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "author_style_result", result.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save author style: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM author_source_articles WHERE analysis_id = ? AND user_id = ?`, result.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("replace source articles: %w", err)
	}
	for i, article := range result.Source.Articles {
		articleJSON, err := marshalString(article)
		if err != nil {
			return fmt.Errorf("encode source article %d: %w", i, err)
		}
		contentHash := hashString(strings.Join([]string{article.ID, article.URL, article.Title}, "\x00"))
		_, err = tx.Exec(`
INSERT INTO author_source_articles (analysis_id, position, article_id, url, title, fetched_at, content_hash, source_json, user_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			result.ID, i, article.ID, article.URL, article.Title, formatTime(article.At), contentHash, articleJSON, s.currentUserID())
		if err != nil {
			return fmt.Errorf("save source article %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save author style: %w", err)
	}
	return nil
}

// GetAuthorStyle returns an analysis result by analysis ID, profile ID, or guide ID.
func (s *WorkflowStore) GetAuthorStyle(id string) (authorstyleapp.AnalyzeResult, bool) {
	var result authorstyleapp.AnalyzeResult
	var sourceJSON, profileJSON, guideJSON, createdAt string
	err := s.db.QueryRow(`
SELECT id, source_json, profile_json, guide_json, article_count, created_at
FROM author_style_results
WHERE user_id = ? AND (id = ? OR profile_id = ? OR guide_id = ?)
ORDER BY CASE WHEN id = ? THEN 0 ELSE 1 END, created_at DESC
LIMIT 1`, s.currentUserID(), id, id, id, id).Scan(&result.ID, &sourceJSON, &profileJSON, &guideJSON, &result.ArticleCount, &createdAt)
	if err != nil {
		return authorstyleapp.AnalyzeResult{}, false
	}
	if err := unmarshalString(sourceJSON, &result.Source); err != nil {
		return authorstyleapp.AnalyzeResult{}, false
	}
	if err := unmarshalString(profileJSON, &result.Profile); err != nil {
		return authorstyleapp.AnalyzeResult{}, false
	}
	if err := unmarshalString(guideJSON, &result.Guide); err != nil {
		return authorstyleapp.AnalyzeResult{}, false
	}
	result.CreatedAt = parseTime(createdAt)
	return result, true
}

// ListAuthorStyles returns all stored author style analyses in newest-first order.
func (s *WorkflowStore) ListAuthorStyles() ([]authorstyleapp.AnalyzeResult, error) {
	rows, err := s.db.Query(`
SELECT id, source_json, profile_json, guide_json, article_count, created_at
FROM author_style_results
WHERE user_id = ?
ORDER BY created_at DESC, id`, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list author styles: %w", err)
	}
	defer rows.Close()
	var results []authorstyleapp.AnalyzeResult
	for rows.Next() {
		var result authorstyleapp.AnalyzeResult
		var sourceJSON, profileJSON, guideJSON, createdAt string
		if err := rows.Scan(&result.ID, &sourceJSON, &profileJSON, &guideJSON, &result.ArticleCount, &createdAt); err != nil {
			return nil, fmt.Errorf("scan author style: %w", err)
		}
		if err := unmarshalString(sourceJSON, &result.Source); err != nil {
			return nil, fmt.Errorf("decode author source %q: %w", result.ID, err)
		}
		if err := unmarshalString(profileJSON, &result.Profile); err != nil {
			return nil, fmt.Errorf("decode author profile %q: %w", result.ID, err)
		}
		if err := unmarshalString(guideJSON, &result.Guide); err != nil {
			return nil, fmt.Errorf("decode writing style guide %q: %w", result.ID, err)
		}
		result.CreatedAt = parseTime(createdAt)
		results = append(results, result)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate author styles: %w", err)
	}
	return results, nil
}

// GetProfileAndGuide returns style assets by profile, guide, or analysis ID.
func (s *WorkflowStore) GetProfileAndGuide(id string) (authordomain.AuthorStyleProfile, authordomain.WritingStyleGuide, bool) {
	result, ok := s.GetAuthorStyle(id)
	if !ok {
		return authordomain.AuthorStyleProfile{}, authordomain.WritingStyleGuide{}, false
	}
	return result.Profile, result.Guide, true
}

// SaveSession stores a brief interview session.
func (s *WorkflowStore) SaveSession(session briefdomain.ArticleBriefSession) error {
	if session.ID == "" {
		return fmt.Errorf("brief session id is required")
	}
	if strings.TrimSpace(session.StyleProfileID) == "" {
		return fmt.Errorf("brief session style profile id is required")
	}
	questionsJSON, err := marshalString(session.Questions)
	if err != nil {
		return fmt.Errorf("encode session questions: %w", err)
	}
	answersJSON, err := marshalString(session.Answers)
	if err != nil {
		return fmt.Errorf("encode session answers: %w", err)
	}
	now := nowUTC()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save session: %w", err)
	}
	defer rollbackUnlessDone(tx)
	resultExec, err := tx.Exec(`
INSERT INTO brief_sessions (
	id, style_profile_id, persona_id, output_format_id, parent_session_id, phase,
	completed, deep_dive_skipped, question_template_version, questions_json,
	answers_json, created_at, updated_at, user_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	style_profile_id = excluded.style_profile_id,
	persona_id = excluded.persona_id,
	output_format_id = excluded.output_format_id,
	parent_session_id = excluded.parent_session_id,
	phase = excluded.phase,
	completed = excluded.completed,
	deep_dive_skipped = excluded.deep_dive_skipped,
	question_template_version = excluded.question_template_version,
	questions_json = excluded.questions_json,
	answers_json = excluded.answers_json,
	updated_at = excluded.updated_at
	WHERE brief_sessions.user_id = excluded.user_id`,
		session.ID, session.StyleProfileID, session.PersonaID, session.OutputFormatID, session.ParentSessionID, string(session.Phase),
		boolInt(session.Completed), boolInt(session.DeepDiveSkipped), defaultTemplateVersion, questionsJSON, answersJSON, formatTime(now), formatTime(now), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "brief_session", session.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM brief_answers WHERE session_id = ? AND user_id = ?`, session.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("replace brief answers: %w", err)
	}
	for i, answer := range session.Answers {
		answerJSON, err := marshalString(answer)
		if err != nil {
			return fmt.Errorf("encode brief answer %d: %w", i, err)
		}
		_, err = tx.Exec(`
INSERT INTO brief_answers (
	session_id, position, question_id, content, flow_type,
	target_question_id, follow_up_index, parent_answer_id, answer_json, user_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			session.ID, i, answer.QuestionID, answer.Content, string(answer.FlowType), answer.TargetQuestionID, answer.FollowUpIndex, "", answerJSON, s.currentUserID())
		if err != nil {
			return fmt.Errorf("save brief answer %d: %w", i, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save session: %w", err)
	}
	return nil
}

// GetSession returns a brief interview session by ID.
func (s *WorkflowStore) GetSession(id string) (briefdomain.ArticleBriefSession, bool) {
	var session briefdomain.ArticleBriefSession
	var phase, questionsJSON, answersJSON string
	var completed, deepDiveSkipped int
	err := s.db.QueryRow(`
SELECT id, style_profile_id, persona_id, output_format_id, parent_session_id, phase,
	completed, deep_dive_skipped, questions_json, answers_json
FROM brief_sessions
WHERE id = ? AND user_id = ?`, id, s.currentUserID()).Scan(&session.ID, &session.StyleProfileID, &session.PersonaID, &session.OutputFormatID, &session.ParentSessionID, &phase, &completed, &deepDiveSkipped, &questionsJSON, &answersJSON)
	if err != nil {
		return briefdomain.ArticleBriefSession{}, false
	}
	session.Phase = briefdomain.InterviewPhase(phase)
	session.Completed = completed != 0
	session.DeepDiveSkipped = deepDiveSkipped != 0
	if err := unmarshalString(questionsJSON, &session.Questions); err != nil {
		return briefdomain.ArticleBriefSession{}, false
	}
	if err := unmarshalString(answersJSON, &session.Answers); err != nil {
		return briefdomain.ArticleBriefSession{}, false
	}
	return session, true
}

// ListSessions returns all stored brief interview sessions in newest-first order.
func (s *WorkflowStore) ListSessions() ([]briefdomain.ArticleBriefSession, error) {
	rows, err := s.db.Query(`
SELECT id
FROM brief_sessions
WHERE user_id = ?
ORDER BY updated_at DESC, id`, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan session id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session ids: %w", err)
	}
	sessions := make([]briefdomain.ArticleBriefSession, 0, len(ids))
	for _, id := range ids {
		session, ok := s.GetSession(id)
		if !ok {
			return nil, fmt.Errorf("session %q disappeared while listing", id)
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

// SaveBrief stores the completed brief for a session.
func (s *WorkflowStore) SaveBrief(sessionID string, brief briefdomain.ArticleBrief) error {
	if sessionID == "" {
		return fmt.Errorf("brief session id is required")
	}
	if brief.StyleProfileID == "" {
		return fmt.Errorf("brief style profile id is required")
	}
	briefJSON, err := marshalString(brief)
	if err != nil {
		return fmt.Errorf("encode brief: %w", err)
	}
	now := nowUTC()
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save brief: %w", err)
	}
	defer rollbackUnlessDone(tx)
	var previousStyleProfileID, previousPersonaID, previousOutputFormatID, previousBriefJSON, previousCreatedAt string
	previousErr := tx.QueryRow(`
SELECT style_profile_id, persona_id, output_format_id, brief_json, created_at
FROM briefs
WHERE session_id = ? AND user_id = ?`, sessionID, s.currentUserID()).Scan(&previousStyleProfileID, &previousPersonaID, &previousOutputFormatID, &previousBriefJSON, &previousCreatedAt)
	if previousErr != nil && previousErr != sql.ErrNoRows {
		return fmt.Errorf("load previous brief: %w", previousErr)
	}
	var maxVersion int
	if err := tx.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM brief_versions WHERE session_id = ? AND user_id = ?`, sessionID, s.currentUserID()).Scan(&maxVersion); err != nil {
		return fmt.Errorf("load brief version: %w", err)
	}
	if maxVersion == 0 && previousErr == nil {
		_, err = tx.Exec(`
INSERT INTO brief_versions (
	session_id, version, style_profile_id, persona_id, output_format_id, brief_json, created_at, user_id
)
VALUES (?, 1, ?, ?, ?, ?, ?, ?)`,
			sessionID, previousStyleProfileID, previousPersonaID, previousOutputFormatID, previousBriefJSON, previousCreatedAt, s.currentUserID())
		if err != nil {
			return fmt.Errorf("seed previous brief version: %w", err)
		}
		maxVersion = 1
	}
	resultExec, err := tx.Exec(`
INSERT INTO briefs (session_id, style_profile_id, persona_id, output_format_id, brief_json, created_at, updated_at, user_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
	style_profile_id = excluded.style_profile_id,
	persona_id = excluded.persona_id,
	output_format_id = excluded.output_format_id,
	brief_json = excluded.brief_json,
	updated_at = excluded.updated_at
	WHERE briefs.user_id = excluded.user_id`,
		sessionID, brief.StyleProfileID, brief.PersonaID, brief.OutputFormatID, briefJSON, formatTime(now), formatTime(now), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save brief: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "brief", sessionID, s.currentUserID()); err != nil {
		return fmt.Errorf("save brief: %w", err)
	}
	_, err = tx.Exec(`
INSERT INTO brief_versions (
	session_id, version, style_profile_id, persona_id, output_format_id, brief_json, created_at, user_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		sessionID, maxVersion+1, brief.StyleProfileID, brief.PersonaID, brief.OutputFormatID, briefJSON, formatTime(now), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save brief version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save brief: %w", err)
	}
	return nil
}

// GetBrief returns a completed brief by session ID.
func (s *WorkflowStore) GetBrief(sessionID string) (briefdomain.ArticleBrief, bool) {
	var brief briefdomain.ArticleBrief
	var briefJSON string
	err := s.db.QueryRow(`SELECT brief_json FROM briefs WHERE session_id = ? AND user_id = ?`, sessionID, s.currentUserID()).Scan(&briefJSON)
	if err != nil {
		return briefdomain.ArticleBrief{}, false
	}
	if err := unmarshalString(briefJSON, &brief); err != nil {
		return briefdomain.ArticleBrief{}, false
	}
	return brief, true
}

// ListBriefs returns all stored completed briefs keyed by session id.
func (s *WorkflowStore) ListBriefs() (map[string]briefdomain.ArticleBrief, error) {
	rows, err := s.db.Query(`
SELECT session_id, brief_json
FROM briefs
WHERE user_id = ?
ORDER BY updated_at DESC, session_id`, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list briefs: %w", err)
	}
	defer rows.Close()
	briefs := map[string]briefdomain.ArticleBrief{}
	for rows.Next() {
		var sessionID, briefJSON string
		var brief briefdomain.ArticleBrief
		if err := rows.Scan(&sessionID, &briefJSON); err != nil {
			return nil, fmt.Errorf("scan brief: %w", err)
		}
		if err := unmarshalString(briefJSON, &brief); err != nil {
			return nil, fmt.Errorf("decode brief %q: %w", sessionID, err)
		}
		briefs[sessionID] = brief
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate briefs: %w", err)
	}
	return briefs, nil
}

// ListBriefVersions returns persisted brief revisions for a session.
func (s *WorkflowStore) ListBriefVersions(sessionID string) ([]briefdomain.ArticleBriefVersion, error) {
	rows, err := s.db.Query(`
SELECT version, brief_json, created_at
FROM brief_versions
WHERE session_id = ?
AND user_id = ?
ORDER BY version`, sessionID, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list brief versions: %w", err)
	}
	defer rows.Close()
	var versions []briefdomain.ArticleBriefVersion
	for rows.Next() {
		var version briefdomain.ArticleBriefVersion
		var briefJSON, createdAt string
		if err := rows.Scan(&version.Version, &briefJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan brief version: %w", err)
		}
		if err := unmarshalString(briefJSON, &version.Brief); err != nil {
			return nil, fmt.Errorf("decode brief version %d: %w", version.Version, err)
		}
		version.SessionID = sessionID
		version.CreatedAt = parseTime(createdAt)
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate brief versions: %w", err)
	}
	return versions, nil
}

// SavePersona stores a user-authored persona.
func (s *WorkflowStore) SavePersona(persona personadomain.Persona) error {
	if err := persona.ValidateCustom(); err != nil {
		return err
	}
	if strings.TrimSpace(persona.ID) == "" {
		return fmt.Errorf("persona id is required")
	}
	personaJSON, err := marshalString(persona)
	if err != nil {
		return fmt.Errorf("encode persona: %w", err)
	}
	now := nowUTC()
	resultExec, err := s.db.Exec(`
INSERT INTO custom_personas (id, display_name, default_format, persona_json, created_at, updated_at, user_id)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	display_name = excluded.display_name,
	default_format = excluded.default_format,
	persona_json = excluded.persona_json,
	updated_at = excluded.updated_at
	WHERE custom_personas.user_id = excluded.user_id`,
		persona.ID, persona.DisplayName, persona.DefaultFormat, personaJSON, formatTime(now), formatTime(now), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save persona: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "custom_persona", persona.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save persona: %w", err)
	}
	return nil
}

// GetPersona returns a user-authored persona by ID.
func (s *WorkflowStore) GetPersona(id string) (personadomain.Persona, bool) {
	var personaJSON string
	err := s.db.QueryRow(`SELECT persona_json FROM custom_personas WHERE id = ? AND user_id = ?`, strings.TrimSpace(id), s.currentUserID()).Scan(&personaJSON)
	if err != nil {
		return personadomain.Persona{}, false
	}
	var persona personadomain.Persona
	if err := unmarshalString(personaJSON, &persona); err != nil {
		return personadomain.Persona{}, false
	}
	return persona, true
}

// ListPersonas returns all user-authored personas in creation order.
func (s *WorkflowStore) ListPersonas() ([]personadomain.Persona, error) {
	rows, err := s.db.Query(`
SELECT persona_json
FROM custom_personas
WHERE user_id = ?
ORDER BY created_at, id`, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list personas: %w", err)
	}
	defer rows.Close()
	var personas []personadomain.Persona
	for rows.Next() {
		var personaJSON string
		if err := rows.Scan(&personaJSON); err != nil {
			return nil, fmt.Errorf("scan persona: %w", err)
		}
		var persona personadomain.Persona
		if err := unmarshalString(personaJSON, &persona); err != nil {
			return nil, fmt.Errorf("decode persona: %w", err)
		}
		personas = append(personas, persona)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate personas: %w", err)
	}
	return personas, nil
}

// DeletePersona removes an unreferenced user-authored persona.
func (s *WorkflowStore) DeletePersona(id string) error {
	id = strings.TrimSpace(id)
	var exists int
	err := s.db.QueryRow(`SELECT 1 FROM custom_personas WHERE id = ? AND user_id = ?`, id, s.currentUserID()).Scan(&exists)
	if err == sql.ErrNoRows {
		return personadomain.ErrPersonaNotFound
	}
	if err != nil {
		return fmt.Errorf("check persona: %w", err)
	}
	referenced, err := s.personaReferenced(id)
	if err != nil {
		return err
	}
	if referenced {
		return personadomain.ErrPersonaReferenced
	}
	if _, err := s.db.Exec(`DELETE FROM custom_personas WHERE id = ? AND user_id = ?`, id, s.currentUserID()); err != nil {
		return fmt.Errorf("delete persona: %w", err)
	}
	return nil
}

func (s *WorkflowStore) personaReferenced(id string) (bool, error) {
	checks := []string{
		`SELECT 1 FROM brief_sessions WHERE persona_id = ? AND user_id = ? LIMIT 1`,
		`SELECT 1 FROM briefs WHERE persona_id = ? AND user_id = ? LIMIT 1`,
		`SELECT 1 FROM brief_versions WHERE persona_id = ? AND user_id = ? LIMIT 1`,
		`SELECT 1 FROM articles WHERE persona_id = ? AND user_id = ? LIMIT 1`,
		`SELECT 1 FROM drafts WHERE persona_id = ? AND user_id = ? LIMIT 1`,
	}
	for _, query := range checks {
		var exists int
		err := s.db.QueryRow(query, id, s.currentUserID()).Scan(&exists)
		if err == nil {
			return true, nil
		}
		if err != sql.ErrNoRows {
			return false, fmt.Errorf("check persona references: %w", err)
		}
	}
	return false, nil
}

// SaveProject stores a project aggregate.
func (s *WorkflowStore) SaveProject(project ProjectRecord) error {
	if strings.TrimSpace(project.ID) == "" {
		return fmt.Errorf("project id is required")
	}
	if strings.TrimSpace(project.Name) == "" {
		return fmt.Errorf("project name is required")
	}
	project.CreatedAt = defaultTime(project.CreatedAt)
	project.UpdatedAt = defaultTime(project.UpdatedAt)
	metadataJSON, err := marshalString(nonNilMap(project.Metadata))
	if err != nil {
		return fmt.Errorf("encode project metadata: %w", err)
	}
	resultExec, err := s.db.Exec(`
INSERT INTO projects (id, name, created_at, updated_at, metadata_json, user_id)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	updated_at = excluded.updated_at,
	metadata_json = excluded.metadata_json
	WHERE projects.user_id = excluded.user_id`,
		project.ID, project.Name, formatTime(project.CreatedAt), formatTime(project.UpdatedAt), metadataJSON, s.currentUserID())
	if err != nil {
		return fmt.Errorf("save project: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "project", project.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save project: %w", err)
	}
	return nil
}

// GetProject returns a project by ID.
func (s *WorkflowStore) GetProject(id string) (ProjectRecord, bool) {
	var project ProjectRecord
	var createdAt, updatedAt, metadataJSON string
	err := s.db.QueryRow(`SELECT id, name, created_at, updated_at, metadata_json FROM projects WHERE id = ? AND user_id = ?`, id, s.currentUserID()).
		Scan(&project.ID, &project.Name, &createdAt, &updatedAt, &metadataJSON)
	if err != nil {
		return ProjectRecord{}, false
	}
	project.CreatedAt = parseTime(createdAt)
	project.UpdatedAt = parseTime(updatedAt)
	_ = unmarshalString(metadataJSON, &project.Metadata)
	return project, true
}

// ListProjects returns projects in most-recently-updated order.
func (s *WorkflowStore) ListProjects() ([]ProjectRecord, error) {
	rows, err := s.db.Query(`
SELECT id, name, created_at, updated_at, metadata_json
FROM projects
WHERE user_id = ?
ORDER BY updated_at DESC, id`, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()
	var records []ProjectRecord
	for rows.Next() {
		var record ProjectRecord
		var createdAt, updatedAt, metadataJSON string
		if err := rows.Scan(&record.ID, &record.Name, &createdAt, &updatedAt, &metadataJSON); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		record.CreatedAt = parseTime(createdAt)
		record.UpdatedAt = parseTime(updatedAt)
		_ = unmarshalString(metadataJSON, &record.Metadata)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return records, nil
}

// SaveArticle stores an article aggregate.
func (s *WorkflowStore) SaveArticle(article ArticleRecord) error {
	if strings.TrimSpace(article.ID) == "" {
		return fmt.Errorf("article id is required")
	}
	if strings.TrimSpace(article.PersonaID) == "" {
		return fmt.Errorf("article persona id is required")
	}
	if strings.TrimSpace(article.OutputFormatID) == "" {
		return fmt.Errorf("article output format id is required")
	}
	article.CreatedAt = defaultTime(article.CreatedAt)
	article.UpdatedAt = defaultTime(article.UpdatedAt)
	metadataJSON, err := marshalString(nonNilMap(article.Metadata))
	if err != nil {
		return fmt.Errorf("encode article metadata: %w", err)
	}
	resultExec, err := s.db.Exec(`
INSERT INTO articles (
	id, project_id, persona_id, output_format_id, brief_session_id,
	current_draft_id, title, created_at, updated_at, metadata_json, user_id
)
VALUES (?, nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	project_id = excluded.project_id,
	persona_id = excluded.persona_id,
	output_format_id = excluded.output_format_id,
	brief_session_id = excluded.brief_session_id,
	current_draft_id = excluded.current_draft_id,
	title = excluded.title,
	updated_at = excluded.updated_at,
	metadata_json = excluded.metadata_json
	WHERE articles.user_id = excluded.user_id`,
		article.ID, article.ProjectID, article.PersonaID, article.OutputFormatID, article.BriefSessionID, article.CurrentDraftID,
		article.Title, formatTime(article.CreatedAt), formatTime(article.UpdatedAt), metadataJSON, s.currentUserID())
	if err != nil {
		return fmt.Errorf("save article: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "article", article.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save article: %w", err)
	}
	return nil
}

// GetArticle returns an article by ID.
func (s *WorkflowStore) GetArticle(id string) (ArticleRecord, bool) {
	var article ArticleRecord
	var projectID, briefSessionID, currentDraftID sql.NullString
	var createdAt, updatedAt, metadataJSON string
	err := s.db.QueryRow(`
SELECT id, project_id, persona_id, output_format_id, brief_session_id, current_draft_id,
	title, created_at, updated_at, metadata_json
FROM articles WHERE id = ? AND user_id = ?`, id, s.currentUserID()).Scan(&article.ID, &projectID, &article.PersonaID, &article.OutputFormatID, &briefSessionID, &currentDraftID, &article.Title, &createdAt, &updatedAt, &metadataJSON)
	if err != nil {
		return ArticleRecord{}, false
	}
	article.ProjectID = projectID.String
	article.BriefSessionID = briefSessionID.String
	article.CurrentDraftID = currentDraftID.String
	article.CreatedAt = parseTime(createdAt)
	article.UpdatedAt = parseTime(updatedAt)
	_ = unmarshalString(metadataJSON, &article.Metadata)
	return article, true
}

// ListArticlesByProject returns articles for a project in most-recently-updated order.
func (s *WorkflowStore) ListArticlesByProject(projectID string) ([]ArticleRecord, error) {
	rows, err := s.db.Query(`
SELECT id
FROM articles
WHERE project_id = ?
AND user_id = ?
ORDER BY updated_at DESC, id`, projectID, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list project articles: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan article id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate article ids: %w", err)
	}
	records := make([]ArticleRecord, 0, len(ids))
	for _, id := range ids {
		record, ok := s.GetArticle(id)
		if !ok {
			return nil, fmt.Errorf("article %q disappeared while listing", id)
		}
		records = append(records, record)
	}
	return records, nil
}

// SaveSourceSnapshot stores source selector and fetch snapshots.
func (s *WorkflowStore) SaveSourceSnapshot(snapshot SourceSnapshotRecord) error {
	if strings.TrimSpace(snapshot.ID) == "" {
		return fmt.Errorf("source snapshot id is required")
	}
	if strings.TrimSpace(snapshot.ScopeType) == "" || strings.TrimSpace(snapshot.ScopeID) == "" {
		return fmt.Errorf("source snapshot scope is required")
	}
	if err := snapshot.Selector.Validate(); err != nil {
		return err
	}
	selectorJSON, err := marshalString(snapshot.Selector)
	if err != nil {
		return fmt.Errorf("encode selector: %w", err)
	}
	profileJSON, err := optionalMarshalString(snapshot.Profile)
	if err != nil {
		return fmt.Errorf("encode profile snapshot: %w", err)
	}
	articleJSON, err := optionalMarshalString(snapshot.Article)
	if err != nil {
		return fmt.Errorf("encode article snapshot: %w", err)
	}
	contentHash := snapshot.ContentHash
	fetchedAt := snapshot.FetchedAt
	if snapshot.Article != nil {
		if contentHash == "" {
			contentHash = hashString(snapshot.Article.Content)
		}
		if fetchedAt.IsZero() {
			fetchedAt = snapshot.Article.FetchedAt
		}
	}
	if contentHash == "" {
		contentHash = hashString(selectorJSON + "\x00" + articleJSON)
	}
	if fetchedAt.IsZero() {
		fetchedAt = nowUTC()
	}
	createdAt := defaultTime(snapshot.CreatedAt)
	resultExec, err := s.db.Exec(`
INSERT INTO source_selector_snapshots (
	id, scope_type, scope_id, selector_json, profile_json, article_json,
	content_hash, fetched_at, created_at, user_id
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	scope_type = excluded.scope_type,
	scope_id = excluded.scope_id,
	selector_json = excluded.selector_json,
	profile_json = excluded.profile_json,
	article_json = excluded.article_json,
	content_hash = excluded.content_hash,
	fetched_at = excluded.fetched_at
	WHERE source_selector_snapshots.user_id = excluded.user_id`,
		snapshot.ID, snapshot.ScopeType, snapshot.ScopeID, selectorJSON, nullString(profileJSON), nullString(articleJSON), contentHash, formatTime(fetchedAt), formatTime(createdAt), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save source snapshot: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "source_selector_snapshot", snapshot.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save source snapshot: %w", err)
	}
	return nil
}

// ListSourceSnapshots returns source snapshots for a scope in insertion order.
func (s *WorkflowStore) ListSourceSnapshots(scopeType, scopeID string) ([]SourceSnapshotRecord, error) {
	rows, err := s.db.Query(`
SELECT id, scope_type, scope_id, selector_json, profile_json, article_json, content_hash, fetched_at, created_at
FROM source_selector_snapshots
WHERE scope_type = ? AND scope_id = ?
AND user_id = ?
ORDER BY created_at, id`, scopeType, scopeID, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list source snapshots: %w", err)
	}
	defer rows.Close()
	var records []SourceSnapshotRecord
	for rows.Next() {
		var record SourceSnapshotRecord
		var selectorJSON, fetchedAt, createdAt string
		var profileJSON, articleJSON sql.NullString
		if err := rows.Scan(&record.ID, &record.ScopeType, &record.ScopeID, &selectorJSON, &profileJSON, &articleJSON, &record.ContentHash, &fetchedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan source snapshot: %w", err)
		}
		if err := unmarshalString(selectorJSON, &record.Selector); err != nil {
			return nil, fmt.Errorf("decode selector %s: %w", record.ID, err)
		}
		if profileJSON.Valid {
			var profile sourcedomain.ProfileSnapshot
			if err := unmarshalString(profileJSON.String, &profile); err != nil {
				return nil, fmt.Errorf("decode profile snapshot %s: %w", record.ID, err)
			}
			record.Profile = &profile
		}
		if articleJSON.Valid {
			var article sourcedomain.ArticleSnapshot
			if err := unmarshalString(articleJSON.String, &article); err != nil {
				return nil, fmt.Errorf("decode article snapshot %s: %w", record.ID, err)
			}
			record.Article = &article
		}
		record.FetchedAt = parseTime(fetchedAt)
		record.CreatedAt = parseTime(createdAt)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate source snapshots: %w", err)
	}
	return records, nil
}

// SaveDraft stores a generated draft version with evaluation and verification metadata.
func (s *WorkflowStore) SaveDraft(record DraftRecord) error {
	if strings.TrimSpace(record.ID) == "" {
		return fmt.Errorf("draft id is required")
	}
	if strings.TrimSpace(record.Markdown) == "" {
		return fmt.Errorf("draft markdown is required")
	}
	if record.Version <= 0 {
		return fmt.Errorf("draft version must be positive")
	}
	record.CreatedAt = defaultTime(record.CreatedAt)
	if record.ContentHash == "" {
		record.ContentHash = hashString(record.Markdown)
	}
	if record.QuestionTemplateVersion == "" {
		record.QuestionTemplateVersion = defaultTemplateVersion
	}
	evaluationJSON, err := marshalString(record.Evaluation)
	if err != nil {
		return fmt.Errorf("encode draft evaluation: %w", err)
	}
	verificationJSON, err := marshalString(record.Verification)
	if err != nil {
		return fmt.Errorf("encode draft verification: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin save draft: %w", err)
	}
	defer rollbackUnlessDone(tx)
	resultExec, err := tx.Exec(`
INSERT INTO drafts (
	id, article_id, session_id, style_profile_id, persona_id, output_format_id,
	version, markdown, content_hash, evaluation_json, verification_json,
	question_template_version, created_at, user_id
)
VALUES (?, nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	article_id = excluded.article_id,
	session_id = excluded.session_id,
	style_profile_id = excluded.style_profile_id,
	persona_id = excluded.persona_id,
	output_format_id = excluded.output_format_id,
	version = excluded.version,
	markdown = excluded.markdown,
	content_hash = excluded.content_hash,
	evaluation_json = excluded.evaluation_json,
	verification_json = excluded.verification_json,
	question_template_version = excluded.question_template_version
	WHERE drafts.user_id = excluded.user_id`,
		record.ID, record.ArticleID, record.SessionID, record.StyleProfileID, record.PersonaID, record.OutputFormatID, record.Version, record.Markdown, record.ContentHash,
		evaluationJSON, verificationJSON, record.QuestionTemplateVersion, formatTime(record.CreatedAt), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "draft", record.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save draft: %w", err)
	}
	if record.ArticleID != "" {
		if _, err := tx.Exec(`UPDATE articles SET current_draft_id = ?, updated_at = ? WHERE id = ? AND user_id = ?`, record.ID, formatTime(record.CreatedAt), record.ArticleID, s.currentUserID()); err != nil {
			return fmt.Errorf("update article current draft: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit save draft: %w", err)
	}
	return nil
}

// GetDraft returns a draft by ID.
func (s *WorkflowStore) GetDraft(id string) (DraftRecord, bool) {
	var record DraftRecord
	var articleID sql.NullString
	var evaluationJSON, verificationJSON, createdAt string
	err := s.db.QueryRow(`
SELECT id, article_id, session_id, style_profile_id, persona_id, output_format_id,
	version, markdown, content_hash, evaluation_json, verification_json,
	question_template_version, created_at
FROM drafts WHERE id = ? AND user_id = ?`, id, s.currentUserID()).Scan(&record.ID, &articleID, &record.SessionID, &record.StyleProfileID, &record.PersonaID, &record.OutputFormatID, &record.Version, &record.Markdown, &record.ContentHash, &evaluationJSON, &verificationJSON, &record.QuestionTemplateVersion, &createdAt)
	if err != nil {
		return DraftRecord{}, false
	}
	record.ArticleID = articleID.String
	_ = unmarshalString(evaluationJSON, &record.Evaluation)
	_ = unmarshalString(verificationJSON, &record.Verification)
	record.CreatedAt = parseTime(createdAt)
	return record, true
}

// ListDrafts returns all draft versions for an article in version order.
func (s *WorkflowStore) ListDrafts(articleID string) ([]DraftRecord, error) {
	rows, err := s.db.Query(`
SELECT id FROM drafts
WHERE article_id = ?
AND user_id = ?
ORDER BY version, created_at, id`, articleID, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan draft id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate draft ids: %w", err)
	}
	records := make([]DraftRecord, 0, len(ids))
	for _, id := range ids {
		record, ok := s.GetDraft(id)
		if !ok {
			return nil, fmt.Errorf("draft %q disappeared while listing", id)
		}
		records = append(records, record)
	}
	return records, nil
}

// SaveSectionRegeneration stores a section-regeneration draft version.
func (s *WorkflowStore) SaveSectionRegeneration(record SectionRegenerationRecord) error {
	if strings.TrimSpace(record.ID) == "" {
		return fmt.Errorf("section regeneration id is required")
	}
	if strings.TrimSpace(record.DraftID) == "" {
		return fmt.Errorf("section regeneration draft id is required")
	}
	if strings.TrimSpace(record.SectionAnchor) == "" {
		return fmt.Errorf("section regeneration anchor is required")
	}
	if record.BaseVersion <= 0 || record.Version <= 0 {
		return fmt.Errorf("section regeneration versions must be positive")
	}
	if strings.TrimSpace(record.UpdatedDraftMarkdown) == "" {
		return fmt.Errorf("updated draft markdown is required")
	}
	record.CreatedAt = defaultTime(record.CreatedAt)
	if record.UpdatedContentHash == "" {
		record.UpdatedContentHash = hashString(record.UpdatedDraftMarkdown)
	}
	verificationJSON, err := marshalString(record.Verification)
	if err != nil {
		return fmt.Errorf("encode section regeneration verification: %w", err)
	}
	resultExec, err := s.db.Exec(`
INSERT INTO section_regenerations (
	id, draft_id, article_id, section_anchor, section_heading, base_version,
	version, replacement_markdown, updated_draft_markdown, updated_content_hash,
	verification_json, created_at, user_id
)
VALUES (?, ?, nullif(?, ''), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	draft_id = excluded.draft_id,
	article_id = excluded.article_id,
	section_anchor = excluded.section_anchor,
	section_heading = excluded.section_heading,
	base_version = excluded.base_version,
	version = excluded.version,
	replacement_markdown = excluded.replacement_markdown,
	updated_draft_markdown = excluded.updated_draft_markdown,
	updated_content_hash = excluded.updated_content_hash,
	verification_json = excluded.verification_json
	WHERE section_regenerations.user_id = excluded.user_id`,
		record.ID, record.DraftID, record.ArticleID, record.SectionAnchor, record.SectionHeading, record.BaseVersion, record.Version,
		record.ReplacementMarkdown, record.UpdatedDraftMarkdown, record.UpdatedContentHash, verificationJSON, formatTime(record.CreatedAt), s.currentUserID())
	if err != nil {
		return fmt.Errorf("save section regeneration: %w", err)
	}
	if err := ensureScopedWrite(resultExec, "section_regeneration", record.ID, s.currentUserID()); err != nil {
		return fmt.Errorf("save section regeneration: %w", err)
	}
	return nil
}

// ListSectionRegenerations returns regenerations for a draft in version order.
func (s *WorkflowStore) ListSectionRegenerations(draftID string) ([]SectionRegenerationRecord, error) {
	rows, err := s.db.Query(`
SELECT id, draft_id, article_id, section_anchor, section_heading, base_version,
	version, replacement_markdown, updated_draft_markdown, updated_content_hash,
	verification_json, created_at
FROM section_regenerations
WHERE draft_id = ?
AND user_id = ?
ORDER BY version, created_at, id`, draftID, s.currentUserID())
	if err != nil {
		return nil, fmt.Errorf("list section regenerations: %w", err)
	}
	defer rows.Close()
	var records []SectionRegenerationRecord
	for rows.Next() {
		var record SectionRegenerationRecord
		var articleID sql.NullString
		var verificationJSON, createdAt string
		if err := rows.Scan(&record.ID, &record.DraftID, &articleID, &record.SectionAnchor, &record.SectionHeading, &record.BaseVersion, &record.Version, &record.ReplacementMarkdown, &record.UpdatedDraftMarkdown, &record.UpdatedContentHash, &verificationJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan section regeneration: %w", err)
		}
		record.ArticleID = articleID.String
		_ = unmarshalString(verificationJSON, &record.Verification)
		record.CreatedAt = parseTime(createdAt)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate section regenerations: %w", err)
	}
	return records, nil
}

func migrationVersion(name string) (int, error) {
	prefix := strings.SplitN(name, "_", 2)[0]
	version, err := strconv.Atoi(prefix)
	if err != nil {
		return 0, fmt.Errorf("invalid migration filename %q: %w", name, err)
	}
	return version, nil
}

func rollbackUnlessDone(tx *sql.Tx) {
	_ = tx.Rollback()
}

func marshalString(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func optionalMarshalString[T any](value *T) (string, error) {
	if value == nil {
		return "", nil
	}
	return marshalString(value)
}

func unmarshalString(encoded string, out any) error {
	return json.Unmarshal([]byte(encoded), out)
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nowUTC() time.Time {
	return time.Now().UTC().Round(0)
}

func defaultTime(value time.Time) time.Time {
	if value.IsZero() {
		return nowUTC()
	}
	return value.UTC().Round(0)
}

func formatTime(value time.Time) string {
	return defaultTime(value).Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func nonNilMap(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	return value
}
