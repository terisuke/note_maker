package sqlite

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authorstyleapp "github.com/teradakousuke/note_maker/internal/application/authorstyle"
	draftapp "github.com/teradakousuke/note_maker/internal/application/draft"
	articledomain "github.com/teradakousuke/note_maker/internal/domain/article"
	authordomain "github.com/teradakousuke/note_maker/internal/domain/author"
	briefdomain "github.com/teradakousuke/note_maker/internal/domain/brief"
	outputformat "github.com/teradakousuke/note_maker/internal/domain/format"
	personadomain "github.com/teradakousuke/note_maker/internal/domain/persona"
	sourcedomain "github.com/teradakousuke/note_maker/internal/domain/source"
)

func TestWorkflowStoreRestoresDraftInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note_maker.db")
	store, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	result := testAnalyzeResult(t)
	if err := store.SaveAuthorStyle(result); err != nil {
		t.Fatalf("save author style: %v", err)
	}
	session := testCompletedSession(t, result.Profile.ID)
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	brief := session.AssembleBrief()
	if err := store.SaveBrief(session.ID, brief); err != nil {
		t.Fatalf("save brief: %v", err)
	}
	editedBrief := brief
	editedBrief.Theme = "Edited SQLite workflow brief"
	if err := store.SaveBrief(session.ID, editedBrief); err != nil {
		t.Fatalf("save edited brief: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	for _, id := range []string{result.ID, result.Profile.ID, result.Guide.ID} {
		profile, guide, ok := reopened.GetProfileAndGuide(id)
		if !ok {
			t.Fatalf("expected profile and guide for id %q after reopen", id)
		}
		if profile.ID != result.Profile.ID || guide.ID != result.Guide.ID {
			t.Fatalf("restored wrong style assets: profile=%s guide=%s", profile.ID, guide.ID)
		}
	}
	restoredSession, ok := reopened.GetSession(session.ID)
	if !ok {
		t.Fatal("expected session after reopen")
	}
	if restoredSession.PersonaID != personadomain.IDTerisuke || restoredSession.OutputFormatID != outputformat.IDNoteArticle {
		t.Fatalf("session mode was not restored: %#v", restoredSession)
	}
	if len(restoredSession.Answers) != len(session.Answers) {
		t.Fatalf("answers = %d, want %d", len(restoredSession.Answers), len(session.Answers))
	}
	restoredBrief, ok := reopened.GetBrief(session.ID)
	if !ok {
		t.Fatal("expected brief after reopen")
	}
	if restoredBrief.Theme != editedBrief.Theme || restoredBrief.PersonalContext == "" || len(restoredBrief.DeepDives) != 1 || len(restoredBrief.CustomAnswers) != 1 {
		t.Fatalf("brief did not preserve generation context: %#v", restoredBrief)
	}
	versions, err := reopened.ListBriefVersions(session.ID)
	if err != nil {
		t.Fatalf("list brief versions: %v", err)
	}
	if len(versions) != 2 || versions[0].Brief.Theme == editedBrief.Theme || versions[1].Brief.Theme != editedBrief.Theme {
		t.Fatalf("unexpected brief versions: %#v", versions)
	}
}

func TestWorkflowStoreRejectsCrossUserNaturalIDConflicts(t *testing.T) {
	base, err := NewWorkflowStore(filepath.Join(t.TempDir(), "note_maker.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = base.Close() })
	alice := base.ForUser("alice")
	bob := base.ForUser("bob")

	persona := testCustomPersona()
	if err := alice.SavePersona(persona); err != nil {
		t.Fatalf("save alice persona: %v", err)
	}
	if err := bob.SavePersona(persona); !errors.Is(err, ErrScopedIDConflict) {
		t.Fatalf("save bob persona err = %v, want ErrScopedIDConflict", err)
	}

	now := time.Unix(1710000200, 0).UTC()
	project := ProjectRecord{ID: "shared-project", Name: "Shared ID", CreatedAt: now, UpdatedAt: now}
	if err := alice.SaveProject(project); err != nil {
		t.Fatalf("save alice project: %v", err)
	}
	if err := bob.SaveProject(project); !errors.Is(err, ErrScopedIDConflict) {
		t.Fatalf("save bob project err = %v, want ErrScopedIDConflict", err)
	}
}

func TestWorkflowStoreRestoresCustomPersonas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note_maker.db")
	store, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	persona := testCustomPersona()
	if err := store.SavePersona(persona); err != nil {
		t.Fatalf("save persona: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restored, ok := reopened.GetPersona(persona.ID)
	if !ok {
		t.Fatal("expected persona after reopen")
	}
	if restored.DisplayName != persona.DisplayName || restored.DefaultFormat != persona.DefaultFormat {
		t.Fatalf("unexpected restored persona: %#v", restored)
	}
	listed, err := reopened.ListPersonas()
	if err != nil {
		t.Fatalf("list personas: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != persona.ID {
		t.Fatalf("unexpected persona list: %#v", listed)
	}
}

func TestWorkflowStoreDeletesOnlyUnreferencedCustomPersonas(t *testing.T) {
	store, err := NewWorkflowStore(filepath.Join(t.TempDir(), "note_maker.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	persona := testCustomPersona()
	if err := store.SavePersona(persona); err != nil {
		t.Fatalf("save persona: %v", err)
	}
	if err := store.DeletePersona(persona.ID); err != nil {
		t.Fatalf("delete unreferenced persona: %v", err)
	}
	if _, ok := store.GetPersona(persona.ID); ok {
		t.Fatal("persona should be deleted")
	}
	if err := store.DeletePersona(persona.ID); err != personadomain.ErrPersonaNotFound {
		t.Fatalf("delete missing persona err = %v, want ErrPersonaNotFound", err)
	}

	if err := store.SavePersona(persona); err != nil {
		t.Fatalf("resave persona: %v", err)
	}
	session, err := briefdomain.NewArticleBriefSessionWithOptions("session_custom", "profile_custom", persona.ID, outputformat.IDNoteArticle, "", briefdomain.FixedQuestions())
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	if err := store.DeletePersona(persona.ID); err != personadomain.ErrPersonaReferenced {
		t.Fatalf("delete referenced persona err = %v, want ErrPersonaReferenced", err)
	}
}

func TestWorkflowStoreAppliesSchemaMigrations(t *testing.T) {
	store, err := NewWorkflowStore(filepath.Join(t.TempDir(), "note_maker.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var migrationCount int
	if err := store.DB().QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = 1`).Scan(&migrationCount); err != nil {
		t.Fatalf("query schema migrations: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("migration count = %d, want 1", migrationCount)
	}
	if err := store.DB().QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = 3`).Scan(&migrationCount); err != nil {
		t.Fatalf("query brief version migration: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("brief version migration count = %d, want 1", migrationCount)
	}
	if err := store.DB().QueryRow(`SELECT count(*) FROM schema_migrations WHERE version = 4`).Scan(&migrationCount); err != nil {
		t.Fatalf("query user id migration: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("user id migration count = %d, want 1", migrationCount)
	}
	for _, table := range []string{"projects", "articles", "brief_sessions", "brief_answers", "briefs", "brief_versions", "custom_personas", "drafts", "section_regenerations", "source_selector_snapshots"} {
		var name string
		if err := store.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
		var columnName string
		if err := store.DB().QueryRow(`SELECT name FROM pragma_table_info(?) WHERE name = 'user_id'`, table).Scan(&columnName); err != nil {
			t.Fatalf("expected %s.user_id column: %v", table, err)
		}
	}
}

func TestWorkflowStoreScopesRecordsByUser(t *testing.T) {
	base, err := NewWorkflowStore(filepath.Join(t.TempDir(), "note_maker.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = base.Close() })
	alice := base.ForUser("alice")
	bob := base.ForUser("bob")
	now := time.Unix(1710000100, 0).UTC()

	aliceStyle := testAnalyzeResult(t)
	aliceStyle.ID = "asr_alice"
	aliceStyle.Profile.ID = "profile_alice"
	aliceStyle.Guide.ID = "guide_alice"
	if err := alice.SaveAuthorStyle(aliceStyle); err != nil {
		t.Fatalf("save alice style: %v", err)
	}
	bobStyle := testAnalyzeResult(t)
	bobStyle.ID = "asr_bob"
	bobStyle.Profile.ID = "profile_bob"
	bobStyle.Guide.ID = "guide_bob"
	if err := bob.SaveAuthorStyle(bobStyle); err != nil {
		t.Fatalf("save bob style: %v", err)
	}
	aliceSession := testCompletedSessionWithID(t, "brief_alice", aliceStyle.Profile.ID)
	if err := alice.SaveSession(aliceSession); err != nil {
		t.Fatalf("save alice session: %v", err)
	}
	if err := alice.SaveBrief(aliceSession.ID, aliceSession.AssembleBrief()); err != nil {
		t.Fatalf("save alice brief: %v", err)
	}
	bobSession := testCompletedSessionWithID(t, "brief_bob", bobStyle.Profile.ID)
	if err := bob.SaveSession(bobSession); err != nil {
		t.Fatalf("save bob session: %v", err)
	}
	if err := bob.SaveBrief(bobSession.ID, bobSession.AssembleBrief()); err != nil {
		t.Fatalf("save bob brief: %v", err)
	}
	if err := alice.SaveProject(ProjectRecord{ID: "project_alice", Name: "Alice", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("save alice project: %v", err)
	}
	if err := bob.SaveProject(ProjectRecord{ID: "project_bob", Name: "Bob", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("save bob project: %v", err)
	}

	if _, ok := alice.GetAuthorStyle(bobStyle.ID); ok {
		t.Fatal("alice should not read bob author style")
	}
	if _, ok := alice.GetBrief(bobSession.ID); ok {
		t.Fatal("alice should not read bob brief")
	}
	if _, ok := alice.GetProject("project_bob"); ok {
		t.Fatal("alice should not read bob project")
	}
	styles, err := alice.ListAuthorStyles()
	if err != nil {
		t.Fatalf("list alice styles: %v", err)
	}
	if len(styles) != 1 || styles[0].ID != aliceStyle.ID {
		t.Fatalf("alice styles = %#v", styles)
	}
	briefs, err := alice.ListBriefs()
	if err != nil {
		t.Fatalf("list alice briefs: %v", err)
	}
	if len(briefs) != 1 {
		t.Fatalf("alice brief count = %d, want 1", len(briefs))
	}
	projects, err := alice.ListProjects()
	if err != nil {
		t.Fatalf("list alice projects: %v", err)
	}
	if len(projects) != 1 || projects[0].ID != "project_alice" {
		t.Fatalf("alice projects = %#v", projects)
	}
}

func TestBriefVersionsMigrationBackfillIsIdempotent(t *testing.T) {
	store, err := NewWorkflowStore(filepath.Join(t.TempDir(), "note_maker.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	session := testCompletedSession(t, "profile-idempotent")
	if err := store.SaveSession(session); err != nil {
		t.Fatalf("save session: %v", err)
	}
	brief := session.AssembleBrief()
	if err := store.SaveBrief(session.ID, brief); err != nil {
		t.Fatalf("save brief: %v", err)
	}
	if _, err := store.DB().Exec(`DELETE FROM brief_versions WHERE session_id = ?`, session.ID); err != nil {
		t.Fatalf("clear brief versions: %v", err)
	}

	migration, err := migrationFiles.ReadFile("migrations/0003_brief_versions.sql")
	if err != nil {
		t.Fatalf("read brief versions migration: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := store.DB().Exec(string(migration)); err != nil {
			t.Fatalf("rerun brief versions migration %d: %v", i+1, err)
		}
	}
	var count int
	if err := store.DB().QueryRow(`SELECT count(*) FROM brief_versions WHERE session_id = ?`, session.ID).Scan(&count); err != nil {
		t.Fatalf("count brief versions: %v", err)
	}
	if count != 1 {
		t.Fatalf("backfilled version count = %d, want 1", count)
	}
}

func TestWorkflowStorePersistsHistoryRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "note_maker.db")
	store, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	now := time.Unix(1710000000, 0).UTC()

	project := ProjectRecord{
		ID:        "project_media_matrix",
		Name:      "Media matrix",
		CreatedAt: now,
		UpdatedAt: now,
		Metadata:  map[string]any{"owner": "scenario"},
	}
	if err := store.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}
	article := ArticleRecord{
		ID:             "article_zenn",
		ProjectID:      project.ID,
		PersonaID:      personadomain.IDCloudia,
		OutputFormatID: outputformat.IDZennArticle,
		BriefSessionID: "brief_zenn",
		Title:          "SQLite-backed history",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := store.SaveArticle(article); err != nil {
		t.Fatalf("save article: %v", err)
	}
	sourceArticle := sourcedomain.ArticleSnapshot{
		ID:        "source-1",
		Kind:      sourcedomain.KindZenn,
		URL:       "https://zenn.dev/cloudia/articles/sqlite-history",
		Title:     "SQLite history",
		Content:   "source body",
		FetchedAt: now,
	}
	if err := store.SaveSourceSnapshot(SourceSnapshotRecord{
		ID:        "snapshot-1",
		ScopeType: "article",
		ScopeID:   article.ID,
		Selector: sourcedomain.Ref{
			Kind: sourcedomain.KindZenn,
			Ref:  "cloudia",
		},
		Article:   &sourceArticle,
		FetchedAt: now,
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("save source snapshot: %v", err)
	}
	draftRecord := DraftRecord{
		ID:             "draft-1",
		ArticleID:      article.ID,
		SessionID:      article.BriefSessionID,
		StyleProfileID: "profile-1",
		PersonaID:      article.PersonaID,
		OutputFormatID: article.OutputFormatID,
		Version:        1,
		Markdown:       "---\ntitle: \"SQLite\"\nemoji: \"🧪\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n## 実装\n\n本文です。",
		Evaluation: draftapp.StyleEvaluation{
			Passed: true,
		},
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
			Summary:   "ok",
		},
		CreatedAt: now,
	}
	if err := store.SaveDraft(draftRecord); err != nil {
		t.Fatalf("save draft: %v", err)
	}
	if err := store.SaveSectionRegeneration(SectionRegenerationRecord{
		ID:                   "regen-1",
		DraftID:              draftRecord.ID,
		ArticleID:            article.ID,
		SectionAnchor:        "implementation",
		SectionHeading:       "実装",
		BaseVersion:          1,
		Version:              2,
		ReplacementMarkdown:  "## 実装\n\n更新後です。",
		UpdatedDraftMarkdown: "---\ntitle: \"SQLite\"\nemoji: \"🧪\"\ntype: \"tech\"\ntopics: [\"go\"]\npublished: false\n---\n\n## 実装\n\n更新後です。",
		Verification: draftapp.FinalVerification{
			Performed: true,
			Passed:    true,
			Summary:   "regenerated ok",
		},
		CreatedAt: now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("save section regeneration: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := NewWorkflowStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restoredProject, ok := reopened.GetProject(project.ID)
	if !ok || restoredProject.Name != project.Name || restoredProject.Metadata["owner"] != "scenario" {
		t.Fatalf("unexpected restored project: %#v ok=%v", restoredProject, ok)
	}
	restoredArticle, ok := reopened.GetArticle(article.ID)
	if !ok || restoredArticle.CurrentDraftID != draftRecord.ID {
		t.Fatalf("unexpected restored article: %#v ok=%v", restoredArticle, ok)
	}
	snapshots, err := reopened.ListSourceSnapshots("article", article.ID)
	if err != nil {
		t.Fatalf("list source snapshots: %v", err)
	}
	if len(snapshots) != 1 || snapshots[0].ContentHash == "" || snapshots[0].Article.Content != sourceArticle.Content {
		t.Fatalf("unexpected snapshots: %#v", snapshots)
	}
	drafts, err := reopened.ListDrafts(article.ID)
	if err != nil {
		t.Fatalf("list drafts: %v", err)
	}
	if len(drafts) != 1 || drafts[0].ContentHash == "" || !drafts[0].Verification.Passed || drafts[0].QuestionTemplateVersion == "" {
		t.Fatalf("unexpected drafts: %#v", drafts)
	}
	regenerations, err := reopened.ListSectionRegenerations(draftRecord.ID)
	if err != nil {
		t.Fatalf("list regenerations: %v", err)
	}
	if len(regenerations) != 1 || regenerations[0].Version != 2 || !strings.Contains(regenerations[0].UpdatedDraftMarkdown, "更新後") {
		t.Fatalf("unexpected regenerations: %#v", regenerations)
	}
}

func testAnalyzeResult(t *testing.T) authorstyleapp.AnalyzeResult {
	t.Helper()
	fetchedAt := time.Unix(1700000000, 0).UTC()
	article := articledomain.Article{
		URL:     "https://note.com/tera/n/n111",
		Title:   "AIと音楽",
		Content: "僕はAIと音楽の違和感を言語化する。\n\n「これは大事だ」と思った。",
	}
	source := authordomain.AuthorSource{
		Username:  "tera",
		Articles:  authordomain.SourceArticlesFromArticles([]articledomain.Article{article}, fetchedAt),
		FetchedAt: fetchedAt,
	}
	profile, err := authordomain.BuildAuthorStyleProfile(source, []articledomain.Article{article})
	if err != nil {
		t.Fatalf("build profile: %v", err)
	}
	guide, err := authordomain.BuildWritingStyleGuide(profile)
	if err != nil {
		t.Fatalf("build guide: %v", err)
	}
	return authorstyleapp.AnalyzeResult{
		ID:           "asr_test",
		Source:       source,
		Profile:      profile,
		Guide:        guide,
		ArticleCount: 1,
		CreatedAt:    fetchedAt,
	}
}

func testCustomPersona() personadomain.Persona {
	return personadomain.Persona{
		ID:            "custom_writer",
		DisplayName:   "Custom Writer",
		Description:   "A locally authored test persona.",
		DefaultFormat: outputformat.IDNoteArticle,
		Sources: []personadomain.AuthorSource{
			{Kind: "note", Ref: "custom_writer"},
		},
		VoiceNotes: personadomain.VoiceNotes{
			FirstPerson:   []string{"私"},
			Tone:          "Calm, direct, and specific.",
			TitlePatterns: []string{"How I use local workflows"},
			AntiPatterns:  []string{"empty claims"},
		},
	}
}

func testCompletedSession(t *testing.T, profileID string) briefdomain.ArticleBriefSession {
	return testCompletedSessionWithID(t, "brief_test", profileID)
}

func testCompletedSessionWithID(t *testing.T, sessionID, profileID string) briefdomain.ArticleBriefSession {
	t.Helper()
	questions := append(briefdomain.FixedQuestions(), briefdomain.ArticleQuestion{
		ID:          "custom_origin",
		Text:        "どの原体験を入れますか？",
		FlowType:    briefdomain.QuestionFlowMain,
		Required:    false,
		TargetField: "custom",
	})
	session, err := briefdomain.NewArticleBriefSessionWithQuestions(sessionID, profileID, questions)
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	answers := map[string]string{
		briefdomain.QuestionIDTheme:                 "ローカルLLMで思想を言語化する",
		briefdomain.QuestionIDOpeningEpisode:        "生成文を読んで自分の切実さが抜けていた",
		briefdomain.QuestionIDReader:                "AIで発信したい個人開発者",
		briefdomain.QuestionIDExpectedReaderAction:  "AIに取材させる視点を持つ",
		briefdomain.QuestionIDMustInclude:           "Note API、文体ガイド、一問一答、深掘り",
		briefdomain.QuestionIDPersonalContext:       "音楽家、エンジニア、起業、LT登壇の経験",
		briefdomain.QuestionIDExclusions:            "根拠のない性能断言",
		briefdomain.QuestionIDTargetLengthStructure: "3000字前後、最低2800字",
		briefdomain.QuestionIDToneStance:            "内省と技術検証を両立する",
		"custom_origin":                             "音楽の練習で身体化した感覚",
	}
	for _, question := range questions {
		if _, err := session.RecordAnswer(answers[question.ID]); err != nil {
			t.Fatalf("answer %s: %v", question.ID, err)
		}
	}
	if _, err := session.RecordAnswer("自分の言葉を失う怖さを最初に見せる"); err != nil {
		t.Fatalf("answer deep dive: %v", err)
	}
	if _, err := session.Complete(); err != nil {
		t.Fatalf("complete: %v", err)
	}
	return session
}
